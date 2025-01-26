# Simple Makefile for a Go project

# Build the application
all: build test
templ-install:
	@if ! command -v templ > /dev/null; then \
		echo "Go's 'templ' is not installed on your machine. Installing..."; \
		go install github.com/a-h/templ/cmd/templ@latest; \
		if [ ! -x "$$(command -v templ)" ]; then \
			echo "templ installation failed. Exiting..."; \
			exit 1; \
		fi; \
	fi

tailwind-install:
	@if [ ! -f tailwindcss ]; then \
        if [ "$$(uname)" = "Darwin" ]; then \
            curl -sL https://github.com/tailwindlabs/tailwindcss/releases/download/v3.4.16/tailwindcss-macos-arm64 -o tailwindcss; \
        else \
            curl -sL https://github.com/tailwindlabs/tailwindcss/releases/download/v3.4.16/tailwindcss-linux-x64 -o tailwindcss; \
        fi \
    fi
	@chmod +x tailwindcss

build: tailwind-install templ-install
	@echo "Building..."
	@templ generate
	@./tailwindcss -i cmd/web/assets/css/input.css -o cmd/web/assets/css/output.css
	@go build -ldflags "-X main.version=$$(git describe --tags --always) -X main.commit=$$(git rev-parse --short HEAD) -X main.date=$$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o ./bin/volaticus cmd/api/main.go

# Run the application
run:
	@go run cmd/api/main.go
# Create DB container
docker-run:
	@if docker compose up --build 2>/dev/null; then \
		: ; \
	else \
		echo "Falling back to Docker Compose V1"; \
		docker-compose up --build; \
	fi

# Shutdown DB container
docker-down:
	@if docker compose down 2>/dev/null; then \
		: ; \
	else \
		echo "Falling back to Docker Compose V1"; \
		docker-compose down; \
	fi

# Test the application
test:
	@echo "Testing..."
	@go test ./... -v

lint:
	@echo "Linting..."
	@golangci-lint run

# Clean the binary
clean:
	@echo "Cleaning..."
	@find cmd/web -name "*_templ.go" -type f -delete
	@rm -f ./bin/volaticus

# Live Reload
watch:
	@if command -v air > /dev/null; then \
            air; \
            echo "Watching...";\
        else \
            read -p "Go's 'air' is not installed on your machine. Do you want to install it? [Y/n] " choice; \
            if [ "$$choice" != "n" ] && [ "$$choice" != "N" ]; then \
                go install github.com/air-verse/air@latest; \
                air; \
                echo "Watching...";\
            else \
                echo "You chose not to install air. Exiting..."; \
                exit 1; \
            fi; \
        fi

# Development

dev: dev-up
	docker compose -f docker-compose.dev.yml logs -f

dev-up:
	mkdir -p ./tmp ./tmp/psql ./tmp/gcs
	docker compose -f docker-compose.dev.yml up --build

dev-down:
	docker compose -f docker-compose.dev.yml down

dev-clean:
	docker compose -f docker-compose.dev.yml down -v

dev-logs:
	docker compose -f docker-compose.dev.yml logs -f

.PHONY: all build run test clean watch tailwind-install docker-run docker-down lint templ-install dev dev-up dev-down dev-logs dev-clean