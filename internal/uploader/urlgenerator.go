package uploader

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// URLGenerator handles different URL generation strategies
type URLGenerator struct{}

// NewURLGenerator creates a new URL generator
func NewURLGenerator() *URLGenerator {
	return &URLGenerator{}
}

// GenerateURL generates a URL based on the specified type
func (g *URLGenerator) GenerateURL(urlType URLType, originalName string) (string, error) {
	switch urlType {
	case URLTypeOriginalName:
		return g.generateOriginalNameURL(originalName)
	case URLTypeDefault:
		return g.generateDefaultURL()
	case URLTypeRandom:
		return g.generateRandomURL()
	case URLTypeDate:
		return g.generateDateURL()
	case URLTypeUUID:
		return g.generateUUIDURL()
	case URLTypeGfycat:
		return g.generateGfycatURL()
	default:
		return "", fmt.Errorf("unsupported URL type: %v", urlType)
	}
}

// generateOriginalNameURL creates a URL using the original filename
func (g *URLGenerator) generateOriginalNameURL(originalName string) (string, error) {
	// Clean the filename and remove any potentially problematic characters
	base := filepath.Base(originalName)
	base = strings.ToLower(base)
	base = strings.ReplaceAll(base, " ", "-")

	// Add a random suffix to prevent collisions
	// TODO: I'm not sure if we need this!
	suffix := make([]byte, 4)
	if _, err := rand.Read(suffix); err != nil {
		return "", err
	}

	return fmt.Sprintf("%s-%x", base, suffix), nil
}

// generateDefaultURL creates a URL using a timestamp
func (g *URLGenerator) generateDefaultURL() (string, error) {
	return fmt.Sprintf("%d", time.Now().UnixNano()), nil
}

// generateRandomURL creates a random alphanumeric URL
func (g *URLGenerator) generateRandomURL() (string, error) {
	const (
		charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
		length  = 8
	)

	result := make([]byte, length)
	for i := range result {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		result[i] = charset[n.Int64()]
	}

	return string(result), nil
}

// generateDateURL creates a URL using the current date and a random suffix
func (g *URLGenerator) generateDateURL() (string, error) {
	date := time.Now().Format("2006-01-02")

	// Add random suffix
	suffix := make([]byte, 4)
	if _, err := rand.Read(suffix); err != nil {
		return "", err
	}

	return fmt.Sprintf("%s-%x", date, suffix), nil
}

// generateUUIDURL creates a URL using a UUID
func (g *URLGenerator) generateUUIDURL() (string, error) {
	return uuid.New().String(), nil
}

// generateGfycatURL creates a URL using random words
func (g *URLGenerator) generateGfycatURL() (string, error) {
	// Get random indices for each word list
	adjectiveIdx, err := rand.Int(rand.Reader, big.NewInt(int64(len(adjectives))))
	if err != nil {
		return "", err
	}

	animalIdx, err := rand.Int(rand.Reader, big.NewInt(int64(len(animals))))
	if err != nil {
		return "", err
	}

	colorIdx, err := rand.Int(rand.Reader, big.NewInt(int64(len(colors))))
	if err != nil {
		return "", err
	}

	// Combine the words
	return fmt.Sprintf("%s-%s-%s",
		adjectives[adjectiveIdx.Int64()],
		colors[colorIdx.Int64()],
		animals[animalIdx.Int64()],
	), nil
}
