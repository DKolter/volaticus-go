package shortener

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"
	"volaticus-go/internal/common/models"
	"volaticus-go/internal/database"
	"volaticus-go/internal/database/migrate"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	testHost     string
	testPort     string
	testDatabase string
	testUsername string
	testPassword string
)

func TestMain(m *testing.M) {
	// Start the container before running tests
	teardown, err := mustStartPostgresContainer()
	if err != nil {
		log.Fatalf("could not start postgres container: %v", err)
	}

	// Run the tests
	code := m.Run()

	// Cleanup after tests finish
	if teardown != nil {
		if err := teardown(context.Background()); err != nil {
			log.Printf("could not teardown postgres container: %v", err)
		}
	}

	os.Exit(code)
}

// Setup Postgres container for testing
func mustStartPostgresContainer() (func(context.Context) error, error) {
	ctx := context.Background()
	var (
		dbName = "testdb"
		dbPwd  = "testpass"
		dbUser = "testuser"
	)

	container, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:14-alpine"),
		postgres.WithDatabase(dbName),
		postgres.WithUsername(dbUser),
		postgres.WithPassword(dbPwd),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second)),
	)
	if err != nil {
		return nil, err
	}

	// Setze die globalen Testvariablen
	testDatabase = dbName
	testPassword = dbPwd
	testUsername = dbUser

	// Hole Host und Port
	host, err := container.Host(ctx)
	if err != nil {
		return container.Terminate, err
	}
	testHost = host

	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		return container.Terminate, err
	}
	testPort = port.Port()

	log.Printf("Started postgres container on %s:%s", testHost, testPort)
	return container.Terminate, nil
}

func setupTestDB(t *testing.T) *database.DB {
	cfg := database.Config{
		Host:     testHost,
		Port:     testPort,
		Database: testDatabase,
		Username: testUsername,
		Password: testPassword,
		Schema:   "public",
	}
	db, err := database.New(cfg)
	require.NoError(t, err)
	require.NotNil(t, db)

	// Run migrations to create necessary tables
	err = migrate.RunMigrations(db.DB)
	require.NoError(t, err)

	return db
}

// createTestUser creates a test user and returns its ID
func createTestUser(ctx context.Context, db *database.DB) (uuid.UUID, error) {
	userID := uuid.New()
	// Generiere eindeutige E-Mail und Benutzernamen mit UUID
	email := fmt.Sprintf("test-%s@example.com", uuid.New().String())
	username := fmt.Sprintf("testuser-%s", uuid.New().String())

	query := `
        INSERT INTO users (id, email, username, password_hash) 
        VALUES ($1, $2, $3, $4)
    `
	_, err := db.ExecContext(ctx, query,
		userID,
		email,
		username,
		"hashedpassword",
	)
	return userID, err
}

func TestRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	// Create a test user
	userID, err := createTestUser(ctx, db)
	require.NoError(t, err)

	t.Run("successful creation", func(t *testing.T) {
		url := &models.ShortenedURL{
			ID:          uuid.New(),
			UserID:      userID,
			OriginalURL: "https://example.com",
			ShortCode:   "abc123",
			CreatedAt:   time.Now(),
			IsActive:    true,
		}

		err := repo.Create(ctx, url)
		assert.NoError(t, err)

		// Verify URL was created
		stored, err := repo.GetByShortCode(ctx, url.ShortCode)
		assert.NoError(t, err)
		assert.Equal(t, url.OriginalURL, stored.OriginalURL)
		assert.Equal(t, url.ShortCode, stored.ShortCode)
		assert.Equal(t, url.UserID, stored.UserID)
	})

	t.Run("duplicate short code", func(t *testing.T) {
		url1 := &models.ShortenedURL{
			ID:          uuid.New(),
			UserID:      userID,
			OriginalURL: "https://example.com/1",
			ShortCode:   "duplicate",
			CreatedAt:   time.Now(),
			IsActive:    true,
		}

		url2 := &models.ShortenedURL{
			ID:          uuid.New(),
			UserID:      userID,
			OriginalURL: "https://example.com/2",
			ShortCode:   "duplicate", // Same short code
			CreatedAt:   time.Now(),
			IsActive:    true,
		}

		err := repo.Create(ctx, url1)
		assert.NoError(t, err)

		err = repo.Create(ctx, url2)
		assert.Error(t, err) // Should fail due to unique constraint
	})
}

func TestRepository_GetByShortCode(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()
	userID, err := createTestUser(ctx, db)
	require.NoError(t, err)

	t.Run("get existing url", func(t *testing.T) {
		url := &models.ShortenedURL{
			ID:          uuid.New(),
			UserID:      userID,
			OriginalURL: "https://example.com",
			ShortCode:   "test123",
			CreatedAt:   time.Now(),
			IsActive:    true,
		}

		err := repo.Create(ctx, url)
		require.NoError(t, err)

		found, err := repo.GetByShortCode(ctx, url.ShortCode)
		assert.NoError(t, err)
		assert.Equal(t, url.OriginalURL, found.OriginalURL)
	})

	t.Run("get non-existent url", func(t *testing.T) {
		_, err := repo.GetByShortCode(ctx, "nonexistent")
		assert.Error(t, err)
	})

	t.Run("get expired url", func(t *testing.T) {
		url := &models.ShortenedURL{
			ID:          uuid.New(),
			UserID:      userID,
			OriginalURL: "https://example.com/expired",
			ShortCode:   "expired",
			CreatedAt:   time.Now(),
			ExpiresAt:   ptr(time.Now().Add(-24 * time.Hour)), // Expired
			IsActive:    true,
		}

		err := repo.Create(ctx, url)
		require.NoError(t, err)

		_, err = repo.GetByShortCode(ctx, url.ShortCode)
		assert.Error(t, err)
	})
}

func TestRepository_GetByUserID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()
	userID, err := createTestUser(ctx, db)
	require.NoError(t, err)

	t.Run("get user urls", func(t *testing.T) {
		// Create multiple URLs
		for i := 0; i < 3; i++ {
			url := &models.ShortenedURL{
				ID:          uuid.New(),
				UserID:      userID,
				OriginalURL: fmt.Sprintf("https://example.com/%d", i),
				ShortCode:   fmt.Sprintf("test%d", i),
				CreatedAt:   time.Now(),
				IsActive:    true,
			}
			err := repo.Create(ctx, url)
			require.NoError(t, err)
		}

		urls, err := repo.GetByUserID(ctx, userID)
		assert.NoError(t, err)
		assert.Len(t, urls, 3)

		// Verify ordering (newest first)
		for i := 1; i < len(urls); i++ {
			assert.True(t, urls[i-1].CreatedAt.After(urls[i].CreatedAt) ||
				urls[i-1].CreatedAt.Equal(urls[i].CreatedAt))
		}
	})

	t.Run("get urls for user with no urls", func(t *testing.T) {
		emptyUserID := uuid.New()
		urls, err := repo.GetByUserID(ctx, emptyUserID)
		assert.NoError(t, err)
		assert.Empty(t, urls)
	})
}

func TestRepository_IncrementAccessCount(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()
	userID, err := createTestUser(ctx, db)
	require.NoError(t, err)

	t.Run("increment access count", func(t *testing.T) {
		url := &models.ShortenedURL{
			ID:          uuid.New(),
			UserID:      userID,
			OriginalURL: "https://example.com",
			ShortCode:   "count123",
			CreatedAt:   time.Now(),
			IsActive:    true,
		}

		err := repo.Create(ctx, url)
		require.NoError(t, err)

		// Increment multiple times
		for i := 0; i < 3; i++ {
			err = repo.IncrementAccessCount(ctx, url.ID)
			assert.NoError(t, err)
		}

		// Verify count
		updated, err := repo.GetByShortCode(ctx, url.ShortCode)
		assert.NoError(t, err)
		assert.Equal(t, 3, updated.AccessCount)
		assert.NotNil(t, updated.LastAccessedAt)
	})
}

func TestRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()
	userID, err := createTestUser(ctx, db)
	require.NoError(t, err)

	t.Run("delete existing url", func(t *testing.T) {
		url := &models.ShortenedURL{
			ID:          uuid.New(),
			UserID:      userID,
			OriginalURL: "https://example.com",
			ShortCode:   "delete123",
			CreatedAt:   time.Now(),
			IsActive:    true,
		}

		err := repo.Create(ctx, url)
		require.NoError(t, err)

		err = repo.Delete(ctx, url.ID)
		assert.NoError(t, err)

		// Try to get the URL - should fail
		_, err = repo.GetByShortCode(ctx, url.ShortCode)
		assert.Error(t, err)
	})

	t.Run("delete non-existent url", func(t *testing.T) {
		err := repo.Delete(ctx, uuid.New())
		assert.Error(t, err)
	})
}

func TestRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()
	userID, err := createTestUser(ctx, db)
	require.NoError(t, err)

	t.Run("update url", func(t *testing.T) {
		url := &models.ShortenedURL{
			ID:          uuid.New(),
			UserID:      userID,
			OriginalURL: "https://example.com",
			ShortCode:   "update123",
			CreatedAt:   time.Now(),
			IsActive:    true,
		}

		err := repo.Create(ctx, url)
		require.NoError(t, err)

		// Update URL
		url.IsActive = false
		url.ExpiresAt = ptr(time.Now().Add(24 * time.Hour))

		err = repo.Update(ctx, url)
		assert.NoError(t, err)

		// Verify updates
		updated, err := repo.GetByShortCode(ctx, url.ShortCode)
		assert.Error(t, err) // Should fail because IsActive is false
		assert.Nil(t, updated)
	})
}

func TestRepository_AnalyticsFunctions(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()
	userID, err := createTestUser(ctx, db)
	require.NoError(t, err)

	// Create a test URL
	url := &models.ShortenedURL{
		ID:          uuid.New(),
		UserID:      userID,
		OriginalURL: "https://example.com",
		ShortCode:   "analytics123",
		CreatedAt:   time.Now(),
		IsActive:    true,
	}
	err = repo.Create(ctx, url)
	require.NoError(t, err)

	t.Run("record and retrieve analytics", func(t *testing.T) {
		// Record multiple clicks with different properties
		clicks := []*models.ClickAnalytics{
			{
				ID:          uuid.New(),
				URLID:       url.ID,
				ClickedAt:   time.Now().Add(-1 * time.Hour),
				Referrer:    "https://google.com",
				UserAgent:   "Mozilla/5.0",
				IPAddress:   "1.1.1.1",
				CountryCode: "US",
				City:        "New York",
				Region:      "NY",
			},
			{
				ID:          uuid.New(),
				URLID:       url.ID,
				ClickedAt:   time.Now(),
				Referrer:    "https://google.com", // Same referrer
				UserAgent:   "Mozilla/5.0",
				IPAddress:   "2.2.2.2", // Different IP
				CountryCode: "GB",
				City:        "London",
				Region:      "Greater London",
			},
			{
				ID:          uuid.New(),
				URLID:       url.ID,
				ClickedAt:   time.Now(),
				Referrer:    "https://facebook.com", // Different referrer
				UserAgent:   "Mozilla/5.0",
				IPAddress:   "3.3.3.3", // Different IP
				CountryCode: "DE",
				City:        "Berlin",
				Region:      "Berlin",
			},
		}

		// Record each click
		for _, click := range clicks {
			err := repo.RecordClick(ctx, click)
			assert.NoError(t, err)
		}

		// Get analytics
		analytics, err := repo.GetURLAnalytics(ctx, url.ID)
		assert.NoError(t, err)
		assert.NotNil(t, analytics)

		// Verwende int statt int64 für die Vergleiche
		expectedTotalClicks := 3
		expectedUniqueClicks := 3
		expectedGoogleCount := 2
		expectedFBCount := 1
		expectedCountryCount := 1

		// Verify basic analytics
		assert.Equal(t, expectedTotalClicks, analytics.TotalClicks)
		assert.Equal(t, expectedUniqueClicks, analytics.UniqueClicks)

		// Verify top referrers
		assert.Len(t, analytics.TopReferrers, 2)
		assert.Equal(t, "https://google.com", analytics.TopReferrers[0].Referrer)
		assert.Equal(t, expectedGoogleCount, analytics.TopReferrers[0].Count)
		assert.Equal(t, "https://facebook.com", analytics.TopReferrers[1].Referrer)
		assert.Equal(t, expectedFBCount, analytics.TopReferrers[1].Count)

		// Verify top countries
		assert.Len(t, analytics.TopCountries, 3)
		countryMap := make(map[string]int)
		for _, c := range analytics.TopCountries {
			countryMap[c.CountryCode] = c.Count
			assert.Equal(t, expectedCountryCount, c.Count)
		}
		assert.Contains(t, countryMap, "US")
		assert.Contains(t, countryMap, "GB")
		assert.Contains(t, countryMap, "DE")
	})

	t.Run("analytics for non-existent URL", func(t *testing.T) {
		analytics, err := repo.GetURLAnalytics(ctx, uuid.New())
		assert.Error(t, err)
		assert.Nil(t, analytics)
	})
}

// Helper function to create pointer to time
func ptr(t time.Time) *time.Time {
	return &t
}
