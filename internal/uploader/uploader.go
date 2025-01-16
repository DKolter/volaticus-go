package uploader

import (
	"volaticus-go/internal/models"

	"github.com/google/uuid"
)

type Repository interface {
	CreateWithURL(file *models.UploadedFile, urlValue string) error
	GetAllFiles() ([]*models.UploadedFile, error)
	GetByUniqueFilename(code string) (*models.UploadedFile, error)
	GetByURLValue(urlValue string) (*models.UploadedFile, error)
	IncrementAccessCount(id uuid.UUID) error
	GetExpiredFiles() ([]*models.UploadedFile, error)
	Delete(id uuid.UUID) error
}
