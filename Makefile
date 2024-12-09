# Simple Makefile for a Go project

# Build the application
all: dev-install build test
dev-install:
	@echo Installing dev dependecies
	@echo Installing air...
	go install github.com/air-verse/air@latest

	@echo Installing templ...
	go install github.com/a-h/templ/cmd/templ@latest

	@echo Installing golangci-lint
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.62.2

	@echo Make sure installed binaries are in PATH

build:
	@echo "Building..."
	@templ generate
	@go build -o ./bin/volaticus cmd/api/main.go

# Run the application
run:
	@go run cmd/api/main.go

# Create DB container
docker-run:
	@docker compose up --build

# Shutdown DB container
docker-down:
	@docker compose down

# Test the application
test:
	@echo "Testing..."
	@go test ./... -v
	
# Integrations Tests for the application
itest:
	@echo "Running integration tests..."
	@go test ./internal/database -v

# Clean the binary
clean:
	@echo "Cleaning..."
	@rm -f main

# Live Reload
watch:
	@echo "Watching..."
	@air

.PHONY: all build run test clean watch docker-run docker-down itest dev-install
