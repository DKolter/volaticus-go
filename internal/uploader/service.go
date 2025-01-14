package uploader

import (
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

const (
	uploadDir = "./uploads"
	alphabet  = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

type Service struct {
	repo    Repository
	baseURL string
}

func NewService(repo Repository, baseURL string) *Service {
	// Make sure that directory exists
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Fatalf("Failed to create upload directory: %v", err)
	}

	return &Service{
		repo:    repo,
		baseURL: baseURL,
	}
}

func (s *Service) UploadFile(file multipart.File, header *multipart.FileHeader, userID uuid.UUID) (*CreateFileResponse, error) {
	// Generate Unix timestamp for file
	unixFilename := uint64(time.Now().UnixNano())

	// Create uploaded file
	uploadedFile := &UploadedFile{
		ID:           uuid.New(),
		OriginalName: header.Filename,
		UnixFilename: unixFilename,
		MimeType:     header.Header.Get("Content-Type"),
		FileSize:     uint64(header.Size),
		UserID:       userID,
		CreatedAt:    time.Now(),
		AccessCount:  0,
		ExpiredAt:    time.Now().AddDate(0, 0, 7),
	}

	// Create physical file
	dst, err := os.Create(filepath.Join(uploadDir, fmt.Sprintf("%d", unixFilename)))
	if err != nil {
		return nil, fmt.Errorf("creating file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		return nil, fmt.Errorf("saving file: %w", err)
	}

	// Create URL using Unix timestamp
	urls := []FileURL{
    {
      ID:        uuid.New(),
      FileID:    uploadedFile.ID,
      UrlType:   URLTypeDefault.String(),
      UrlValue:  fmt.Sprintf("%d", unixFilename),
      CreatedAt: time.Now(),
    },
	}

	// Save to database
	if err := s.repo.CreateWithURLs(uploadedFile, urls); err != nil {
		os.Remove(filepath.Join(uploadDir, fmt.Sprintf("%d", unixFilename)))
		return nil, fmt.Errorf("saving to database: %w", err)
	}

	// Create response
	return &CreateFileResponse{
    FileUrl:      fmt.Sprintf("%s/f/%d", s.baseURL, unixFilename),
		OriginalName: header.Filename,
		UnixFilename: fmt.Sprintf("%d", unixFilename),
	}, nil
}

func (s *Service) GetFile(filename string) (*UploadedFile, error) {
	file, err := s.repo.GetByUnixFilename(filename)
	if err != nil {
		return nil, err
	}

	if err := s.repo.IncrementAccessCount(file.ID); err != nil {
		log.Printf("Error incrementing access count: %v", err)
	}

	return file, nil
}

func (s *Service) ServeFile(w io.Writer, filename string) error {
	filepath := filepath.Join(uploadDir, filename)
	file, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(w, file)
	return err
}
