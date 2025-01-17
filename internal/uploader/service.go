package uploader

import (
	"fmt"
	"github.com/google/uuid"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"volaticus-go/internal/common/models"
	"volaticus-go/internal/config"
)

type Service struct {
	repo         Repository
	config       config.Config
	urlGenerator *URLGenerator
}

func NewService(repo Repository, config *config.Config) *Service {
	// Make sure that directory exists
	if err := os.MkdirAll(config.UploadDirectory, 0755); err != nil {
		log.Fatalf("Failed to create upload directory: %v", err)
	}

	return &Service{
		repo:         repo,
		config:       *config,
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
	if header.Size > s.config.MaxUploadSize {
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

	// Here we could add more checks like file type, etc.
	// and allow/deny based on that, for example only allow images, textfiles, etc.

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
	uniqueFilename := fmt.Sprintf("%d%s", unixTimestamp, ext)

	// Create physical file
	if err := s.saveFile(req.File, uniqueFilename); err != nil {
		return nil, fmt.Errorf("saving file: %w", err)
	}

	// Create uploaded file record
	uploadedFile := &models.UploadedFile{
		ID:             uuid.New(),
		OriginalName:   req.Header.Filename,
		UniqueFilename: uniqueFilename,
		MimeType:       validation.ContentType,
		FileSize:       uint64(req.Header.Size),
		UserID:         req.UserID,
		CreatedAt:      time.Now(),
		AccessCount:    0,
		ExpiresAt:      time.Now().Add(time.Duration(s.config.UploadExpiresIn) * time.Hour),
		URLValue:       urlValue,
	}

	// Save to database
	if err := s.repo.CreateWithURL(uploadedFile, urlValue); err != nil {
		err := os.Remove(filepath.Join(s.config.UploadDirectory, urlValue))
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("saving to database: %w", err)
	}

	return &models.CreateFileResponse{
		FileUrl:      fmt.Sprintf("%s/f/%s", s.config.BaseURL, urlValue),
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
	dst, err := os.Create(filepath.Join(s.config.UploadDirectory, filename))
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer func(dst *os.File) {
		err := dst.Close()
		if err != nil {
			log.Printf("Error closing file: %v", err)
		}
	}(dst)

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
	filePath := filepath.Join(s.config.UploadDirectory, filename)
	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("deleting file: %w", err)
	}
	return nil
}

// CleanupExpiredFiles retrieves and deletes expired files.
func (s *Service) CleanupExpiredFiles() error {
	files, err := s.repo.GetExpiredFiles()
	if err != nil {
		return err
	}
	if len(files) > 0 {
		log.Printf("Found %d expired files\n", len(files))
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
	filesInDir, err := os.ReadDir(s.config.UploadDirectory)
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
	if deletedCount > 0 {
		log.Printf("Deleted %d orphaned files", deletedCount)
	}

	return nil
}
