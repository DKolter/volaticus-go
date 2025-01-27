package uploader

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"strconv"
	"volaticus-go/cmd/web/components"
	"volaticus-go/cmd/web/pages"
	"volaticus-go/internal/context"
	userctx "volaticus-go/internal/context"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

const (
	defaultPageSize = 10
	maxPageSize     = 50
)

type Haaandler interface {
	HandleUpload(w http.ResponseWriter, r *http.Request)
	HandleAPIUpload(w http.ResponseWriter, r *http.Request)
	HandleServeFile(w http.ResponseWriter, r *http.Request)
	HandleFilesList(w http.ResponseWriter, r *http.Request)
	HandleDeleteFile(w http.ResponseWriter, r *http.Request)
	HandleGetFileStats(w http.ResponseWriter, r *http.Request)
}

type Handler struct {
	service *service
}

func NewHandler(service *service) *Handler {
	return &Handler{
		service: service,
	}
}

// HandleVerifyFile handles file validation
func (h *Handler) HandleVerifyFile(w http.ResponseWriter, r *http.Request) {
	file, header, err := r.FormFile("file")
	if err != nil {
		err := components.ValidationError("Invalid file").Render(r.Context(), w)
		if err != nil {
			log.Error().
				Err(err).
				Msg("Error rendering validation error")
		}
		return
	}
	defer func(file multipart.File) {
		err := file.Close()
		if err != nil {
			log.Error().
				Err(err).
				Msg("Error closing file")
		}
	}(file)

	// Get the user context
	userContext := userctx.GetUserFromContext(r.Context())
	if userContext == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Validate the file using service
	result := h.service.ValidateFile(r.Context(), file, header)

	if !result.IsValid {
		err := components.ValidationError(result.Error).Render(r.Context(), w)
		if err != nil {
			log.Error().
				Err(err).
				Msg("Error rendering validation")
		}
		return
	}

	// Render success component
	err = components.ValidationSuccess(result.FileName, result.FileSize, result.ContentType).Render(r.Context(), w)
	if err != nil {
		log.Error().
			Err(err).
			Str("filename", result.FileName).
			Int64("size", result.FileSize).
			Str("contentType", result.ContentType).
			Msg("Error rendering validation success")
	}
}

// HandleUpload handles file upload
func (h *Handler) HandleUpload(w http.ResponseWriter, r *http.Request) {
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Invalid File", http.StatusBadRequest)
		return
	}
	defer func(file multipart.File) {
		err := file.Close()
		if err != nil {
			log.Error().
				Err(err).
				Msg("Error closing file")
		}
	}(file)

	userContext := userctx.GetUserFromContext(r.Context())
	if userContext == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse the URL type from the form
	urlType := r.FormValue("url_type")
	if urlType == "" {
		urlType = "default"
	}

	parsedURLType, err := ParseURLType(urlType)
	if err != nil {
		http.Error(w, "Invalid URL type", http.StatusBadRequest)
		return
	}

	uploadReq := &UploadRequest{
		File:    file,
		Header:  header,
		URLType: parsedURLType,
		UserID:  userContext.ID,
	}

	uploadedFile, err := h.service.UploadFile(r.Context(), uploadReq)
	if err != nil {
		log.Error().
			Err(err).
			Str("userId", userContext.ID.String()).
			Str("filename", header.Filename).
			Str("urlType", urlType).
			Msg("Error uploading file")
		http.Error(w, "Error uploading file", http.StatusInternalServerError)
		return
	}

	url := fmt.Sprintf("%s/f/%s", h.service.config.BaseURL, uploadedFile.URLValue)

	// Render success template
	if err := pages.UploadSuccess(url, uploadedFile.OriginalName).Render(r.Context(), w); err != nil {
		log.Error().
			Err(err).
			Str("fileUrl", url).
			Str("originalName", uploadedFile.OriginalName).
			Msg("Error rendering success template")
		http.Error(w, "Error rendering response", http.StatusInternalServerError)
	}
}

// HandleServeFile serves the uploaded file
func (h *Handler) HandleServeFile(w http.ResponseWriter, r *http.Request) {
	urlValue := chi.URLParam(r, "fileUrl")
	log.Info().
		Str("fileUrl", urlValue).
		Msg("Got Serve File Request")

	if urlValue == "" {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	file, err := h.service.GetFile(r.Context(), urlValue)
	if err != nil {
		if errors.Is(err, ErrNoRows) {
			http.Error(w, "File not found", http.StatusNotFound)
		} else {
			log.Printf("Error retrieving file: %v", err)
			http.Error(w, "Error retrieving file", http.StatusInternalServerError)
		}
		return
	}

	log.Info().
		Str("filename", file.OriginalName).
		Str("mimeType", file.MimeType).
		Msg("Serving file")

	contentType := file.MimeType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)

	if r.URL.Query().Get("download") == "true" {
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.OriginalName))
	} else {
		w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, file.OriginalName))
	}

	// Add cache control
	w.Header().Set("Cache-Control", "public, max-age=86400") // 24 hours
	w.Header().Set("ETag", fmt.Sprintf(`"%s"`, file.UniqueFilename))

	// Check if client has a cached version
	if match := r.Header.Get("If-None-Match"); match != "" {
		if match == fmt.Sprintf(`"%s"`, file.UniqueFilename) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	// Serve the file
	if err := h.service.ServeFile(r.Context(), w, file); err != nil {
		log.Printf("Error serving file: %v", err)
		http.Error(w, "Error serving file", http.StatusInternalServerError)
		return
	}
}

type APIUploadResponse struct {
	Success bool   `json:"success"`
	URL     string `json:"url,omitempty"`
	Error   string `json:"error,omitempty"`
}

// sendAPIResponse handles JSON response formatting consistently
func sendAPIResponse(w http.ResponseWriter, status int, success bool, url string, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	response := APIUploadResponse{
		Success: success,
		URL:     url,
	}

	if err != nil {
		response.Error = err.Error()
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error().
			Err(err).
			Msg("Error encoding response")
	}
}

// HandleAPIUpload handles file upload through the API with minimal configuration
func (h *Handler) HandleAPIUpload(w http.ResponseWriter, r *http.Request) {
	log.Info().
		Str("remoteAddr", r.RemoteAddr).
		Msg("API Upload request from")

	// Get user from context
	userContext := userctx.GetUserFromContext(r.Context())
	if userContext == nil {
		sendAPIResponse(w, http.StatusUnauthorized, false, "", errors.New("unauthorized"))
		return
	}

	// Check content length against max size
	if r.ContentLength > h.service.config.UploadMaxSize {
		sendAPIResponse(w, http.StatusRequestEntityTooLarge, false, "", ErrFileTooLarge)
		return
	}

	// Check current storage usage against quota
	stats, err := h.service.repo.GetFileStats(r.Context(), userContext.ID)
	if err != nil {
		log.Error().
			Err(err).
			Str("user_id", userContext.ID.String()).
			Msg("Failed to get user storage stats")
		sendAPIResponse(w, http.StatusInternalServerError, false, "", errors.New("failed to check storage quota"))
		return
	}

	// Check if this upload would exceed quota
	if stats.TotalSize+r.ContentLength > h.service.config.UploadUserQuota {
		log.Warn().
			Str("user_id", userContext.ID.String()).
			Int64("current_size", stats.TotalSize).
			Int64("upload_size", r.ContentLength).
			Int64("quota", h.service.config.UploadUserQuota).
			Msg("Upload would exceed user quota")
		sendAPIResponse(w, http.StatusBadRequest, false, "", fmt.Errorf("upload would exceed your storage quota of %s", formatSize(h.service.config.UploadUserQuota)))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, http.ErrMissingFile) {
			err = ErrNoFile
		}
		sendAPIResponse(w, status, false, "", err)
		return
	}
	defer func(file multipart.File) {
		err := file.Close()
		if err != nil {
			log.Error().
				Err(err).
				Msg("Error closing file")
		}
	}(file)

	// Parse URL type from header
	urlType := URLTypeDefault
	if typeHeader := r.Header.Get("Url-Type"); typeHeader != "" {
		parsedType, err := ParseURLType(typeHeader)
		if err != nil {
			sendAPIResponse(w, http.StatusBadRequest, false, "", ErrInvalidURLType)
			return
		}
		urlType = parsedType
	}

	uploadReq := &UploadRequest{
		File:    file,
		Header:  header,
		URLType: urlType,
		UserID:  userContext.ID,
	}

	uploadedFile, err := h.service.UploadFile(r.Context(), uploadReq)
	if err != nil {
		log.Error().
			Err(err).
			Msg("Upload error")
		sendAPIResponse(w, http.StatusInternalServerError, false, "", errors.New("upload failed"))
		return
	}

	url := fmt.Sprintf("%s/f/%s", h.service.config.BaseURL, uploadedFile.URLValue)
	sendAPIResponse(w, http.StatusOK, true, url, nil)
}

// HandleFilesList handles the GET /files/list endpoint
func (h *Handler) HandleFilesList(w http.ResponseWriter, r *http.Request) {
	user := context.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse pagination parameters
	page := 1
	limit := defaultPageSize

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= maxPageSize {
			limit = l
		}
	}

	offset := (page - 1) * limit

	// Get files and stats for the current user with pagination
	files, err := h.service.GetUserFiles(r.Context(), user.ID, limit, offset)
	if err != nil {
		log.Error().
			Err(err).
			Msg("Error fetching files")
		http.Error(w, "Error fetching files", http.StatusInternalServerError)
		return
	}

	// Get total count for pagination
	total, err := h.service.GetUserFilesCount(r.Context(), user.ID)
	if err != nil {
		log.Error().
			Err(err).
			Msg("Error fetching file count")
		http.Error(w, "Error fetching file count", http.StatusInternalServerError)
		return
	}

	totalPages := (total + limit - 1) / limit // Ceiling division

	// Render the file list component
	props := components.FileListProps{
		Files:      files,
		ShowPaging: true,
		Page:       page,
		TotalPages: totalPages,
		EmptyState: "No files uploaded yet",
	}

	err = components.FileListComponent(props).Render(r.Context(), w)
	if err != nil {
		log.Error().
			Err(err).
			Msg("Error rendering file list")
		http.Error(w, "Error rendering file list", http.StatusInternalServerError)
		return
	}
}

// HandleRecentFiles returns the last N files for a user
func (h *Handler) HandleRecentFiles(w http.ResponseWriter, r *http.Request, limit int) {
	user := context.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get recent files
	files, err := h.service.GetUserFiles(r.Context(), user.ID, limit, 0)
	if err != nil {
		http.Error(w, "Error fetching recent files", http.StatusInternalServerError)
		return
	}

	// Render the file list component without pagination
	props := components.FileListProps{
		Files:      files,
		ShowPaging: false,
		EmptyState: "Upload your first file above",
	}

	err = components.FileListComponent(props).Render(r.Context(), w)
	if err != nil {
		http.Error(w, "Error rendering file list", http.StatusInternalServerError)
		return
	}
}

func (h *Handler) HandleDeleteFile(w http.ResponseWriter, r *http.Request) {
	user := context.GetUserFromContext(r.Context())
	log.Info().
		Interface("user", user).
		Str("fileID", chi.URLParam(r, "fileID")).
		Msg("User is attempting to delete File")
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		log.Info().Msg("Unauthorized")
		return
	}

	fileID := chi.URLParam(r, "fileID")
	if fileID == "" {
		http.Error(w, "Missing file ID", http.StatusBadRequest)
		log.Info().Msg("Missing file ID")
		return
	}

	// Parse file ID
	id, err := uuid.Parse(fileID)
	if err != nil {
		http.Error(w, "Invalid file ID", http.StatusBadRequest)
		log.Error().
			Err(err).
			Msg("Invalid file ID")
		return
	}

	// Delete the file
	err = h.service.DeleteFileByID(r.Context(), id, user.ID)
	if err != nil {
		switch {
		case errors.Is(err, ErrUnauthorized):
			http.Error(w, "Unauthorized", http.StatusForbidden)
			log.Info().Msg("Unauthorized")
		case errors.Is(err, ErrNoRows):
			http.Error(w, "File not found", http.StatusNotFound)
			log.Info().Msg("File not found")
		default:
			log.Error().
				Err(err).
				Msg("Error deleting file")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	// Set header to trigger refresh of file lists
	w.Header().Set("HX-Trigger", "fileDeleted")
	w.WriteHeader(http.StatusOK)
}

// HandleGetFileStats returns the file stats component for a user
func (h *Handler) HandleGetFileStats(w http.ResponseWriter, r *http.Request) {
	user := context.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	stats, err := h.service.GetFileStats(r.Context(), user.ID)
	if err != nil {
		http.Error(w, "Error fetching file stats", http.StatusInternalServerError)
		return
	}

	err = components.FileStatsComponent(stats).Render(r.Context(), w)
	if err != nil {
		http.Error(w, "Error rendering file stats", http.StatusInternalServerError)
		return
	}
}
