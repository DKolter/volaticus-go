package uploader

import (
	"context"
	"fmt"
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

	"github.com/google/uuid"
)

type Service interface {
	UploadFile(ctx context.Context, req *UploadRequest) (*models.CreateFileResponse, error)
	GetFile(ctx context.Context, fileurl string) (*models.UploadedFile, error)
	GetUserFiles(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.UploadedFile, error)
	GetUserFilesCount(ctx context.Context, userID uuid.UUID) (int, error)
	DeleteFileByID(ctx context.Context, fileID, userID uuid.UUID) error
	GetFileStats(ctx context.Context, userID uuid.UUID) (*models.FileStats, error)
	ValidateFile(ctx context.Context, file multipart.File, header *multipart.FileHeader) *FileValidationResult
	CleanupExpiredFiles(ctx context.Context) error
}

type service struct {
	repo         Repository
	config       config.Config
	urlGenerator *URLGenerator
}

func NewService(repo Repository, config *config.Config) *service {
	// Make sure that directory exists
	if err := os.MkdirAll(config.UploadDirectory, 0755); err != nil {
		log.Fatalf("Failed to create upload directory: %v", err)
	}

	return &service{
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

// FileValidationResult contains validation results TODO: json tags
type FileValidationResult struct {
	IsValid     bool
	FileName    string
	FileSize    int64
	ContentType string
	Error       string
}

// VerifyFile checks if the file meets upload requirements
func (s *service) VerifyFile(ctx context.Context, file multipart.File, header *multipart.FileHeader) *FileValidationResult {
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

func (s *service) UploadFile(ctx context.Context, req *UploadRequest) (*models.CreateFileResponse, error) {
	// Verify file first
	validation := s.VerifyFile(ctx, req.File, req.Header)
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

	if ext != "" && !strings.Contains(urlValue, ext) {
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
	if err := s.repo.CreateWithURL(ctx, uploadedFile, urlValue); err != nil {
		// Rollback file creation
		log.Printf("Error saving to database, rolling back file creation: %v", err)
		err := os.Remove(filepath.Join(s.config.UploadDirectory, uniqueFilename))
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
func (s *service) GetFile(ctx context.Context, fileurl string) (*models.UploadedFile, error) {

	file, err := s.repo.GetByURLValue(ctx, fileurl)
	if err != nil {
		return nil, fmt.Errorf("retrieving file: %w", err)
	}

	if err := s.repo.IncrementAccessCount(ctx, file.ID); err != nil {
		// Log but don't fail the request if increment fails
		log.Printf("Error incrementing access count: %v", err)
	}

	return file, nil
}

// saveFile saves the uploaded file to disk
func (s *service) saveFile(file multipart.File, filename string) error {
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

func StartExpiredFilesWorker(ctx context.Context, svc *service, interval time.Duration) {
	// Initial cleanup
	if err := svc.CleanupExpiredFiles(ctx); err != nil {
		log.Printf("Error cleaning up expired files: %v", err)
	}
	if err := svc.CleanupOrphanedFiles(ctx); err != nil {
		log.Printf("Error cleaning up orphaned files: %v", err)
	}

	ticker := time.NewTicker(interval)

	go func() {
		for range ticker.C {
			if err := svc.CleanupExpiredFiles(ctx); err != nil {
				log.Printf("Error cleaning up expired files: %v", err)
			}
		}
	}()
}

// DeleteFile deletes the physical file from the disk
func (s *service) DeleteFile(filename string) error {
	filePath := filepath.Join(s.config.UploadDirectory, filename)
	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("deleting file: %w", err)
	}
	return nil
}

// DeleteFileByID deletes the file from the database and disk by ID
func (s *service) DeleteFileByID(ctx context.Context, fileID, userID uuid.UUID) error {
	// Get file details first to get the filename
	file, err := s.repo.GetByID(ctx, fileID)
	if err != nil {
		return fmt.Errorf("getting file details: %w", err)
	}

	// Check if the file belongs to the user
	if file.UserID != userID {
		return ErrUnauthorized
	}

	// Delete from database
	if err := s.repo.Delete(ctx, fileID); err != nil {
		return fmt.Errorf("deleting file from database: %w", err)
	}

	// Delete physical file
	path := filepath.Join(s.config.UploadDirectory, file.UniqueFilename)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		// Log but don't fail if physical file is already gone
		log.Printf("Error deleting physical file %s: %v", path, err)
	}

	return nil
}

// CleanupExpiredFiles retrieves and deletes expired files.
func (s *service) CleanupExpiredFiles(ctx context.Context) error {
	files, err := s.repo.GetExpiredFiles(ctx)
	if err != nil {
		return err
	}
	if len(files) > 0 {
		log.Printf("Found %d expired files\n", len(files))
	}

	for _, file := range files {
		if err := s.repo.Delete(ctx, file.ID); err != nil {
			log.Printf("Error deleting file %s from database: %v\n", file.ID, err)
			continue
		}
		if err := s.DeleteFile(file.UniqueFilename); err != nil {
			log.Printf("Error deleting file %s from disk: %v\n", file.UniqueFilename, err)
		}
	}
	return nil
}

// CleanupOrphanedFiles deletes files in the uploadDir that are not in the database and vice versa.
func (s *service) CleanupOrphanedFiles(ctx context.Context) error {
	filesInDir, err := os.ReadDir(s.config.UploadDirectory)
	if err != nil {
		return fmt.Errorf("reading upload directory: %w", err)
	}

	filesInDB, err := s.repo.GetAllFiles(ctx)
	if err != nil {
		return fmt.Errorf("retrieving files from database: %w", err)
	}

	fileMap := make(map[string]bool)
	for _, file := range filesInDir {
		fileMap[file.Name()] = true
	}

	var deletedCount int
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

	var missingFiles []string
	for _, file := range filesInDB {
		if !fileMap[file.UniqueFilename] {
			missingFiles = append(missingFiles, file.UniqueFilename)
		}
	}
	if len(missingFiles) > 0 {
		log.Printf("Files in database but missing in directory: %v", missingFiles)
		for _, file := range missingFiles {
			if err := s.repo.DeleteByUniqueName(ctx, file); err != nil {
				log.Printf("Error deleting database entry for missing file %s: %v", file, err)
			}
		}
	}

	return nil
}

// Update the GetUserFiles method to include stats
func (s *service) GetUserFiles(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.UploadedFile, error) {
	files, err := s.repo.GetUserFiles(ctx, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("getting user files: %w", err)
	}

	return files, nil
}

func (s *service) GetUserFilesCount(ctx context.Context, userID uuid.UUID) (int, error) {
	count, err := s.repo.GetUserFilesCount(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("getting user files count: %w", err)
	}
	return count, nil
}

// GetFileStats retrieves statistics about uploaded files
func (s *service) GetFileStats(ctx context.Context, userID uuid.UUID) (*models.FileStats, error) {
	return s.repo.GetFileStats(ctx, userID)
}
