package uploader

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
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

// handles file upload and returns URLs for accessing the files
func (h *Handler) handleUpload(w http.ResponseWriter, r *http.Request) {
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Invalid File", http.StatusBadRequest)
		return
	}
	defer file.Close()

	userContext := userctx.GetUserFromContext(r.Context())
  if userContext == nil {
    http.Error(w, "Unauthorized", http.StatusUnauthorized)
    return
  }

  if header.Header.Get("Content-Type") == "" {
    buff := make([]byte, 512)
    n, err := file.Read(buff)
    if err != nil && err != io.EOF {
      http.Error(w, "Error reading file", http.StatusInternalServerError)
      return
    }

    if _,err := file.Seek(0,0); err != nil {
      http.Error(w, "Error processing file", http.StatusInternalServerError)
      return
    }
    filetype := http.DetectContentType(buff[:n])
    header.Header.Set("Content-Type", filetype)
  }

  response, err := h.service.UploadFile(file, header, userContext.ID)
  if err != nil {
    log.Printf("Error uploading file: %v", err)
    http.Error(w, "Error uploading file", http.StatusInternalServerError)
    return
  }

  // Return JSON response
  w.Header().Set("Content-Type", "application/json")
  if err := json.NewEncoder(w).Encode(response); err != nil {
    log.Printf("Error encoding response: %v", err)
  }
}

func (h *Handler) HandleServeFile(w http.ResponseWriter, r *http.Request) {
  filename := chi.URLParam(r, "filename")
  if filename == "" {
    http.Error(w, "File not found", http.StatusNotFound)
    return
  }

  file, err := h.service.GetFile(filename)
  if err != nil {
    http.Error(w, "File not found", http.StatusNotFound)
    return
  }

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

  if strings.HasPrefix(contentType, "video/") || strings.HasPrefix(contentType, "audio/") {
    http.ServeFile(w,r,filepath.Join("uploads", filename))
    return
  }

  if err := h.service.ServeFile(w,filename); err != nil {
    log.Printf("Error serving file: %v", err)
    http.Error(w, "Error serving file", http.StatusInternalServerError)
    return
  }
}
