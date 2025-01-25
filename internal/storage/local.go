package storage

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type LocalStorageProvider struct {
	baseDir string
	baseURL string
}

func NewLocalStorage(baseDir, baseURL string) (*LocalStorageProvider, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create upload directory: %w", err)
	}

	return &LocalStorageProvider{
		baseDir: baseDir,
		baseURL: baseURL,
	}, nil
}

func (l *LocalStorageProvider) Upload(ctx context.Context, file io.Reader, filename string) (string, error) {
	fullPath := filepath.Join(l.baseDir, filename)
	dst, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return filename, nil
}

func (l *LocalStorageProvider) Stream(ctx context.Context, filename string, w http.ResponseWriter) error {
	fullPath := filepath.Join(l.baseDir, filename)
	file, err := os.Open(fullPath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get file info for content type detection
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Detect content type
	buffer := make([]byte, 512)
	_, err = file.Read(buffer)
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to read file header: %w", err)
	}
	contentType := http.DetectContentType(buffer)

	// Reset file pointer after reading header
	if _, err := file.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to reset file pointer: %w", err)
	}

	// Set response headers
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.FormatInt(fileInfo.Size(), 10))
	w.Header().Set("Cache-Control", "public, max-age=86400") // 24 hours cache

	// Stream the file
	if _, err := io.Copy(w, file); err != nil {
		return fmt.Errorf("failed to stream file: %w", err)
	}

	return nil
}

func (l *LocalStorageProvider) Exists(ctx context.Context, filename string) (bool, error) {
	fullPath := filepath.Join(l.baseDir, filename)
	log.Printf("Checking file existence: %s", fullPath)
	// Check if file exists and is accessible
	_, err := os.Stat(fullPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("error checking file existence: %w", err)
}

func (l *LocalStorageProvider) Delete(ctx context.Context, filename string) error {
	fullPath := filepath.Join(l.baseDir, filename)
	if err := os.Remove(fullPath); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return nil
}

func (l *LocalStorageProvider) GetURL(ctx context.Context, filename string) (string, time.Duration, error) {
	return fmt.Sprintf("%s/f/%s", l.baseURL, filename), 0, nil
}

func (l *LocalStorageProvider) ListFiles(ctx context.Context, prefix string) ([]FileInfo, error) {
	var files []FileInfo
	basePath := filepath.Join(l.baseDir, prefix)

	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Get relative path from base directory
		relPath, err := filepath.Rel(l.baseDir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		// Read file header for content type
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer file.Close()

		// Read first 512 bytes for content type detection
		buffer := make([]byte, 512)
		_, err = file.Read(buffer)
		if err != nil && err != io.EOF {
			return fmt.Errorf("failed to read file header: %w", err)
		}
		contentType := http.DetectContentType(buffer)

		files = append(files, FileInfo{
			Name:         relPath,
			Size:         info.Size(),
			ContentType:  contentType,
			ModifiedTime: info.ModTime(),
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error walking directory: %w", err)
	}

	return files, nil
}

func (l *LocalStorageProvider) Close() error {
	return nil
}
