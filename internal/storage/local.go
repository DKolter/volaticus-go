package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
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

	log.Debug().
		Str("path", fullPath).
		Str("filename", filename).
		Msg("uploading file to local storage")

	dst, err := os.Create(fullPath)
	if err != nil {
		log.Error().
			Err(err).
			Str("path", fullPath).
			Msg("failed to create file")
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		log.Error().
			Err(err).
			Str("path", fullPath).
			Msg("failed to write file")
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	log.Debug().
		Str("path", fullPath).
		Msg("file uploaded successfully")

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

	log.Debug().
		Str("path", fullPath).
		Msg("checking file existence")

	_, err := os.Stat(fullPath)
	if err == nil {
		log.Debug().
			Str("path", fullPath).
			Msg("file exists")
		return true, nil
	}
	if os.IsNotExist(err) {
		log.Debug().
			Str("path", fullPath).
			Msg("file does not exist")
		return false, nil
	}

	log.Error().
		Err(err).
		Str("path", fullPath).
		Msg("error checking file existence")
	return false, fmt.Errorf("error checking file existence: %w", err)
}

func (l *LocalStorageProvider) Delete(ctx context.Context, filename string) error {
	fullPath := filepath.Join(l.baseDir, filename)

	log.Debug().
		Str("path", fullPath).
		Msg("deleting file")

	if err := os.Remove(fullPath); err != nil {
		log.Error().
			Err(err).
			Str("path", fullPath).
			Msg("failed to delete file")
		return fmt.Errorf("failed to delete file: %w", err)
	}

	log.Debug().
		Str("path", fullPath).
		Msg("file deleted successfully")

	return nil
}

func (l *LocalStorageProvider) GetURL(ctx context.Context, filename string) (string, time.Duration, error) {
	return fmt.Sprintf("%s/f/%s", l.baseURL, filename), 0, nil
}

func (l *LocalStorageProvider) ListFiles(ctx context.Context, prefix string) ([]FileInfo, error) {
	var files []FileInfo
	basePath := filepath.Join(l.baseDir, prefix)

	log.Debug().
		Str("base_path", basePath).
		Str("prefix", prefix).
		Msg("listing files")

	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Error().
				Err(err).
				Str("path", path).
				Msg("error accessing path")
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(l.baseDir, path)
		if err != nil {
			log.Error().
				Err(err).
				Str("path", path).
				Str("base_dir", l.baseDir).
				Msg("failed to get relative path")
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		file, err := os.Open(path)
		if err != nil {
			log.Error().
				Err(err).
				Str("path", path).
				Msg("failed to open file")
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer file.Close()

		buffer := make([]byte, 512)
		_, err = file.Read(buffer)
		if err != nil && err != io.EOF {
			log.Error().
				Err(err).
				Str("path", path).
				Msg("failed to read file header")
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
		log.Error().
			Err(err).
			Str("base_path", basePath).
			Msg("error walking directory")
		return nil, fmt.Errorf("error walking directory: %w", err)
	}

	log.Debug().
		Str("base_path", basePath).
		Int("file_count", len(files)).
		Msg("completed listing files")

	return files, nil
}

func (l *LocalStorageProvider) Close() error {
	return nil
}
