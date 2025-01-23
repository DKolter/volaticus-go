FROM golang:1.23-alpine AS build

RUN apk add --no-cache curl git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go install github.com/a-h/templ/cmd/templ@v0.3.819 && \
    templ generate && \
    curl -sL https://github.com/tailwindlabs/tailwindcss/releases/download/v3.4.16/tailwindcss-linux-x64 -o tailwindcss && \
    chmod +x tailwindcss && \
    ./tailwindcss -i cmd/web/assets/css/input.css -o cmd/web/assets/css/output.css

# Build the binary with version information
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=""

RUN if [ -z "$BUILD_DATE" ]; then BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ); fi && \
    if [ "$COMMIT" = "unknown" ]; then COMMIT=$(git rev-parse --short HEAD); fi && \
    CGO_ENABLED=0 go build \
    -ldflags "-s -w \
    -X main.version=${VERSION} \
    -X main.commit=${COMMIT} \
    -X main.date=${BUILD_DATE}" \
    -o volaticus cmd/api/main.go

FROM alpine:3.20.1 AS prod
WORKDIR /app
COPY --from=build /app/volaticus /app/volaticus
EXPOSE ${PORT}
CMD ["./volaticus"]
