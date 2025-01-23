package auth

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

// createTestUser creates a test user in the database and returns its ID
func createTestUser(ctx context.Context, db *database.DB) (uuid.UUID, error) {
	userID := uuid.New()
	// Generiere eindeutige E-Mail und Benutzernamen mit UUID
	email := "test-" + uuid.New().String() + "@example.com"
	username := "testuser-" + uuid.New().String()

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

func TestRepository_CreateToken(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	// Create a test user
	userID, err := createTestUser(ctx, db)
	require.NoError(t, err)

	t.Run("successful token creation", func(t *testing.T) {
		token := &models.APIToken{
			ID:       uuid.New(),
			UserID:   userID, // Use the created user's ID
			Name:     "Test Token",
			Token:    "test-token-" + uuid.New().String(),
			IsActive: true,
		}

		err := repo.CreateToken(ctx, token)
		assert.NoError(t, err)
		assert.NotEmpty(t, token.ID)

		// Verify token exists
		exists, err := repo.TokenExists(ctx, token.Token)
		assert.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("duplicate token", func(t *testing.T) {
		token := &models.APIToken{
			ID:       uuid.New(),
			UserID:   userID, // Use the created user's ID
			Name:     "Test Token",
			Token:    "duplicate-token",
			IsActive: true,
		}

		// First creation should succeed
		err := repo.CreateToken(ctx, token)
		assert.NoError(t, err)

		// Second creation with same token should fail
		duplicateToken := &models.APIToken{
			ID:       uuid.New(),
			UserID:   userID,
			Name:     "Another Test Token",
			Token:    "duplicate-token",
			IsActive: true,
		}
		err = repo.CreateToken(ctx, duplicateToken)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrTokenExists)
	})
}

func TestRepository_GetAPITokenByToken(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	// Create a test user
	userID, err := createTestUser(ctx, db)
	require.NoError(t, err)

	t.Run("get existing token", func(t *testing.T) {
		// Create a token first
		originalToken := &models.APIToken{
			ID:       uuid.New(),
			UserID:   userID,
			Name:     "Test Token",
			Token:    "get-test-token-" + uuid.New().String(),
			IsActive: true,
		}
		err := repo.CreateToken(ctx, originalToken)
		require.NoError(t, err)

		// Retrieve the token
		foundToken, err := repo.GetAPITokenByToken(ctx, originalToken.Token)
		assert.NoError(t, err)
		assert.NotNil(t, foundToken)
		assert.Equal(t, originalToken.ID, foundToken.ID)
		assert.Equal(t, originalToken.UserID, foundToken.UserID)
		assert.Equal(t, originalToken.Name, foundToken.Name)
		assert.Equal(t, originalToken.Token, foundToken.Token)
	})

	t.Run("get non-existent token", func(t *testing.T) {
		token, err := repo.GetAPITokenByToken(ctx, "non-existent-token")
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrTokenNotFound)
		assert.Nil(t, token)
	})
}

func TestRepository_ListUserTokens(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	// Create a test user
	userID, err := createTestUser(ctx, db)
	require.NoError(t, err)

	t.Run("list user tokens", func(t *testing.T) {
		// Create multiple tokens for the user
		for i := 0; i < 3; i++ {
			token := &models.APIToken{
				ID:       uuid.New(),
				UserID:   userID,
				Name:     "Test Token " + uuid.New().String(),
				Token:    "list-test-token-" + uuid.New().String(),
				IsActive: true,
			}
			err := repo.CreateToken(ctx, token)
			require.NoError(t, err)
		}

		// List tokens
		tokens, err := repo.ListUserTokens(ctx, userID)
		assert.NoError(t, err)
		assert.Len(t, tokens, 3)

		// Verify tokens are ordered by created_at DESC
		for i := 1; i < len(tokens); i++ {
			assert.True(t, tokens[i-1].CreatedAt.After(tokens[i].CreatedAt) ||
				tokens[i-1].CreatedAt.Equal(tokens[i].CreatedAt))
		}
	})

	t.Run("list tokens for user with no tokens", func(t *testing.T) {
		emptyUserID := uuid.New()
		tokens, err := repo.ListUserTokens(ctx, emptyUserID)
		assert.NoError(t, err)
		assert.Empty(t, tokens)
	})
}

func TestRepository_RevokeToken(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	// Create a test user
	userID, err := createTestUser(ctx, db)
	require.NoError(t, err)

	t.Run("revoke existing token", func(t *testing.T) {
		// Create a token first
		token := &models.APIToken{
			ID:       uuid.New(),
			UserID:   userID,
			Name:     "Test Token",
			Token:    "revoke-test-token-" + uuid.New().String(),
			IsActive: true,
		}
		err := repo.CreateToken(ctx, token)
		require.NoError(t, err)

		// Revoke the token
		err = repo.RevokeToken(ctx, token.ID)
		assert.NoError(t, err)

		// Verify token is revoked
		revokedToken, err := repo.GetAPITokenByToken(ctx, token.Token)
		assert.NoError(t, err)
		assert.False(t, revokedToken.IsActive)
		assert.NotNil(t, revokedToken.RevokedAt)
	})

	t.Run("revoke non-existent token", func(t *testing.T) {
		err := repo.RevokeToken(ctx, uuid.New())
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrTokenNotFound)
	})
}

func TestRepository_UpdateLastUsed(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	// Create a test user
	userID, err := createTestUser(ctx, db)
	require.NoError(t, err)

	t.Run("update last used timestamp", func(t *testing.T) {
		// Create a token first
		token := &models.APIToken{
			ID:       uuid.New(),
			UserID:   userID,
			Name:     "Test Token",
			Token:    "update-test-token-" + uuid.New().String(),
			IsActive: true,
		}
		err := repo.CreateToken(ctx, token)
		require.NoError(t, err)

		// Update last used timestamp
		time.Sleep(time.Second) // Ensure time difference
		err = repo.UpdateLastUsed(ctx, token.ID)
		assert.NoError(t, err)

		// Verify last used timestamp is updated
		updatedToken, err := repo.GetAPITokenByToken(ctx, token.Token)
		assert.NoError(t, err)
		assert.NotNil(t, updatedToken.LastUsedAt)
	})

	t.Run("update non-existent token", func(t *testing.T) {
		err := repo.UpdateLastUsed(ctx, uuid.New())
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrTokenNotFound)
	})
}

func TestRepository_DeleteTokenByUserIdAndToken(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	// Create a test user
	userID, err := createTestUser(ctx, db)
	require.NoError(t, err)

	t.Run("delete existing token", func(t *testing.T) {
		token := &models.APIToken{
			ID:       uuid.New(),
			UserID:   userID,
			Name:     "Test Token",
			Token:    "delete-test-token-" + uuid.New().String(),
			IsActive: true,
		}
		err := repo.CreateToken(ctx, token)
		require.NoError(t, err)

		// Delete the token
		err = repo.DeleteTokenByUserIdAndToken(ctx, userID, token.Token)
		assert.NoError(t, err)

		// Verify token is deleted
		_, err = repo.GetAPITokenByToken(ctx, token.Token)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrTokenNotFound)
	})

	t.Run("delete non-existent token", func(t *testing.T) {
		err := repo.DeleteTokenByUserIdAndToken(ctx, uuid.New(), "non-existent-token")
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrTokenNotFound)
	})
}
