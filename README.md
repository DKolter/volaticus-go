# Volaticus

A modern, secure file sharing and URL shortening platform built with Go.


<div align = center>


---

**[<kbd> <br> 🚀 Install <br> </kbd>][Installation]** 
**[<kbd> <br> 📘 Documentation <br> </kbd>][Documentation]** 
**[<kbd> <br> 🕹 Features <br> </kbd>][Features]** 
**[<kbd> <br> 💙 Contribute <br> </kbd>][Contribution]**  

---
</div>

## 🌟 Features

### File Sharing

- 📤 Secure file uploads with customizable expiration
- 🔗 Multiple URL generation styles (UUID, GfyCat-style, Custom)
- 📊 File access tracking and analytics
- ⏰ Automatic cleanup of expired files
- 🔒 User-based file management

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

## 💻 Installation

### Docker (Recommended)

1. Clone the repository:

```bash
git clone https://github.com/yourusername/volaticus.git
cd volaticus
```

2. Create a `.env` file based on the example:

```bash
cp .env.example .env
```

3. Configure your environment variables in `.env`:

```env
PORT=8080
APP_ENV=production
DB_HOST=psql
DB_PORT=5432
DB_DATABASE=volaticus_db
DB_USERNAME=volaticus_service
DB_PASSWORD=very_secure_password
DB_SCHEMA=public
SECRET=your-secret-key
BASE_URL=http://localhost
UPLOAD_DIR=./uploads
MAX_UPLOAD_SIZE=150MB
UPLOAD_EXPIRES_IN=24
```

4. Start the application:

```bash
docker compose up -d
```

### Building from Source

Requirements:

- Go 1.23 or higher
- PostgreSQL

1. Clone and setup:

```bash
git clone https://github.com/yourusername/volaticus.git
cd volaticus
cp .env.example .env
```

2. Install dependencies:

```bash
make dev-install
```

3. Download Maxmind Geo-IP database, if you want to enable Geographic tracking

```
internal/database/GeoLite2-City.mmdb
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

## 🔧 NGINX Configuration

Example NGINX configuration for reverse proxy:

```nginx
server {
    listen 80;
    server_name your-domain.com;

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

## 🤝 Contributing

We welcome contributions! Here's how you can help:

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Commit changes: `git commit -m 'Add amazing feature'`
4. Push to branch: `git push origin feature/amazing-feature`
5. Open a Pull Request

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
[Documentation]: #-nginx-configuration
[Features]: #-features
[Contribution]: #-contributing
