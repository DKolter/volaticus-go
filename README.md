# Volaticus

A modern, secure file sharing and URL shortening platform built with Go.

<div align = center>

---

**[<kbd> <br> 🕹 Features <br> </kbd>][Features]**
**[<kbd> <br> 🚀 Install <br> </kbd>][Installation]**
**[<kbd> <br> 📘 Documentation <br> </kbd>][Documentation]**
**[<kbd> <br> 💙 Contribute <br> </kbd>][Contribution]** 

---

</div>

## 🌟 Features

### File Sharing

- 📤 Secure file uploads with customizable expiration
- 🔗 Multiple URL generation styles (UUID, GfyCat-style, etc.)
- 📊 File access tracking and analytics
- ⏰ Automatic cleanup of expired files
- 🔒 User-based file management
- 🗄️ Store files locally or in GCS buckets

### URL Shortening

- 🔤 Custom vanity URLs
- 📈 Comprehensive click analytics
- 🌍 Geographic tracking
- 📱 QR code generation
- ⏱️ Configurable expiration dates

### Security & Management

- 🔐 JWT-based authentication
- 🔑 API token management
- 👥 User account system
- 📱 Mobile-responsive UI
- 🚀 HTMX-powered interactions
- 📊 Structured logging with environment-aware log levels

## 💻 Installation

### Quick Deploy with Docker Compose (Recommended)

1. Create a new directory and navigate to it:

```bash
mkdir volaticus && cd volaticus
```

2. Download the compose file:

```bash
curl -o compose.yml https://raw.githubusercontent.com/DKolter/volaticus-go/main/compose.example.yml
```

3. Create a `.env` file with a secure random secret:

```bash
cat > .env << 'EOL'
# Server configuration
PORT=8080
APP_ENV=production
BASE_URL=http://localhost:8080

# Database configuration
DB_HOST=db
DB_PORT=5432
DB_DATABASE=volaticus_db
DB_USERNAME=volaticus_service
DB_PASSWORD=change_this_password
DB_SCHEMA=public

# File upload configuration
UPLOAD_MAX_SIZE=150MB
UPLOAD_USER_MAX_SIZE=500MB
UPLOAD_EXPIRES_IN=24

# Storage configuration
STORAGE_PROVIDER=local
UPLOAD_DIR=/app/uploads # mounted

# GCS settings (if STORAGE_PROVIDER=gcs)
GCS_PROJECT_ID=
GCS_BUCKET_NAME=

# Optional: Base64 encoded service account credentials
# Only needed if not using Workload Identity or running outside GCP
# GOOGLE_CLOUD_CREDENTIALS=<base64-encoded-service-account-json>
EOL

# Generate and append a secure secret
echo "SECRET=$(openssl rand -base64 32)" >> .env
```

4. Start the services:

```bash
docker compose up -d
```

The application will be available at http://localhost:8080.

### Build Docker Image from Source

1. Clone the repository:

```bash
git clone https://github.com/DKolter/volaticus-go.git
cd volaticus
```

2. Create a `.env` file based on the example:

```bash
cp .env.example .env
```

3. Configure your environment variables in `.env`:

```env
# Server configuration
PORT=8080
APP_ENV=production
BASE_URL=http://localhost:8080

# Database configuration
DB_HOST=localhost
DB_PORT=5432
DB_DATABASE=volaticus_db
DB_USERNAME=volaticus_service
DB_PASSWORD=very_secure_password
DB_SCHEMA=public

# Application secrets
SECRET=your-secret-

# File upload configuration
UPLOAD_MAX_SIZE=150MB
UPLOAD_USER_MAX_SIZE=500MB
UPLOAD_EXPIRES_IN=24

# Storage configuration
STORAGE_PROVIDER=local  # or 'gcs'

# Local storage settings (if STORAGE_PROVIDER=local)
UPLOAD_DIR=./uploads

# GCS settings (if STORAGE_PROVIDER=gcs)
GCS_PROJECT_ID=your-project-id
GCS_BUCKET_NAME=your-bucket-name

# Optional: Base64 encoded service account credentials
# Only needed if not using Workload Identity or running outside GCP
# GOOGLE_CLOUD_CREDENTIALS=<base64-encoded-service-account-json>
```

4. Start the application:

```bash
docker compose up -d
```

### Building from Source

Requirements:

- Go 1.23 or higher

1. Clone and setup:

```bash
git clone https://github.com/DKolter/volaticus-go.git
cd volaticus
cp .env.example .env
```

2. Install dependencies:

```bash
make dev-install
```

3. Download Maxmind Geo-IP database, if you want to enable Geographic tracking

```bash
curl -L -o GeoLite2-City.mmdb https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-City.mmdb
```

4. Build the application:

```bash
make build
```

5. Run:

```bash
make run
```

Additional make commands:

- `make watch`: Run with live reload
- `make test`: Run tests
- `make clean`: Clean build files
- `make docker-down`: Stop Docker containers

## 🔧 NGINX Reverse Proxy Configuration

Example NGINX configuration for reverse proxy:

```nginx
server {
    listen 80;
    server_name your-domain.com;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl;
    server_name your-domain.com;

    ssl_certificate /path/to/your/certificate.crt;
    ssl_certificate_key /path/to/your/private.key;

    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers 'ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384:ECDHE-ECDSA-CHACHA20-POLY1305:ECDHE-RSA-CHACHA20-POLY1305:ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256';
    ssl_prefer_server_ciphers on;
    ssl_session_cache shared:SSL:10m;
    ssl_session_timeout 10m;
    ssl_stapling on;
    ssl_stapling_verify on;
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;

    location / {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;

        # File upload settings
        client_max_body_size 150M;
        proxy_read_timeout 600;
        proxy_connect_timeout 600;
        proxy_send_timeout 600;
    }
}
```

## 📋 Logging

Volaticus implements a sophisticated logging system using zerolog for structured, leveled logging that adapts to your environment.

### Features

- 🎨 Beautiful console output with visual hierarchy

  - Color-coded HTTP methods and status codes
  - Clear request/response correlation via request IDs
  - Human readable timestamps and file sizes

- 🔬 Environment-aware logging

  - Development: Detailed debug information for local development
  - Production: Clean, performance-optimized info logging
  - Automatic static asset filtering to reduce noise

- 🔒 Privacy-conscious logging
  - IP address anonymization
  - User agent summarization
  - No sensitive data logging

### Log Levels

The system supports multiple log levels in order of verbosity:

- **DEBUG**: Detailed information for development and troubleshooting
- **INFO**: General operational entries about system behavior
- **WARN**: Warning messages for potentially harmful situations
- **ERROR**: Error events that might still allow the application to continue running

Log levels are automatically set based on the environment but can be manually controlled via the `APP_ENV` variable.

### Configuration

Set the environment in `.env`:

```env
# Development: Detailed debug logs with all information
APP_ENV=local/development

# Production: Clean, filtered info logs
APP_ENV=production
```

## 🔌 API Usage

### File Upload API

You can upload files programmatically using the API endpoint. Here's how to use it with curl:

First generate an API token in the web interface under Settings

```bash
# Upload a file
curl -X POST http://localhost:8080/api/v1/upload \
  -H "Authorization: Bearer your_api_token" \
  -F "file=@/path/to/your/file.jpg"
```

Response will contain the file URL:

```json
{
  "success": true,
  "url": "http://localhost:8080/f/unique-file-url"
}
```

Customize the URL format (optional)

```bash
# Available types: default, original_name, random, date, uuid, gfycat
curl -X POST http://localhost:8080/api/v1/upload \
  -H "Authorization: Bearer your_api_token" \
  -H "Url-Type: gfycat" \
  -F "file=@/path/to/your/file.jpg"
```

## 🤝 Contributing

We welcome contributions! Here's how you can help:

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Commit changes: `git commit -m 'Add amazing feature'`
4. Push to branch: `git push origin feature/amazing-feature`
5. Open a Pull Request

### Development Environment

For development, we provide a specialized Docker Compose setup that includes:

- Live reload for Go code changes
- GCS (Google Cloud Storage) emulator for local development
- PostgreSQL with persistent storage
- Development-specific logging and configurations

1. Clone the repository:

```bash
git clone https://github.com/DKolter/volaticus-go.git
cd volaticus
```

2. Create a development `.env` file:

```bash
cat > .env << 'EOL'
# Server configuration
PORT=8080
APP_ENV=development
BASE_URL=http://localhost:8080

# Database configuration
DB_HOST=psql
DB_PORT=5432
DB_DATABASE=volaticus_db
DB_USERNAME=volaticus_service
DB_PASSWORD=dev_password
DB_SCHEMA=public

# File upload configuration
MAX_UPLOAD_SIZE=150MB
UPLOAD_USER_MAX_SIZE=150MB
UPLOAD_EXPIRES_IN=24h
UPLOAD_DIR=./uploads

# GCS Emulator settings
STORAGE_PROVIDER=gcs
GCS_PROJECT_ID=test-project
GCS_BUCKET_NAME=test-bucket
EOL

# Generate and append a secure secret
echo "SECRET=$(openssl rand -base64 32)" >> .env
```

3. Use the provided Make commands to manage the development environment:

```bash
# Start the development environment
make dev-up

# View logs in real-time
make dev-logs

# Stop the environment
make dev-down

# Clean up volumes and containers
make dev-clean
```

### Development Guidelines

- Follow Go best practices and idioms
- Write tests for new features
- Update documentation as needed
- Follow the existing code style
- Use meaningful commit messages

### Pull Request Process

1. Update the README.md with details of changes if needed
2. Update any relevant documentation
3. Add tests for new functionality
4. Ensure the test suite passes
5. Get approval from maintainers

[Installation]: #-installation
[Documentation]: #-nginx-reverse-proxy-configuration
[Features]: #-features
[Contribution]: #-contributing
