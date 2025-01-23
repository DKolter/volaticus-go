package user

import (
	"context"
	"log"
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
	testDatabase string
	testPassword string
	testUsername string
	testHost     string
	testPort     string
)

// mustStartPostgresContainer initializes a test PostgreSQL container
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

// setupTestDB creates a test database instance with migrations
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

	err = migrate.RunMigrations(db.DB)
	require.NoError(t, err)

	return db
}

// createTestUser creates a user with random email and username for testing
func createTestUser(t *testing.T, repo Repository) *models.User {
	user := &models.User{
		ID:           uuid.New(),
		Email:        "test-" + uuid.New().String() + "@example.com",
		Username:     "testuser-" + uuid.New().String(),
		PasswordHash: "hashed_password",
		IsActive:     true,
	}
	err := repo.Create(context.Background(), user)
	require.NoError(t, err)
	return user
}

func TestRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	t.Run("successful user creation", func(t *testing.T) {
		user := &models.User{
			ID:           uuid.New(),
			Email:        "test@example.com",
			Username:     "testuser",
			PasswordHash: "hashed_password",
			IsActive:     true,
		}

		err := repo.Create(ctx, user)
		assert.NoError(t, err)

		// Verify user exists
		fetched, err := repo.GetByID(ctx, user.ID)
		assert.NoError(t, err)
		assert.Equal(t, user.Email, fetched.Email)
		assert.Equal(t, user.Username, fetched.Username)
	})

	t.Run("duplicate email", func(t *testing.T) {
		email := "duplicate@example.com"

		// Create first user
		user1 := &models.User{
			ID:           uuid.New(),
			Email:        email,
			Username:     "user1",
			PasswordHash: "hashed_password",
			IsActive:     true,
		}
		err := repo.Create(ctx, user1)
		assert.NoError(t, err)

		// Try to create second user with same email
		user2 := &models.User{
			ID:           uuid.New(),
			Email:        email,
			Username:     "user2",
			PasswordHash: "hashed_password",
			IsActive:     true,
		}
		err = repo.Create(ctx, user2)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrEmailExists)
	})

	t.Run("duplicate username", func(t *testing.T) {
		username := "duplicateuser"

		// Create first user
		user1 := &models.User{
			ID:           uuid.New(),
			Email:        "user1@example.com",
			Username:     username,
			PasswordHash: "hashed_password",
			IsActive:     true,
		}
		err := repo.Create(ctx, user1)
		assert.NoError(t, err)

		// Try to create second user with same username
		user2 := &models.User{
			ID:           uuid.New(),
			Email:        "user2@example.com",
			Username:     username,
			PasswordHash: "hashed_password",
			IsActive:     true,
		}
		err = repo.Create(ctx, user2)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrUsernameExists)
	})
}

func TestRepository_GetByID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	t.Run("get existing user", func(t *testing.T) {
		user := createTestUser(t, repo)

		fetched, err := repo.GetByID(ctx, user.ID)
		assert.NoError(t, err)
		assert.Equal(t, user.ID, fetched.ID)
		assert.Equal(t, user.Email, fetched.Email)
		assert.Equal(t, user.Username, fetched.Username)
	})

	t.Run("get non-existent user", func(t *testing.T) {
		_, err := repo.GetByID(ctx, uuid.New())
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrUserNotFound)
	})
}

func TestRepository_GetByEmail(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	t.Run("get existing user", func(t *testing.T) {
		user := createTestUser(t, repo)

		fetched, err := repo.GetByEmail(ctx, user.Email)
		assert.NoError(t, err)
		assert.Equal(t, user.ID, fetched.ID)
		assert.Equal(t, user.Email, fetched.Email)
		assert.Equal(t, user.Username, fetched.Username)
	})

	t.Run("get non-existent user", func(t *testing.T) {
		_, err := repo.GetByEmail(ctx, "nonexistent@example.com")
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrUserNotFound)
	})
}

func TestRepository_GetByUsername(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	t.Run("get existing user", func(t *testing.T) {
		user := createTestUser(t, repo)

		fetched, err := repo.GetByUsername(ctx, user.Username)
		assert.NoError(t, err)
		assert.Equal(t, user.ID, fetched.ID)
		assert.Equal(t, user.Email, fetched.Email)
		assert.Equal(t, user.Username, fetched.Username)
	})

	t.Run("get non-existent user", func(t *testing.T) {
		_, err := repo.GetByUsername(ctx, "nonexistentuser")
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrUserNotFound)
	})
}

func TestRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	t.Run("successful update", func(t *testing.T) {
		user := createTestUser(t, repo)

		// Update user information
		user.Email = "updated@example.com"
		user.Username = "updateduser"
		err := repo.Update(ctx, user)
		assert.NoError(t, err)

		// Verify updates
		fetched, err := repo.GetByID(ctx, user.ID)
		assert.NoError(t, err)
		assert.Equal(t, "updated@example.com", fetched.Email)
		assert.Equal(t, "updateduser", fetched.Username)
	})

	t.Run("update with existing email", func(t *testing.T) {
		user1 := createTestUser(t, repo)
		user2 := createTestUser(t, repo)

		// Try to update user2's email to user1's email
		user2.Email = user1.Email
		err := repo.Update(ctx, user2)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrEmailExists)
	})

	t.Run("update with existing username", func(t *testing.T) {
		user1 := createTestUser(t, repo)
		user2 := createTestUser(t, repo)

		// Try to update user2's username to user1's username
		user2.Username = user1.Username
		err := repo.Update(ctx, user2)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrUsernameExists)
	})

	t.Run("update non-existent user", func(t *testing.T) {
		user := &models.User{
			ID:       uuid.New(),
			Email:    "nonexistent@example.com",
			Username: "nonexistentuser",
		}
		err := repo.Update(ctx, user)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrUserNotFound)
	})
}

func TestRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	t.Run("successful delete", func(t *testing.T) {
		user := createTestUser(t, repo)

		// Delete user
		err := repo.Delete(ctx, user.ID)
		assert.NoError(t, err)

		// Verify user is marked as inactive
		fetched, err := repo.GetByID(ctx, user.ID)
		assert.NoError(t, err)
		assert.False(t, fetched.IsActive)
	})

	t.Run("delete non-existent user", func(t *testing.T) {
		err := repo.Delete(ctx, uuid.New())
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrUserNotFound)
	})
}
