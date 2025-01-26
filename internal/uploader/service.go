package uploader

import (
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"
	"volaticus-go/internal/common/models"
	"volaticus-go/internal/config"
	"volaticus-go/internal/storage"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

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

type Service interface {
	// UploadFile handles file uploads
	UploadFile(ctx context.Context, req *UploadRequest) (*models.CreateFileResponse, error)

	// GetFile retrieves file information
	GetFile(ctx context.Context, fileUrl string) (*models.UploadedFile, error)

	// ServeFile serves a file to an HTTP response
	ServeFile(ctx context.Context, w http.ResponseWriter, file *models.UploadedFile) error

	// DeleteFileByID deletes a file
	DeleteFileByID(ctx context.Context, fileID, userID uuid.UUID) error

	// GetFileStats returns statistics about uploaded files
	GetFileStats(ctx context.Context, userID uuid.UUID) (*models.FileStats, error)

	// CleanupExpiredFiles removes expired files
	CleanupExpiredFiles(ctx context.Context) error

	// SyncStorageWithDatabase ensures storage and database are in sync
	SyncStorageWithDatabase(ctx context.Context) error

	// ValidateFile validates an uploaded file
	ValidateFile(ctx context.Context, file multipart.File, header *multipart.FileHeader) *FileValidationResult
}

type service struct {
	repo         Repository
	config       *config.Config
	storage      storage.StorageProvider
	urlGenerator *URLGenerator
}

func NewService(repo Repository, config *config.Config, storage storage.StorageProvider) *service {
	return &service{
		repo:         repo,
		config:       config,
		storage:      storage,
		urlGenerator: NewURLGenerator(),
	}
}

// UploadFile handles the file upload process
func (s *service) UploadFile(ctx context.Context, req *UploadRequest) (*models.CreateFileResponse, error) {
	// Verify file first
	validation := s.ValidateFile(ctx, req.File, req.Header)
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

	// Upload file to storage
	if _, err := s.storage.Upload(ctx, req.File, uniqueFilename); err != nil {
		return nil, fmt.Errorf("saving file to storage: %w", err)
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
		// Rollback file creation if database save fails
		if delErr := s.storage.Delete(ctx, uniqueFilename); delErr != nil {
			log.Error().
				Err(delErr).
				Str("filename", uniqueFilename).
				Msg("failed to clean up file after failed database save")
		}
		return nil, fmt.Errorf("saving to database: %w", err)
	}

	// Get URL for response
	url, _, err := s.storage.GetURL(ctx, uniqueFilename)
	if err != nil {
		return nil, fmt.Errorf("getting file URL: %w", err)
	}

	return &models.CreateFileResponse{
		FileUrl:      url,
		OriginalName: req.Header.Filename,
		UnixFilename: urlValue,
	}, nil
}

// GetFile retrieves file information
func (s *service) GetFile(ctx context.Context, fileUrl string) (*models.UploadedFile, error) {
	file, err := s.repo.GetByURLValue(ctx, fileUrl)
	if err != nil {
		return nil, fmt.Errorf("retrieving file: %w", err)
	}

	// Check if file is expired
	if !file.ExpiresAt.IsZero() && time.Now().After(file.ExpiresAt) {
		return nil, fmt.Errorf("file has expired")
	}

	if err := s.repo.IncrementAccessCount(ctx, file.ID); err != nil {
		log.Error().
			Err(err).
			Str("file_id", file.ID.String()).
			Msg("failed to increment access count")
	}

	return file, nil
}

// ServeFile serves the file through the storage provider
func (s *service) ServeFile(ctx context.Context, w http.ResponseWriter, file *models.UploadedFile) error {
	return s.storage.Stream(ctx, file.UniqueFilename, w)
}

// ValidateFile checks if the file meets upload requirements
func (s *service) ValidateFile(ctx context.Context, file multipart.File, header *multipart.FileHeader) *FileValidationResult {
	result := &FileValidationResult{
		FileName: header.Filename,
		FileSize: header.Size,
	}

	if header.Size > s.config.UploadMaxSize {
		result.Error = fmt.Sprintf("File too large (max %d MB)", s.config.UploadMaxSize/1024/1024)
		return result
	}

	// Read first 512 bytes for content type detection
	buff := make([]byte, 512)
	if _, err := file.Read(buff); err != nil {
		result.Error = "Error reading file"
		return result
	}

	if _, err := file.Seek(0, 0); err != nil {
		result.Error = "Error processing file"
		return result
	}

	result.ContentType = http.DetectContentType(buff)
	result.IsValid = true
	return result
}

// GetUserFiles retrieves all files for a user
func (s *service) GetUserFiles(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.UploadedFile, error) {
	return s.repo.GetUserFiles(ctx, userID, limit, offset)
}

// GetUserFilesCount gets the total number of files for a user
func (s *service) GetUserFilesCount(ctx context.Context, userID uuid.UUID) (int, error) {
	return s.repo.GetUserFilesCount(ctx, userID)
}

// DeleteFileByID deletes a file
func (s *service) DeleteFileByID(ctx context.Context, fileID, userID uuid.UUID) error {
	file, err := s.repo.GetByID(ctx, fileID)
	if err != nil {
		return fmt.Errorf("getting file details: %w", err)
	}

	if file.UserID != userID {
		return ErrUnauthorized
	}

	if err := s.storage.Delete(ctx, file.UniqueFilename); err != nil {
		return fmt.Errorf("deleting file from storage: %w", err)
	}

	if err := s.repo.Delete(ctx, fileID); err != nil {
		log.Error().
			Err(err).
			Str("file_id", fileID.String()).
			Str("filename", file.UniqueFilename).
			Msg("file deleted from storage but database deletion failed")
		return fmt.Errorf("deleting file from database: %w", err)
	}

	return nil
}

// ListStorageFiles lists all files in storage
func (s *service) ListStorageFiles(ctx context.Context, prefix string) ([]storage.FileInfo, error) {
	files, err := s.storage.ListFiles(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("listing files from storage: %w", err)
	}

	dbFiles, err := s.repo.GetAllFiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("retrieving database files: %w", err)
	}

	dbFileMap := make(map[string]*models.UploadedFile)
	for _, file := range dbFiles {
		dbFileMap[file.UniqueFilename] = file
	}

	var validFiles []storage.FileInfo
	for _, file := range files {
		if _, exists := dbFileMap[file.Name]; exists {
			validFiles = append(validFiles, file)
		} else {
			log.Warn().
				Str("filename", file.Name).
				Msg("found orphaned file in storage")
		}
	}

	return validFiles, nil
}

// CleanupExpiredFiles removes expired files
func (s *service) CleanupExpiredFiles(ctx context.Context) error {
	files, err := s.repo.GetExpiredFiles(ctx)
	if err != nil {
		return fmt.Errorf("getting expired files: %w", err)
	}

	for _, file := range files {
		if err := s.storage.Delete(ctx, file.UniqueFilename); err != nil {
			log.Error().
				Err(err).
				Str("filename", file.UniqueFilename).
				Msg("failed to delete expired file from storage")
			continue
		}

		if err := s.repo.Delete(ctx, file.ID); err != nil {
			log.Error().
				Err(err).
				Str("filename", file.UniqueFilename).
				Str("file_id", file.ID.String()).
				Msg("failed to delete expired file record")
		}
	}

	return nil
}

// SyncStorageWithDatabase ensures storage and database are in sync
func (s *service) SyncStorageWithDatabase(ctx context.Context) error {
	storageFiles, err := s.storage.ListFiles(ctx, "")
	if err != nil {
		return fmt.Errorf("listing storage files: %w", err)
	}

	dbFiles, err := s.repo.GetAllFiles(ctx)
	if err != nil {
		return fmt.Errorf("getting database files: %w", err)
	}

	storageMap := make(map[string]storage.FileInfo)
	for _, file := range storageFiles {
		storageMap[file.Name] = file
	}

	dbMap := make(map[string]*models.UploadedFile)
	for _, file := range dbFiles {
		dbMap[file.UniqueFilename] = file
	}

	// Find and handle orphaned storage files
	for name := range storageMap {
		if _, exists := dbMap[name]; !exists {
			log.Info().
				Str("filename", name).
				Msg("deleting orphaned storage file")
			if err := s.storage.Delete(ctx, name); err != nil {
				log.Error().
					Err(err).
					Str("filename", name).
					Msg("failed to delete orphaned file")
			}
		}
	}

	for name, file := range dbMap {
		if _, exists := storageMap[name]; !exists {
			log.Info().
				Str("filename", name).
				Str("file_id", file.ID.String()).
				Msg("deleting orphaned database record")
			if err := s.repo.Delete(ctx, file.ID); err != nil {
				log.Error().
					Err(err).
					Str("filename", name).
					Str("file_id", file.ID.String()).
					Msg("failed to delete orphaned record")
			}
		}
	}

	return nil
}

// GetFileStats retrieves statistics about uploaded files
func (s *service) GetFileStats(ctx context.Context, userID uuid.UUID) (*models.FileStats, error) {
	return s.repo.GetFileStats(ctx, userID)
}

// GetMaxUploadSize returns the configured maximum upload size
func (s *service) GetMaxUploadSize() int64 {
	return s.config.UploadMaxSize
}
