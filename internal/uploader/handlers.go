package uploader

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"volaticus-go/cmd/web/components"
	"volaticus-go/cmd/web/pages"
	"volaticus-go/internal/context"
	userctx "volaticus-go/internal/context"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const (
	defaultPageSize = 10
	maxPageSize     = 50
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
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
			log.Printf("Error rendering validation error: %v", err)
		}
		return
	}
	defer func(file multipart.File) {
		err := file.Close()
		if err != nil {
			log.Printf("Error closing file: %v", err)
		}
	}(file)

	// Validate the file using service
	result := h.service.VerifyFile(r.Context(), file, header)

	if !result.IsValid {
		err := components.ValidationError(result.Error).Render(r.Context(), w)
		if err != nil {
			log.Printf("Error rendering validation: %v", err)
		}
		return
	}

	// Render success component
	err = components.ValidationSuccess(result.FileName, result.FileSize, result.ContentType).Render(r.Context(), w)
	if err != nil {
		log.Printf("Error rendering validation success: %v", err)
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
			log.Printf("Error closing file: %v", err)
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
		urlType = "default" // Use default if not specified
	}

	// Convert string to URLType
	parsedURLType, err := ParseURLType(urlType)
	if err != nil {
		http.Error(w, "Invalid URL type", http.StatusBadRequest)
		return
	}

	// Create upload request
	uploadReq := &UploadRequest{
		File:    file,
		Header:  header,
		URLType: parsedURLType,
		UserID:  userContext.ID,
	}

	response, err := h.service.UploadFile(r.Context(), uploadReq)
	if err != nil {
		log.Printf("Error uploading file: %v", err)
		http.Error(w, "Error uploading file", http.StatusInternalServerError)
		return
	}

	// Render success template
	if err := pages.UploadSuccess(response.FileUrl, response.OriginalName).Render(r.Context(), w); err != nil {
		log.Printf("Error rendering success template: %v", err)
		http.Error(w, "Error rendering response", http.StatusInternalServerError)
		return
	}
}

// HandleServeFile serves the uploaded file
func (h *Handler) HandleServeFile(w http.ResponseWriter, r *http.Request) {
	urlvalue := chi.URLParam(r, "fileUrl")
	log.Printf("Got Serve File Request: %s", urlvalue)
	if urlvalue == "" {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	file, err := h.service.GetFile(r.Context(), urlvalue)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	log.Printf("Serving file: %s", file.OriginalName)

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

	path := filepath.Join(h.service.config.UploadDirectory, file.UniqueFilename)
	log.Printf("Serving file from path: %s", path)
	http.ServeFile(w, r, path)

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
		log.Printf("Error encoding response: %v", err)
	}
}

// HandleAPIUpload handles file upload through the API with minimal configuration
func (h *Handler) HandleAPIUpload(w http.ResponseWriter, r *http.Request) {
	log.Printf("API Upload request from %s", r.RemoteAddr)
	// Get user from context
	userContext := userctx.GetUserFromContext(r.Context())
	log.Printf("User context: %v", userContext)
	if userContext == nil {
		sendAPIResponse(w, http.StatusUnauthorized, false, "", errors.New("unauthorized"))
		return
	}

	// Check content length against max size before reading the file
	if r.ContentLength > h.service.config.MaxUploadSize {
		sendAPIResponse(w, http.StatusRequestEntityTooLarge, false, "", ErrFileTooLarge)
		return
	}

	// Get file from request
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
			log.Printf("Error closing file: %v", err)
		}
	}(file)

	// Validate file size again after reading the header
	if header.Size > h.service.config.MaxUploadSize {
		sendAPIResponse(w, http.StatusRequestEntityTooLarge, false, "", ErrFileTooLarge)
		return
	}

	// Parse URL type from header (optional)
	urlType := URLTypeDefault
	if typeHeader := r.Header.Get("Url-Type"); typeHeader != "" {
		parsedType, err := ParseURLType(typeHeader)
		if err != nil {
			sendAPIResponse(w, http.StatusBadRequest, false, "", ErrInvalidURLType)
			return
		}
		urlType = parsedType
	}

	// Create upload request
	uploadReq := &UploadRequest{
		File:    file,
		Header:  header,
		URLType: urlType,
		UserID:  userContext.ID,
	}

	// Process upload
	response, err := h.service.UploadFile(r.Context(), uploadReq)
	if err != nil {
		// Log the internal error but don't send it to the client
		log.Printf("Upload error: %v", err)
		sendAPIResponse(w, http.StatusInternalServerError, false, "", errors.New("upload failed"))
		return
	}

	// Return success response
	sendAPIResponse(w, http.StatusOK, true, response.FileUrl, nil)
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
		log.Printf("Error fetching files: %v", err)
		http.Error(w, "Error fetching files", http.StatusInternalServerError)
		return
	}

	// Get total count for pagination
	total, err := h.service.GetUserFilesCount(r.Context(), user.ID)
	if err != nil {
		log.Printf("Error fetching file count: %v", err)
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
		log.Printf("Error rendering file list: %v", err)
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
	log.Printf("INFO: User: %v is attempting to delete File: %v", user, chi.URLParam(r, "fileID"))
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		log.Printf("Unauthorized")
		return
	}

	fileID := chi.URLParam(r, "fileID")
	if fileID == "" {
		http.Error(w, "Missing file ID", http.StatusBadRequest)
		log.Printf("Missing file ID")
		return
	}

	// Parse file ID
	id, err := uuid.Parse(fileID)
	if err != nil {
		http.Error(w, "Invalid file ID", http.StatusBadRequest)
		log.Printf("Invalid file ID: %v", err)
		return
	}

	// Delete the file
	err = h.service.DeleteFileByID(r.Context(), id, user.ID)
	if err != nil {
		switch {
		case errors.Is(err, ErrUnauthorized):
			http.Error(w, "Unauthorized", http.StatusForbidden)
			log.Printf("Unauthorized")
		case errors.Is(err, ErrNoRows):
			http.Error(w, "File not found", http.StatusNotFound)
			log.Printf("File not found")
		default:
			log.Printf("Error deleting file: %v", err)
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
