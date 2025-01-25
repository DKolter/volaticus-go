package storage

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"google.golang.org/api/iterator"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

type GCSStorageProvider struct {
	client     *storage.Client
	bucket     *storage.BucketHandle
	bucketName string
}

func NewGCSStorage(projectID, bucketName string) (*GCSStorageProvider, error) {
	ctx := context.Background()
	var client *storage.Client
	var err error

	if emulatorHost := os.Getenv("STORAGE_EMULATOR_HOST"); emulatorHost != "" {
		client, err = storage.NewClient(
			ctx,
			option.WithEndpoint("http://"+emulatorHost),
			option.WithoutAuthentication(),
		)
	} else {
		if creds := os.Getenv("GOOGLE_CLOUD_CREDENTIALS"); creds != "" {
			decodedCreds, err := base64.StdEncoding.DecodeString(creds)
			if err != nil {
				return nil, fmt.Errorf("invalid base64 credentials: %w", err)
			}
			client, err = storage.NewClient(ctx, option.WithCredentialsJSON(decodedCreds))
		} else {
			client, err = storage.NewClient(ctx)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create storage client: %w", err)
	}

	bucket := client.Bucket(bucketName)

	return &GCSStorageProvider{
		client:     client,
		bucket:     bucket,
		bucketName: bucketName,
	}, nil
}

func (g *GCSStorageProvider) Upload(ctx context.Context, file io.Reader, filename string) (string, error) {
	obj := g.bucket.Object(filename)
	writer := obj.NewWriter(ctx)

	writer.ObjectAttrs.CacheControl = "public, max-age=86400"

	if _, err := io.Copy(writer, file); err != nil {
		writer.Close()
		return "", fmt.Errorf("failed to copy file to GCS: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("failed to close writer: %w", err)
	}

	return filename, nil
}

func (g *GCSStorageProvider) Stream(ctx context.Context, filename string, w http.ResponseWriter) error {
	obj := g.bucket.Object(filename)

	// Get object attributes for content type and size
	attrs, err := obj.Attrs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get object attributes: %w", err)
	}

	reader, err := obj.NewReader(ctx)
	if err != nil {
		return fmt.Errorf("failed to create reader: %w", err)
	}
	defer reader.Close()

	// Set response headers
	w.Header().Set("Content-Type", attrs.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(attrs.Size, 10))
	w.Header().Set("Cache-Control", attrs.CacheControl)
	if attrs.ContentDisposition != "" {
		w.Header().Set("Content-Disposition", attrs.ContentDisposition)
	}

	// Stream the file
	if _, err := io.Copy(w, reader); err != nil {
		return fmt.Errorf("failed to stream file: %w", err)
	}

	return nil
}

func (g *GCSStorageProvider) Exists(ctx context.Context, filename string) (bool, error) {
	obj := g.bucket.Object(filename)

	// Get object attributes to check existence
	_, err := obj.Attrs(ctx)
	if err == nil {
		return true, nil
	}

	// Check if the error is "not found"
	if errors.Is(err, storage.ErrObjectNotExist) {
		return false, nil
	}

	return false, fmt.Errorf("error checking object existence: %w", err)
}

func (g *GCSStorageProvider) Delete(ctx context.Context, filename string) error {
	obj := g.bucket.Object(filename)
	if err := obj.Delete(ctx); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return nil
}

func (g *GCSStorageProvider) GetURL(ctx context.Context, filename string) (string, time.Duration, error) {
	baseURL := os.Getenv("BASE_URL")
	return fmt.Sprintf("%s/f/%s", baseURL, filename), 0, nil
}

func (g *GCSStorageProvider) ListFiles(ctx context.Context, prefix string) ([]FileInfo, error) {
	// Create an empty slice to return if there are no files
	var files []FileInfo

	// Create an iterator for objects in the bucket with the given prefix
	it := g.bucket.Objects(ctx, &storage.Query{
		Prefix: prefix,
	})

	for {
		attrs, err := it.Next()
		if errors.Is(err, iterator.Done) {
			// Return empty slice instead of error when no files exist
			return files, nil
		}
		if err != nil {
			// Check if the error is specifically about no objects
			if strings.Contains(err.Error(), "404") {
				// Return empty slice for empty bucket
				return files, nil
			}
			return nil, fmt.Errorf("error iterating objects: %w", err)
		}

		files = append(files, FileInfo{
			Name:         attrs.Name,
			Size:         attrs.Size,
			ContentType:  attrs.ContentType,
			ModifiedTime: attrs.Updated,
		})
	}
}

func (g *GCSStorageProvider) Close() error {
	return g.client.Close()
}
