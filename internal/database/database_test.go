package database

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	database string
	password string
	username string
	host     string
	port     string
)

func mustStartPostgresContainer() (func(context.Context) error, error) {
	var (
		dbName = "database"
		dbPwd  = "password"
		dbUser = "user"
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
		log.Error().
			Err(err).
			Msg("failed to start postgres container")
		return nil, err
	}

	database = dbName
	password = dbPwd
	username = dbUser

	dbHost, err := dbContainer.Host(context.Background())
	if err != nil {
		log.Error().
			Err(err).
			Msg("failed to get container host")
		return dbContainer.Terminate, err
	}

	dbPort, err := dbContainer.MappedPort(context.Background(), "5432/tcp")
	if err != nil {
		log.Error().
			Err(err).
			Msg("failed to get container port")
		return dbContainer.Terminate, err
	}

	host = dbHost
	port = dbPort.Port()

	log.Info().
		Str("host", dbHost).
		Str("port", dbPort.Port()).
		Msg("postgres container started successfully")

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

	if teardown != nil {
		if err := teardown(context.Background()); err != nil {
			log.Fatal().
				Err(err).
				Msg("could not teardown postgres container")
		}
	}
}

// TestNew bleibt unverändert, da es Testing-Framework Logging nutzt
func TestNew(t *testing.T) {
	cfg := Config{
		Host:     host,
		Port:     port,
		Database: database,
		Username: username,
		Password: password,
		Schema:   "public",
	}
	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	if srv == nil {
		t.Fatal("New() returned nil")
	}
}

// TestHealth bleibt unverändert, da es Testing-Framework Logging nutzt
func TestHealth(t *testing.T) {
	cfg := Config{
		Host:     host,
		Port:     port,
		Database: database,
		Username: username,
		Password: password,
		Schema:   "public",
	}
	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	stats := srv.Health(context.Background())

	if stats["status"] != "up" {
		t.Fatalf("expected status to be up, got %s", stats["status"])
	}

	if _, ok := stats["error"]; ok {
		t.Fatalf("expected error not to be present")
	}
}

// TestClose bleibt unverändert, da es Testing-Framework Logging nutzt
func TestClose(t *testing.T) {
	cfg := Config{
		Host:     host,
		Port:     port,
		Database: database,
		Username: username,
		Password: password,
		Schema:   "public",
	}
	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	if srv.Close() != nil {
		t.Fatalf("expected Close() to return nil")
	}
}
