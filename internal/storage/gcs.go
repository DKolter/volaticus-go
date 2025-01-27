package storage

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
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
		log.Debug().
			Str("emulator_host", emulatorHost).
			Msg("using GCS emulator")
		client, err = storage.NewClient(
			ctx,
			option.WithEndpoint(fmt.Sprintf("http://%s", emulatorHost)),
			option.WithoutAuthentication(),
		)
	} else {
		if creds := os.Getenv("GOOGLE_CLOUD_CREDENTIALS"); creds != "" {
			decodedCreds, decodeErr := base64.StdEncoding.DecodeString(creds)
			if decodeErr != nil {
				return nil, fmt.Errorf("invalid base64 credentials: %w", decodeErr)
			}
			client, err = storage.NewClient(ctx, option.WithCredentialsJSON(decodedCreds))
			if err != nil {
				return nil, fmt.Errorf("failed to create storage client with credentials: %w", err)
			}
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
		log.Info().
			Str("bucket", bucketName).
			Msg("bucket does not exist, creating...")
		if err := bucket.Create(ctx, projectID, &storage.BucketAttrs{
			Location: "US-CENTRAL1",
		}); err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}
		log.Info().
			Str("bucket", bucketName).
			Msg("successfully created bucket")
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
	log.Debug().
		Str("filename", filename).
		Msg("streaming file")

	obj := g.bucket.Object(filename)
	attrs, err := obj.Attrs(ctx)
	if err != nil {
		log.Error().
			Err(err).
			Str("filename", filename).
			Msg("failed to get object attributes")
		return fmt.Errorf("failed to get object attributes: %w", err)
	}

	log.Debug().
		Str("filename", filename).
		Str("content_type", attrs.ContentType).
		Int64("size", attrs.Size).
		Msg("retrieved object attributes")

	reader, err := obj.NewReader(ctx)
	if err != nil {
		log.Error().
			Err(err).
			Str("filename", filename).
			Msg("failed to create reader")
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
	if err != nil {
		log.Error().
			Err(err).
			Str("filename", filename).
			Int64("bytes_written", bytesWritten).
			Msg("failed to stream file")
		return fmt.Errorf("failed to stream file: %w", err)
	}

	log.Debug().
		Str("filename", filename).
		Int64("bytes_written", bytesWritten).
		Msg("file streamed successfully")

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
	log.Debug().
		Str("filename", filename).
		Msg("getting URL")

	// This currently only works with files that are stored as unixtime.ext

	// Ensure the file exists in the bucket
	obj := g.bucket.Object(filename)
	_, err := obj.Attrs(ctx)
	if err != nil {
		log.Error().
			Err(err).
			Str("filename", filename).
			Msg("failed to get object attributes")
		return "", 0, fmt.Errorf("failed to get object attributes: %w", err)
	}

	log.Debug().
		Str("filename", filename).
		Str("bucket", g.bucketName).
		Msg("object exists in bucket")

	baseURL := os.Getenv("BASE_URL")
	url := fmt.Sprintf("%s/f/%s", baseURL, filename)

	log.Debug().
		Str("filename", filename).
		Str("url", url).
		Msg("constructed URL")

	return url, 0, nil
}

func (g *GCSStorageProvider) ListFiles(ctx context.Context, prefix string) ([]FileInfo, error) {
	log.Debug().
		Str("prefix", prefix).
		Msg("listing files")

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
			log.Error().
				Err(err).
				Str("prefix", prefix).
				Msg("error iterating objects")
			return nil, fmt.Errorf("error iterating objects: %w", err)
		}
		files = append(files, FileInfo{
			Name:         attrs.Name,
			Size:         attrs.Size,
			ContentType:  attrs.ContentType,
			ModifiedTime: attrs.Updated,
		})
	}

	log.Debug().
		Str("prefix", prefix).
		Int("count", len(files)).
		Msg("files listed")

	return files, nil
}

func (g *GCSStorageProvider) Close() error {
	return g.client.Close()
}
