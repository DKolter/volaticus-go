package uploader

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestGenerateDefaultURL(t *testing.T) {
	g := NewURLGenerator()
	url, err := g.generateDefaultURL()
	assert.NoError(t, err)
	assert.NotEmpty(t, url)
}

func TestGenerateOriginalNameURL(t *testing.T) {
	g := NewURLGenerator()
	url, err := g.generateOriginalNameURL("Test File.jpg")
	assert.NoError(t, err)
	assert.Contains(t, url, "test-file")
	assert.Len(t, url, len("test-file.jpg-")+8)
}

func TestGenerateRandomURL(t *testing.T) {
	g := NewURLGenerator()
	url, err := g.generateRandomURL()
	assert.NoError(t, err)
	assert.Len(t, url, 8)
}

func TestGenerateDateURL(t *testing.T) {
	g := NewURLGenerator()
	url, err := g.generateDateURL()
	assert.NoError(t, err)
	assert.Contains(t, url, time.Now().Format("2006-01-02"))
	assert.Len(t, url, len("2006-01-02-")+8)
}

func TestGenerateUUIDURL(t *testing.T) {
	g := NewURLGenerator()
	url, err := g.generateUUIDURL()
	assert.NoError(t, err)
	_, err = uuid.Parse(url)
	assert.NoError(t, err)
}

func TestGenerateGfycatURL(t *testing.T) {
	g := NewURLGenerator()
	url, err := g.generateGfycatURL()
	assert.NoError(t, err)
	parts := strings.Split(url, "-")
	assert.Len(t, parts, 3)
	assert.Contains(t, adjectives, parts[0])
	assert.Contains(t, colors, parts[1])
	assert.Contains(t, animals, parts[2])
}

func TestGenerateUniqueURLs(t *testing.T) {
	g := NewURLGenerator()
	urls := make(map[string]struct{})
	const numURLs = 200000

	for i := 0; i < numURLs; i++ {
		url, err := g.GenerateURL(URLTypeRandom, "")
		assert.NoError(t, err)
		if _, exists := urls[url]; exists {
			t.Fatalf("duplicate URL found: %s", url)
		}
		urls[url] = struct{}{}
	}

	assert.Len(t, urls, numURLs)
}
