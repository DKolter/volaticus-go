package storage

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"google.golang.org/api/iterator"

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
		log.Printf("Using GCS emulator at %s", emulatorHost)
		client, err = storage.NewClient(
			ctx,
			option.WithEndpoint(fmt.Sprintf("http://%s", emulatorHost)),
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

	_, err = bucket.Attrs(ctx)
	if errors.Is(err, storage.ErrBucketNotExist) {
		log.Printf("Bucket %s does not exist, creating...", bucketName)
		if err := bucket.Create(ctx, projectID, &storage.BucketAttrs{
			Location: "US-CENTRAL1",
		}); err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}
		log.Printf("Successfully created bucket %s", bucketName)
	} else if err != nil {
		return nil, fmt.Errorf("failed to check bucket: %w", err)
	}

	return &GCSStorageProvider{
		client:     client,
		bucket:     bucket,
		bucketName: bucketName,
	}, nil
}

func (g *GCSStorageProvider) Upload(ctx context.Context, file io.Reader, filename string) (string, error) {
	obj := g.bucket.Object(filename)
	writer := obj.NewWriter(ctx)

	if _, err := io.Copy(writer, file); err != nil {
		err := writer.Close()
		if err != nil {
			return "", err
		}
		return "", fmt.Errorf("failed to copy file to GCS: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("failed to close writer: %w", err)
	}

	return filename, nil
}

func (g *GCSStorageProvider) Stream(ctx context.Context, filename string, w http.ResponseWriter) error {
	log.Printf("Stream called for filename: %s", filename)

	obj := g.bucket.Object(filename)
	attrs, err := obj.Attrs(ctx)
	if err != nil {
		log.Printf("Error getting object attributes: %v", err)
		return fmt.Errorf("failed to get object attributes: %w", err)
	}
	log.Printf("Object attributes retrieved: %+v", attrs)

	reader, err := obj.NewReader(ctx)
	if err != nil {
		log.Printf("Error creating reader: %v", err)
		return fmt.Errorf("failed to create reader: %w", err)
	}
	defer reader.Close()

	// Set response headers
	w.Header().Set("Content-Type", attrs.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(attrs.Size, 10))
	if attrs.CacheControl != "" {
		w.Header().Set("Cache-Control", attrs.CacheControl)
	}

	// Stream the file
	bytesWritten, err := io.Copy(w, reader)
	log.Printf("Streamed %d bytes for file %s", bytesWritten, filename)
	if err != nil {
		log.Printf("Error streaming file: %v", err)
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
	log.Printf("GetURL called for filename: %s", filename)
	log.Printf("Base URL from environment: %s", os.Getenv("BASE_URL"))

	// Ensure the file exists in the bucket
	obj := g.bucket.Object(filename)
	_, err := obj.Attrs(ctx)
	if err != nil {
		log.Printf("Error getting object attributes: %v", err)
		return "", 0, fmt.Errorf("failed to get object attributes: %w", err)
	}
	log.Printf("Object exists in bucket: %s", filename)

	baseURL := os.Getenv("BASE_URL")
	url := fmt.Sprintf("%s/f/%s", baseURL, filename)
	log.Printf("Constructed URL: %s", url)
	return url, 0, nil
}

func (g *GCSStorageProvider) ListFiles(ctx context.Context, prefix string) ([]FileInfo, error) {
	log.Printf("ListFiles called with prefix: %s'", prefix)

	var files []FileInfo
	it := g.bucket.Objects(ctx, &storage.Query{
		Prefix: prefix,
	})

	for {
		attrs, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			log.Printf("Error iterating objects: %v", err)
			return nil, fmt.Errorf("error iterating objects: %w", err)
		}
		files = append(files, FileInfo{
			Name:         attrs.Name,
			Size:         attrs.Size,
			ContentType:  attrs.ContentType,
			ModifiedTime: attrs.Updated,
		})
	}

	log.Printf("Found %d files", len(files))
	return files, nil
}

func (g *GCSStorageProvider) Close() error {
	return g.client.Close()
}
