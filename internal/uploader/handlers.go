package uploader

import (
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"volaticus-go/cmd/web/components"
	"volaticus-go/cmd/web/pages"
	userctx "volaticus-go/internal/context"

	"github.com/go-chi/chi/v5"
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
	result := h.service.VerifyFile(file, header)

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

	response, err := h.service.UploadFile(uploadReq)
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

	file, err := h.service.GetFile(urlvalue)
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
