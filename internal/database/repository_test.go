package database

import (
	"context"
	"fmt"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestStruct struct {
	ID   int    `db:"id"`
	Name string `db:"name"`
}

func setupRepositoryTestDB(t *testing.T) *DB {
	cfg := Config{
		Host:     host,     // Verwenden der existierenden Variablen aus database_test.go
		Port:     port,     // Verwenden der existierenden Variablen aus database_test.go
		Database: database, // Verwenden der existierenden Variablen aus database_test.go
		Username: username, // Verwenden der existierenden Variablen aus database_test.go
		Password: password, // Verwenden der existierenden Variablen aus database_test.go
		Schema:   "public",
	}
	db, err := New(cfg)
	require.NoError(t, err)
	require.NotNil(t, db)
	return db
}

func TestRepository_BasicOperations(t *testing.T) {
	db := setupRepositoryTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	// Create test table
	_, err := repo.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS test_table (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL
		)
	`)
	require.NoError(t, err)

	t.Run("test Exec and QueryRow", func(t *testing.T) {
		// Insert data using Exec
		result, err := repo.Exec(ctx, "INSERT INTO test_table (name) VALUES ($1) RETURNING id", "test1")
		assert.NoError(t, err)
		affected, err := result.RowsAffected()
		assert.NoError(t, err)
		assert.Equal(t, int64(1), affected)

		// Query the inserted row
		var name string
		err = repo.QueryRow(ctx, "SELECT name FROM test_table WHERE name = $1", "test1").Scan(&name)
		assert.NoError(t, err)
		assert.Equal(t, "test1", name)
	})

	t.Run("test Query", func(t *testing.T) {
		// Insert multiple rows
		_, err := repo.Exec(ctx, "INSERT INTO test_table (name) VALUES ($1), ($2)", "test2", "test3")
		assert.NoError(t, err)

		// Query multiple rows
		rows, err := repo.Query(ctx, "SELECT name FROM test_table ORDER BY id")
		assert.NoError(t, err)
		defer rows.Close()

		var names []string
		for rows.Next() {
			var name string
			err := rows.Scan(&name)
			assert.NoError(t, err)
			names = append(names, name)
		}
		assert.Equal(t, []string{"test1", "test2", "test3"}, names)
	})

	t.Run("test Get", func(t *testing.T) {
		var result TestStruct
		err := repo.Get(ctx, &result, "SELECT id, name FROM test_table WHERE name = $1", "test1")
		assert.NoError(t, err)
		assert.Equal(t, "test1", result.Name)
	})

	t.Run("test Select", func(t *testing.T) {
		var results []TestStruct
		err := repo.Select(ctx, &results, "SELECT id, name FROM test_table ORDER BY id")
		assert.NoError(t, err)
		assert.Len(t, results, 3)
		assert.Equal(t, "test1", results[0].Name)
		assert.Equal(t, "test2", results[1].Name)
		assert.Equal(t, "test3", results[2].Name)
	})

	t.Run("test WithTx success", func(t *testing.T) {
		err := repo.WithTx(ctx, func(tx *sqlx.Tx) error {
			_, err := tx.Exec("INSERT INTO test_table (name) VALUES ($1)", "test4")
			return err
		})
		assert.NoError(t, err)

		var name string
		err = repo.QueryRow(ctx, "SELECT name FROM test_table WHERE name = $1", "test4").Scan(&name)
		assert.NoError(t, err)
		assert.Equal(t, "test4", name)
	})

	t.Run("test WithTx rollback", func(t *testing.T) {
		err := repo.WithTx(ctx, func(tx *sqlx.Tx) error {
			_, err := tx.Exec("INSERT INTO test_table (name) VALUES ($1)", "test5")
			if err != nil {
				return err
			}
			return fmt.Errorf("forced error")
		})
		assert.Error(t, err)

		var name string
		err = repo.QueryRow(ctx, "SELECT name FROM test_table WHERE name = $1", "test5").Scan(&name)
		assert.Error(t, err) // Should not find the rolled back record
	})

	t.Run("test Error wrapper", func(t *testing.T) {
		baseErr := fmt.Errorf("base error")
		wrappedErr := repo.Error("test operation", baseErr)
		assert.Contains(t, wrappedErr.Error(), "repository test operation")
		assert.Contains(t, wrappedErr.Error(), baseErr.Error())
	})
}
