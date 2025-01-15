package uploader

import (
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	uploadDir = "./uploads"
	maxSize   = 100 << 20 // 100MB
)

type Service struct {
	repo         Repository
	baseURL      string
	urlGenerator *URLGenerator
}

func NewService(repo Repository, baseURL string) *Service {
	// Make sure that directory exists
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Fatalf("Failed to create upload directory: %v", err)
	}

	return &Service{
		repo:         repo,
		baseURL:      baseURL,
		urlGenerator: NewURLGenerator(),
	}
}

// UploadRequest represents file upload parameters
type UploadRequest struct {
	File    multipart.File
	Header  *multipart.FileHeader
	URLType URLType
	UserID  uuid.UUID
}

// FileValidationResult contains validation results
type FileValidationResult struct {
	IsValid     bool
	FileName    string
	FileSize    int64
	ContentType string
	Error       string
}

// VerifyFile checks if the file meets upload requirements
func (s *Service) VerifyFile(file multipart.File, header *multipart.FileHeader) *FileValidationResult {
	result := &FileValidationResult{
		FileName: header.Filename,
		FileSize: header.Size,
	}

	// Check file size
	if header.Size > maxSize {
		result.Error = "File too large (max 100MB)"
		return result
	}

	// Read first 512 bytes for content type detection
	buff := make([]byte, 512)
	if _, err := file.Read(buff); err != nil {
		result.Error = "Error reading file"
		return result
	}

	// Reset file pointer
	if _, err := file.Seek(0, 0); err != nil {
		result.Error = "Error processing file"
		return result
	}

	// Detect content type
	contentType := http.DetectContentType(buff)
	result.ContentType = contentType

	// List of allowed MIME types
	allowedTypes := map[string]bool{
		"image/jpeg":         true,
		"image/png":          true,
		"image/gif":          true,
		"application/pdf":    true,
		"text/plain":         true,
		"application/msword": true,
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
		"video/mp4":                    true,
		"video/webm":                   true,
		"audio/mpeg":                   true,
		"audio/wav":                    true,
		"audio/x-wav":                  true,
		"audio/webm":                   true,
		"application/zip":              true,
		"application/x-zip-compressed": true,
	}

	if !allowedTypes[contentType] {
		result.Error = fmt.Sprintf("File type %s not allowed", contentType)
		return result
	}

	result.IsValid = true
	return result
}

func (s *Service) UploadFile(req *UploadRequest) (*CreateFileResponse, error) {
	// Verify file first
	validation := s.VerifyFile(req.File, req.Header)
	if !validation.IsValid {
		return nil, fmt.Errorf("file validation failed: %s", validation.Error)
	}

	// Generate URL based on selected type
	urlValue, err := s.urlGenerator.GenerateURL(req.URLType, req.Header.Filename)
	if err != nil {
		return nil, fmt.Errorf("error generating URL: %w", err)
	}

	// Add extension if not present
	ext := filepath.Ext(req.Header.Filename)
	if ext == "" {
		ext = getExtensionFromMimeType(validation.ContentType)
	}
	if ext != "" && !strings.HasSuffix(urlValue, ext) {
		urlValue = urlValue + ext
	}

	unixTimestamp := uint64(time.Now().UnixNano())
	unixFilename := fmt.Sprintf("%d%s", unixTimestamp, ext)

	// Create physical file
	if err := s.saveFile(req.File, unixFilename); err != nil {
		return nil, fmt.Errorf("saving file: %w", err)
	}

	// Create uploaded file record
	uploadedFile := &UploadedFile{
		ID:           uuid.New(),
		OriginalName: req.Header.Filename,
		UnixFilename: unixTimestamp,
		MimeType:     validation.ContentType,
		FileSize:     uint64(req.Header.Size),
		UserID:       req.UserID,
		CreatedAt:    time.Now(),
		AccessCount:  0,
		ExpiredAt:    time.Now().AddDate(0, 0, 7), // 7 days expiry
	}

	// Create URL record
	urls := []FileURL{
		{
			ID:        uuid.New(),
			FileID:    uploadedFile.ID,
			UrlType:   req.URLType.String(),
			UrlValue:  urlValue,
			CreatedAt: time.Now(),
		},
	}

	// Save to database
	if err := s.repo.CreateWithURLs(uploadedFile, urls); err != nil {
		// Clean up the file if database operation fails
		os.Remove(filepath.Join(uploadDir, urlValue))
		return nil, fmt.Errorf("saving to database: %w", err)
	}

	return &CreateFileResponse{
		// http://localhost:8080/localhost/f/beautiful-crimson-rabbit.png
		FileUrl:      fmt.Sprintf("%s/f/%s", s.baseURL, urlValue),
		OriginalName: req.Header.Filename,
		UnixFilename: urlValue,
	}, nil
}

// GetFile retrieves file information using the urlvalue
func (s *Service) GetFile(fileurl string) (*UploadedFile, error) {

	file, err := s.repo.GetByURLValue(fileurl)
	if err != nil {
		return nil, fmt.Errorf("retrieving file: %w", err)
	}

	if err := s.repo.IncrementAccessCount(file.ID); err != nil {
		// Log but don't fail the request if increment fails
		log.Printf("Error incrementing access count: %v", err)
	}

	return file, nil
}

// saveFile saves the uploaded file to disk
func (s *Service) saveFile(file multipart.File, filename string) error {
	dst, err := os.Create(filepath.Join(uploadDir, filename))
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	return nil
}

// ServeFile writes the contents of a file to the provided writer
func (s *Service) ServeFile(w io.Writer, filename string) error {
	filepath := filepath.Join(uploadDir, filename)
	file, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(w, file)
	if err != nil {
		return fmt.Errorf("copying file: %w", err)
	}

	return nil
}

// getExtensionFromMimeType returns a file extension for common mime types
func getExtensionFromMimeType(mimeType string) string {
	switch mimeType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "application/pdf":
		return ".pdf"
	case "text/plain":
		return ".txt"
	case "application/msword":
		return ".doc"
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return ".docx"
	case "video/mp4":
		return ".mp4"
	case "video/webm":
		return ".webm"
	case "audio/mpeg":
		return ".mp3"
	case "audio/wav", "audio/x-wav":
		return ".wav"
	case "audio/webm":
		return ".weba"
	case "application/zip", "application/x-zip-compressed":
		return ".zip"
	default:
		return "" // If unknown, don't add an extension
	}
}
