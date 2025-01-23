package dashboard

import (
	"context"
	"github.com/google/uuid"
	"volaticus-go/internal/common/models"
)

type Service interface {
	GetDashboardStats(ctx context.Context, userID uuid.UUID) (*models.DashboardStats, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{
		repo: repo,
	}
}

func (s *service) GetDashboardStats(ctx context.Context, userID uuid.UUID) (*models.DashboardStats, error) {
	// Get main statistics
	stats, err := s.repo.GetDashboardStats(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Get recent URLs
	recentURLs, err := s.repo.GetRecentURLs(ctx, userID, 5)
	if err != nil {
		return nil, err
	}
	stats.RecentURLs = recentURLs

	// Get recent files
	recentFiles, err := s.repo.GetRecentFiles(ctx, userID, 5)
	if err != nil {
		return nil, err
	}
	stats.RecentFiles = recentFiles

	return stats, nil
}
