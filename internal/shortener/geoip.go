package shortener

import (
	"log"
	"net"
	"sync"

	"github.com/oschwald/geoip2-golang"
)

type GeoIPService struct {
	reader *geoip2.Reader
	mu     sync.RWMutex
}

var (
	geoIPInstance *GeoIPService
	geoIPOnce     sync.Once
)

/* //TODO: Remove this comment block after testing

Locally Test GeoIPService:
curl -H "X-Forwarded-For: IP_ADRESSE" http://localhost:PORT/SHORTCODE

Use:
	8.8.8.8 = US (Google DNS)
	185.86.151.11 = GB
	2.2.2.2 = SE
	203.104.128.1 = JP
	103.1.1.1 = AU
*/

// GetGeoIPService returns a singleton instance of GeoIPService
func GetGeoIPService() *GeoIPService {
	geoIPOnce.Do(func() {
		dbPath := "internal/database/GeoLite2-City.mmdb"

		reader, err := geoip2.Open(dbPath)
		if err != nil {
			log.Printf("Warning: Could not load GeoIP database: %v", err)
			geoIPInstance = &GeoIPService{}
			return
		}

		log.Printf("Successfully loaded GeoIP database from: %s", dbPath)
		geoIPInstance = &GeoIPService{
			reader: reader,
		}
	})
	return geoIPInstance
}

// LocationInfo contains geographic information about an IP address
type LocationInfo struct {
	CountryCode string
	City        string
	Region      string
}

// GetLocation returns location information for an IP address
func (g *GeoIPService) GetLocation(ipAddr string) *LocationInfo {
	if g.reader == nil {
		return &LocationInfo{CountryCode: "XX"} // Unknown
	}

	g.mu.RLock()
	defer g.mu.RUnlock()

	ip := net.ParseIP(ipAddr)
	if ip == nil {
		return &LocationInfo{CountryCode: "XX"}
	}

	record, err := g.reader.City(ip)
	if err != nil {
		log.Printf("Error looking up IP %s: %v", ipAddr, err)
		return &LocationInfo{CountryCode: "XX"}
	}

	// Handle potential nil values in record
	countryCode := "XX"
	if record.Country.IsoCode != "" {
		countryCode = record.Country.IsoCode
	}

	city := ""
	if len(record.City.Names) > 0 {
		city = record.City.Names["en"]
	}

	region := ""
	if len(record.Subdivisions) > 0 && len(record.Subdivisions[0].Names) > 0 {
		region = record.Subdivisions[0].Names["en"]
	}

	return &LocationInfo{
		CountryCode: countryCode,
		City:        city,
		Region:      region,
	}
}

// Close releases the GeoIP database resources
func (g *GeoIPService) Close() {
	if g.reader != nil {
		g.reader.Close()
	}
}
