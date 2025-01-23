package dashboard

import (
	"context"
	"log"
	"testing"
	"time"
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
		log.Fatalf("could not start postgres container: %v", err)
	}

	m.Run()

	if teardown != nil && teardown(context.Background()) != nil {
		log.Fatalf("could not teardown postgres container: %v", err)
	}
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

// createTestURLs creates test shortened URLs for a user
func createTestURLs(ctx context.Context, db *database.DB, userID uuid.UUID, count int) error {
	query := `
        INSERT INTO shortened_urls (
            id, user_id, original_url, short_code, 
            access_count, is_active, created_at
        ) 
        VALUES ($1, $2, $3, $4, $5, $6, $7)`

	for i := 0; i < count; i++ {
		_, err := db.ExecContext(ctx, query,
			uuid.New(),
			userID,
			"https://example.com/test-"+uuid.New().String(),
			"test-"+uuid.New().String()[:8],
			i*10, // Different access counts
			true,
			time.Now().Add(-time.Duration(i)*time.Hour),
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// createTestFiles creates test uploaded files for a user
func createTestFiles(ctx context.Context, db *database.DB, userID uuid.UUID, count int) error {
	query := `
        INSERT INTO uploaded_files (
            id, user_id, original_name, unique_filename,
            file_size, url_value, mime_type, access_count, 
            created_at
        ) 
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	for i := 0; i < count; i++ {
		_, err := db.ExecContext(ctx, query,
			uuid.New(),
			userID,
			"test-file-"+uuid.New().String()+".txt",
			"unique-"+uuid.New().String(),
			1024*(i+1), // Different file sizes
			"/files/"+uuid.New().String(),
			"text/plain",
			i*5, // Different access counts
			time.Now().Add(-time.Duration(i)*time.Hour),
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func TestRepository_GetDashboardStats(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	// Create test user
	userID, err := createTestUser(ctx, db)
	require.NoError(t, err)

	t.Run("empty stats", func(t *testing.T) {
		stats, err := repo.GetDashboardStats(ctx, userID)
		assert.NoError(t, err)
		assert.NotNil(t, stats)
		assert.Equal(t, int64(0), stats.TotalURLs)
		assert.Equal(t, int64(0), stats.TotalClicks)
		assert.Equal(t, int64(0), stats.TotalFiles)
		assert.Equal(t, int64(0), stats.TotalStorage)
	})

	t.Run("populated stats", func(t *testing.T) {
		// Create test data
		err = createTestURLs(ctx, db, userID, 3)
		require.NoError(t, err)
		err = createTestFiles(ctx, db, userID, 2)
		require.NoError(t, err)

		stats, err := repo.GetDashboardStats(ctx, userID)
		assert.NoError(t, err)
		assert.NotNil(t, stats)

		// Verify URL stats
		assert.Equal(t, int64(3), stats.TotalURLs)
		assert.Equal(t, int64(30), stats.TotalClicks) // Sum of access_counts (0 + 10 + 20)

		// Verify file stats
		assert.Equal(t, int64(2), stats.TotalFiles)
		assert.Equal(t, int64(3072), stats.TotalStorage) // Sum of file sizes (1024 + 2048)
	})
}

func TestRepository_GetRecentURLs(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	// Create test user
	userID, err := createTestUser(ctx, db)
	require.NoError(t, err)

	t.Run("no urls", func(t *testing.T) {
		urls, err := repo.GetRecentURLs(ctx, userID, 5)
		assert.NoError(t, err)
		assert.Empty(t, urls)
	})

	t.Run("recent urls with limit", func(t *testing.T) {
		// Create test URLs
		err = createTestURLs(ctx, db, userID, 5)
		require.NoError(t, err)

		urls, err := repo.GetRecentURLs(ctx, userID, 3)
		assert.NoError(t, err)
		assert.Len(t, urls, 3)

		// Verify ordering (most recent first)
		for i := 1; i < len(urls); i++ {
			prevTime, err := time.Parse("2006-01-02 15:04:05", urls[i-1].CreatedAt)
			assert.NoError(t, err)
			currTime, err := time.Parse("2006-01-02 15:04:05", urls[i].CreatedAt)
			assert.NoError(t, err)
			assert.True(t, prevTime.After(currTime))
		}
	})
}

func TestRepository_GetRecentFiles(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	// Create test user
	userID, err := createTestUser(ctx, db)
	require.NoError(t, err)

	t.Run("no files", func(t *testing.T) {
		files, err := repo.GetRecentFiles(ctx, userID, 5)
		assert.NoError(t, err)
		assert.Empty(t, files)
	})

	t.Run("recent files with limit", func(t *testing.T) {
		// Create test files
		err = createTestFiles(ctx, db, userID, 5)
		require.NoError(t, err)

		files, err := repo.GetRecentFiles(ctx, userID, 3)
		assert.NoError(t, err)
		assert.Len(t, files, 3)

		// Verify ordering (most recent first)
		for i := 1; i < len(files); i++ {
			prevTime, err := time.Parse("2006-01-02 15:04:05", files[i-1].CreatedAt)
			assert.NoError(t, err)
			currTime, err := time.Parse("2006-01-02 15:04:05", files[i].CreatedAt)
			assert.NoError(t, err)
			assert.True(t, prevTime.After(currTime))
		}

		// Verify file properties
		for _, file := range files {
			assert.NotEmpty(t, file.FileName)
			assert.NotZero(t, file.FileSize)
			assert.GreaterOrEqual(t, int64(file.AccessCount), int64(0))
		}
	})
}
