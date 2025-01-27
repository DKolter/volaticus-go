FROM golang:1.23-alpine AS build

RUN apk add --no-cache curl git npm

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go install github.com/a-h/templ/cmd/templ@v0.3.819 && \
    templ generate && \
    npm install -g tailwindcss@3.4.16 && \
    tailwindcss -i cmd/web/assets/css/input.css -o cmd/web/assets/css/output.css && \
    curl -L -o GeoLite2-City.mmdb https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-City.mmdb


# Build the binary with version information
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=""

ENV VERSION=${VERSION}
ENV COMMIT=${COMMIT}
ENV BUILD_DATE=${BUILD_DATE}

RUN if [ -z "$BUILD_DATE" ]; then BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ); fi && \
    if [ "$COMMIT" = "unknown" ]; then COMMIT=$(git rev-parse --short HEAD); fi && \
    CGO_ENABLED=0 go build \
    -ldflags "-s -w \
    -X main.version=${VERSION} \
    -X main.commit=${COMMIT} \
    -X main.date=${BUILD_DATE}" \
    -o volaticus cmd/api/main.go

# Development stage
FROM golang:1.23-alpine AS dev

RUN apk add --no-cache git curl make

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download && \
    git config --global --add safe.directory /app && \
    addgroup -S volaticus && \
    adduser -S volaticus -G volaticus

USER volaticus

# The rest of the source code will be mounted as a volume
CMD ["make","watch"]

# Production stage
FROM alpine:3.20.1 AS prod

# Create volaticus user and group
RUN addgroup -S volaticus && \
    adduser -S volaticus -G volaticus

WORKDIR /app

# Copy binary and data files
COPY --from=build /app/volaticus /app/volaticus
COPY --from=build /app/GeoLite2-City.mmdb /app/GeoLite2-City.mmdb

# Set permissions
RUN chmod 755 /app/volaticus && \
    chmod 644 /app/GeoLite2-City.mmdb && \
    chown -R volaticus:volaticus /app

# Switch to volaticus user
USER volaticus

EXPOSE ${PORT}
CMD ["./volaticus"]