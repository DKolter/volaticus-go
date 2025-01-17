package uploader

import (
	"crypto/rand"
	"database/sql/driver"
	"fmt"
	"math/big"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	adjectives = []string{
		"adorable", "beautiful", "clever", "delightful", "elegant",
		"fierce", "gentle", "happy", "intelligent", "jolly",
		"kind", "lively", "magical", "noble", "peaceful",
		"quick", "radiant", "silly", "talented", "unique",
		"vibrant", "wise", "zealous", "brave", "calm",
		"agile", "bright", "charming", "daring", "energetic",
		"fearless", "graceful", "humble", "inventive", "joyful",
		"keen", "luminous", "mighty", "neat", "optimistic",
		"playful", "quirky", "resilient", "strong", "thoughtful",
		"uplifting", "versatile", "whimsical", "youthful", "zesty",
		"affectionate", "bold", "creative", "determined", "earnest",
		"festive", "glorious", "hilarious", "inspiring", "judicious",
		"knowledgeable", "loyal", "motivated", "nurturing", "outgoing",
		"perceptive", "questioning", "resolute", "spontaneous", "tenacious",
		"understanding", "valiant", "warm", "xenodochial", "zany",
		"adaptable", "brilliant", "confident", "dazzling", "enthusiastic",
		"friendly", "gentle", "honest", "imaginative", "jubilant",
		"kindhearted", "lovable", "meticulous", "natural", "observant",
		"persistent", "quickwitted", "reliable", "spirited", "thoughtprovoking",
		"unassuming", "vivacious", "wonderful", "xenial", "zestful",
	}

	animals = []string{
		"alpaca", "bear", "cat", "dolphin", "elephant",
		"fox", "giraffe", "horse", "iguana", "jaguar",
		"kangaroo", "lion", "monkey", "narwhal", "octopus",
		"penguin", "quokka", "rabbit", "seal", "tiger",
		"unicorn", "vulture", "whale", "xerus", "zebra",
		"armadillo", "buffalo", "cheetah", "dog", "eel",
		"flamingo", "goat", "hedgehog", "impala", "jellyfish",
		"koala", "lemur", "moose", "newt", "ostrich",
		"panda", "quail", "raccoon", "sheep", "toucan",
		"urchin", "vole", "walrus", "xray fish", "yak",
		"antelope", "beaver", "cougar", "deer", "emu",
		"ferret", "gecko", "hamster", "ibis", "jackal",
		"kudu", "llama", "marmot", "numbat", "oryx",
		"platypus", "quokka", "reindeer", "salamander", "tapir",
		"uakari", "vicuna", "wombat", "xenopus", "zebu",
		"alligator", "bat", "crow", "dingo", "echidna",
		"frog", "gull", "heron", "ibex", "jay",
		"kingfisher", "lynx", "manatee", "nighthawk", "osprey",
		"peacock", "quoll", "robin", "sparrow", "terrapin",
		"urchin", "viper", "wolf", "xenops", "yellowtail",
	}

	colors = []string{
		"amber", "blue", "crimson", "denim", "emerald",
		"fuchsia", "gold", "hazel", "indigo", "jade",
		"kotlin", "lavender", "maroon", "navy", "olive",
		"purple", "quartz", "ruby", "silver", "teal",
		"umber", "violet", "white", "xanthic", "yellow",
		"aqua", "beige", "charcoal", "dusty rose", "eggplant",
		"forest green", "grape", "honey", "ivory", "khaki",
		"lime", "magenta", "neon green", "ocean", "peach",
		"quince", "rose gold", "sapphire", "taupe", "umber",
		"vermillion", "wine", "xenon blue", "blurple", "zinc",
		"apricot", "bronze", "cobalt", "dandelion", "ebony",
		"firebrick", "glacier", "harvest gold", "iris", "jasmine",
		"kiwi", "lemon", "mint", "neon pink", "obsidian",
		"pale blue", "quartz pink", "raspberry", "sand", "terracotta",
		"ultramarine", "vanilla", "whisper white", "xanadu", "zircon",
		"ash", "blush", "cerulean", "daffodil", "eucalyptus",
		"flame", "granite", "heather", "ice", "jet",
		"kelly green", "lilac", "moss", "nylon", "opal",
		"pearl", "quasar yellow", "redwood", "slate", "topaz",
		"umber", "velvet", "wheat", "xenic", "zenith blue",
	}
)

// URLGenerator handles different URL generation strategies
type URLGenerator struct{}

// NewURLGenerator creates a new URL generator
func NewURLGenerator() *URLGenerator {
	return &URLGenerator{}
}

type URLType int

const (
	URLTypeOriginalName URLType = iota
	URLTypeDefault
	URLTypeRandom
	URLTypeDate
	URLTypeUUID
	URLTypeGfycat
)

// String converts the URLType to its database string representation
func (ut URLType) String() string {
	return [...]string{
		"original_name",
		"default",
		"random",
		"date",
		"uuid",
		"gfycat",
	}[ut]
}

func ParseURLType(t string) (URLType, error) {
	switch t {
	case "original_name":
		return URLTypeOriginalName, nil
	case "default":
		return URLTypeDefault, nil
	case "random":
		return URLTypeRandom, nil
	case "date":
		return URLTypeDate, nil
	case "uuid":
		return URLTypeUUID, nil
	case "gfycat":
		return URLTypeGfycat, nil
	default:
		return URLTypeDefault, fmt.Errorf("invalid URL type: %s", t)
	}
}

// Value implements the driver.Valuer interface for database/sql
func (ut URLType) Value() (driver.Value, error) {
	return ut.String(), nil
}

// Scan implements the sql.Scanner interface for database/sql
func (ut *URLType) Scan(value interface{}) error {
	if value == nil {
		return fmt.Errorf("URLType cannot be nil")
	}

	str, ok := value.(string)
	if !ok {
		bytes, ok := value.([]byte)
		if !ok {
			return fmt.Errorf("failed to scan URLType: %v not string or []byte", value)
		}
		str = string(bytes)
	}

	switch str {
	case "original_name":
		*ut = URLTypeOriginalName
	case "default":
		*ut = URLTypeDefault
	case "random":
		*ut = URLTypeRandom
	case "date":
		*ut = URLTypeDate
	case "uuid":
		*ut = URLTypeUUID
	case "gfycat":
		*ut = URLTypeGfycat
	default:
		return fmt.Errorf("invalid URLType: %s", str)
	}

	return nil
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
