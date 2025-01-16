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
	"volaticus-go/internal/models"

	"github.com/google/uuid"
)

const (
	// TODO: Move to config
	uploadDir = "./uploads"
	maxSize   = 100 << 20 // 100MB
	expiresIn = 1 * time.Minute
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
	// TODO: Move to config
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

func (s *Service) UploadFile(req *UploadRequest) (*models.CreateFileResponse, error) {
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
	uploadedFile := &models.UploadedFile{
		ID:             uuid.New(),
		OriginalName:   req.Header.Filename,
		UniqueFilename: unixFilename,
		MimeType:       validation.ContentType,
		FileSize:       uint64(req.Header.Size),
		UserID:         req.UserID,
		CreatedAt:      time.Now(),
		AccessCount:    0,
		ExpiredAt:      time.Now().Add(expiresIn),
		URLValue:       urlValue,
	}

	// Save to database
	if err := s.repo.CreateWithURL(uploadedFile, urlValue); err != nil {
		os.Remove(filepath.Join(uploadDir, urlValue))
		return nil, fmt.Errorf("saving to database: %w", err)
	}

	return &models.CreateFileResponse{
		FileUrl:      fmt.Sprintf("%s/f/%s", s.baseURL, urlValue),
		OriginalName: req.Header.Filename,
		UnixFilename: urlValue,
	}, nil
}

// GetFile retrieves file information using the urlvalue
func (s *Service) GetFile(fileurl string) (*models.UploadedFile, error) {

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

func StartExpiredFilesWorker(svc *Service, interval time.Duration) {
	// Initial cleanup
	if err := svc.CleanupOrphanedFiles(); err != nil {
		log.Printf("Error cleaning up orphaned files: %v", err)
	}

	ticker := time.NewTicker(interval)

	go func() {
		for range ticker.C {
			if err := svc.CleanupExpiredFiles(); err != nil {
				log.Printf("Error cleaning up expired files: %v", err)
			}
		}
	}()
}

// DeleteFile deletes the physical file from the disk
func (s *Service) DeleteFile(filename string) error {
	filePath := filepath.Join(uploadDir, filename)
	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("deleting file: %w", err)
	}
	return nil
}

// CleanupExpiredFiles retrieves and deletes expired files.
func (s *Service) CleanupExpiredFiles() error {
	files, err := s.repo.GetExpiredFiles()
	log.Printf("Found %d expired files\n", len(files))
	if err != nil {
		return err
	}
	for _, file := range files {
		if err := s.repo.Delete(file.ID); err != nil {
			log.Printf("Error deleting file %s from database: %v\n", file.ID, err)
			continue
		}
		if err := s.DeleteFile(file.UniqueFilename); err != nil {
			log.Printf("Error deleting file %s from disk: %v\n", file.UniqueFilename, err)
		}
	}
	return nil
}

// CleanupOrphanedFiles deletes files in the uploadDir that are not in the database.
func (s *Service) CleanupOrphanedFiles() error {
	filesInDir, err := os.ReadDir(uploadDir)
	if err != nil {
		return fmt.Errorf("reading upload directory: %w", err)
	}

	filesInDB, err := s.repo.GetAllFiles()
	if err != nil {
		return fmt.Errorf("retrieving files from database: %w", err)
	}

	fileMap := make(map[string]bool)
	for _, file := range filesInDB {
		fileMap[file.UniqueFilename] = true
	}
	var deletedCount = 0
	for _, file := range filesInDir {
		if !fileMap[file.Name()] {
			if err := s.DeleteFile(file.Name()); err != nil {
				log.Printf("Error deleting orphaned file %s: %v", file.Name(), err)
			} else {
				deletedCount++
			}
		}
	}
	log.Printf("Deleted %d orphaned files", deletedCount)

	return nil
}
