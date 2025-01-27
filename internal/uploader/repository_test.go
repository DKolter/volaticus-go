package uploader

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"

	"testing"
	"time"
	"volaticus-go/internal/common/models"
	"volaticus-go/internal/config"
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
	testDatabase string
	testPassword string
	testUsername string
	testHost     string
	testPort     string
)

// mustStartPostgresContainer starts a PostgreSQL container for testing
func mustStartPostgresContainer() (func(context.Context) error, error) {
	var (
		dbName = "testdb"
		dbPwd  = "testpass"
		dbUser = "testuser"
	)

	dbContainer, err := postgres.Run(
		context.Background(),
		"postgres:latest",
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

	testDatabase = dbName
	testPassword = dbPwd
	testUsername = dbUser

	dbHost, err := dbContainer.Host(context.Background())
	if err != nil {
		return dbContainer.Terminate, err
	}

	dbPort, err := dbContainer.MappedPort(context.Background(), "5432/tcp")
	if err != nil {
		return dbContainer.Terminate, err
	}

	testHost = dbHost
	testPort = dbPort.Port()

	return dbContainer.Terminate, err
}

func TestMain(m *testing.M) {
	teardown, err := mustStartPostgresContainer()
	if err != nil {
		log.Fatal().
			Err(err).
			Msg("could not start postgres container")
	}

	m.Run()

	if teardown != nil && teardown(context.Background()) != nil {
		log.Fatal().
			Err(err).
			Msg("could not teardown postgres container")
	}
}

// setupTestDB creates a test database instance
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

	// Run migrations
	err = migrate.RunMigrations(db.DB)
	require.NoError(t, err)

	return db
}

// createTestUser creates a test user and returns its ID
func createTestUser(ctx context.Context, db *database.DB) (uuid.UUID, error) {
	userID := uuid.New()
	email := "test-" + uuid.New().String() + "@example.com"
	username := "testuser-" + uuid.New().String()

	query := `
        INSERT INTO users (id, email, username, password_hash) 
        VALUES ($1, $2, $3, $4)
    `
	_, err := db.ExecContext(ctx, query, userID, email, username, "hashedpassword")
	return userID, err
}

// createTestFile creates a test file for testing
func createTestFile(ctx context.Context, repo Repository, userID uuid.UUID) (*models.UploadedFile, error) {
	file := &models.UploadedFile{
		ID:             uuid.New(),
		UserID:         userID,
		OriginalName:   "test-" + uuid.New().String() + ".txt",
		UniqueFilename: "unique-" + uuid.New().String(),
		MimeType:       "text/plain",
		FileSize:       1024,
		URLValue:       "/files/" + uuid.New().String(),
		CreatedAt:      time.Now(),
		AccessCount:    0,
	}

	err := repo.CreateWithURL(ctx, file, file.URLValue)
	return file, err
}

func TestRepository_CreateWithURL(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	cfg := config.Config{UploadUserQuota: 1024 * 1024 * 10} // 10 MB

	repo := NewRepository(db, cfg)
	ctx := context.Background()

	// Create test user
	userID, err := createTestUser(ctx, db)
	require.NoError(t, err)

	t.Run("successful creation", func(t *testing.T) {
		file := &models.UploadedFile{
			ID:             uuid.New(),
			UserID:         userID,
			OriginalName:   "test.txt",
			UniqueFilename: "unique-" + uuid.New().String(),
			MimeType:       "text/plain",
			FileSize:       1024,
			URLValue:       "/files/" + uuid.New().String(),
			CreatedAt:      time.Now(),
		}

		err := repo.CreateWithURL(ctx, file, file.URLValue)
		assert.NoError(t, err)

		// Verify file exists
		stored, err := repo.GetByID(ctx, file.ID)
		assert.NoError(t, err)
		assert.Equal(t, file.OriginalName, stored.OriginalName)
		assert.Equal(t, file.UniqueFilename, stored.UniqueFilename)
	})

	t.Run("duplicate URL value", func(t *testing.T) {
		urlValue := "/files/" + uuid.New().String()

		// Create first file
		file1 := &models.UploadedFile{
			ID:             uuid.New(),
			UserID:         userID,
			OriginalName:   "test1.txt",
			UniqueFilename: "unique-" + uuid.New().String(),
			URLValue:       urlValue,
		}
		err := repo.CreateWithURL(ctx, file1, urlValue)
		require.NoError(t, err)

		// Try to create second file with same URL
		file2 := &models.UploadedFile{
			ID:             uuid.New(),
			UserID:         userID,
			OriginalName:   "test2.txt",
			UniqueFilename: "unique-" + uuid.New().String(),
			URLValue:       urlValue,
		}
		err = repo.CreateWithURL(ctx, file2, urlValue)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrDuplicateURLValue)
	})
}

func TestRepository_GetByUniqueFilename(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	cfg := config.Config{UploadUserQuota: 1024 * 1024 * 10} // 10 MB

	repo := NewRepository(db, cfg)
	ctx := context.Background()

	userID, err := createTestUser(ctx, db)
	require.NoError(t, err)

	t.Run("file exists", func(t *testing.T) {
		file, err := createTestFile(ctx, repo, userID)
		require.NoError(t, err)

		found, err := repo.GetByUniqueFilename(ctx, file.UniqueFilename)
		assert.NoError(t, err)
		assert.NotNil(t, found)
		assert.Equal(t, file.ID, found.ID)
		assert.Equal(t, file.OriginalName, found.OriginalName)
	})

	t.Run("file not found", func(t *testing.T) {
		found, err := repo.GetByUniqueFilename(ctx, "nonexistent")
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrNoRows)
		assert.Nil(t, found)
	})
}

func TestRepository_GetFileStats(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	cfg := config.Config{UploadUserQuota: 1024 * 1024 * 10} // 10 MB

	repo := NewRepository(db, cfg)
	ctx := context.Background()

	userID, err := createTestUser(ctx, db)
	require.NoError(t, err)

	t.Run("empty stats", func(t *testing.T) {
		stats, err := repo.GetFileStats(ctx, userID)
		assert.NoError(t, err)
		assert.Equal(t, 0, stats.TotalFiles)
		assert.Equal(t, int64(0), stats.TotalSize)
		assert.Equal(t, int64(0), stats.TotalViews)
	})

	t.Run("populated stats", func(t *testing.T) {
		// Create multiple test files
		for i := 0; i < 3; i++ {
			file := &models.UploadedFile{
				ID:             uuid.New(),
				UserID:         userID,
				OriginalName:   fmt.Sprintf("test%d.txt", i),
				UniqueFilename: "unique-" + uuid.New().String(),
				MimeType:       "text/plain",
				FileSize:       uint64(1024 * int64(i+1)),
				URLValue:       "/files/" + uuid.New().String(),
				CreatedAt:      time.Now(),
				AccessCount:    i,
			}
			err := repo.CreateWithURL(ctx, file, file.URLValue)
			require.NoError(t, err)
		}

		stats, err := repo.GetFileStats(ctx, userID)
		assert.NoError(t, err)
		assert.Equal(t, 3, stats.TotalFiles)
		assert.Equal(t, int64(6144), stats.TotalSize) // 1024 + 2048 + 3072
		assert.Equal(t, int64(3), stats.TotalViews)   // 0 + 1 + 2
		assert.Contains(t, stats.PopularTypes, "text/plain")
	})
}

func TestRepository_DeleteFile(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	cfg := config.Config{UploadUserQuota: 1024 * 1024 * 10} // 10 MB

	repo := NewRepository(db, cfg)
	ctx := context.Background()

	userID, err := createTestUser(ctx, db)
	require.NoError(t, err)

	t.Run("successful delete", func(t *testing.T) {
		file, err := createTestFile(ctx, repo, userID)
		require.NoError(t, err)

		err = repo.Delete(ctx, file.ID)
		assert.NoError(t, err)

		// Verify file is deleted
		_, err = repo.GetByID(ctx, file.ID)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrNoRows)
	})

	t.Run("delete non-existent file", func(t *testing.T) {
		err := repo.Delete(ctx, uuid.New())
		assert.NoError(t, err) // Postgres DELETE is idempotent
	})
}

func TestRepository_IncrementAccessCount(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	cfg := config.Config{UploadUserQuota: 1024 * 1024 * 10} // 10 MB

	repo := NewRepository(db, cfg)
	ctx := context.Background()

	userID, err := createTestUser(ctx, db)
	require.NoError(t, err)

	t.Run("successful increment", func(t *testing.T) {
		file, err := createTestFile(ctx, repo, userID)
		require.NoError(t, err)

		initialCount := file.AccessCount

		err = repo.IncrementAccessCount(ctx, file.ID)
		assert.NoError(t, err)

		// Verify access count increased
		updated, err := repo.GetByID(ctx, file.ID)
		assert.NoError(t, err)
		assert.Equal(t, initialCount+1, updated.AccessCount)
		assert.NotNil(t, updated.LastAccessedAt)
	})
}

func TestRepository_GetUserFiles(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	cfg := config.Config{UploadUserQuota: 1024 * 1024 * 10} // 10 MB

	repo := NewRepository(db, cfg)
	ctx := context.Background()

	userID, err := createTestUser(ctx, db)
	require.NoError(t, err)

	t.Run("pagination", func(t *testing.T) {
		// Create 5 test files
		for i := 0; i < 5; i++ {
			_, err := createTestFile(ctx, repo, userID)
			require.NoError(t, err)
		}

		// Test pagination
		files, err := repo.GetUserFiles(ctx, userID, 2, 0)
		assert.NoError(t, err)
		assert.Len(t, files, 2)

		files, err = repo.GetUserFiles(ctx, userID, 2, 2)
		assert.NoError(t, err)
		assert.Len(t, files, 2)

		files, err = repo.GetUserFiles(ctx, userID, 2, 4)
		assert.NoError(t, err)
		assert.Len(t, files, 1)
	})

	t.Run("ordering", func(t *testing.T) {
		files, err := repo.GetUserFiles(ctx, userID, 5, 0)
		assert.NoError(t, err)

		// Verify files are ordered by created_at DESC
		for i := 1; i < len(files); i++ {
			assert.True(t, files[i-1].CreatedAt.After(files[i].CreatedAt))
		}
	})
}
