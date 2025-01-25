package shortener

import (
	"net"
	"sync"

	"github.com/oschwald/geoip2-golang"
	"github.com/rs/zerolog/log"
)

type GeoIPService struct {
	reader *geoip2.Reader
	mu     sync.RWMutex
}

var (
	geoIPInstance *GeoIPService
	geoIPOnce     sync.Once
)

// GetGeoIPService returns a singleton instance of GeoIPService
func GetGeoIPService() *GeoIPService {
	geoIPOnce.Do(func() {
		dbPath := "./GeoLite2-City.mmdb"

		reader, err := geoip2.Open(dbPath)
		if err != nil {
			log.Warn().
				Err(err).
				Str("path", dbPath).
				Msg("Could not load GeoIP database")
			geoIPInstance = &GeoIPService{}
			return
		}

		log.Info().
			Str("path", dbPath).
			Msg("Successfully loaded GeoIP database")
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
		log.Warn().
			Str("ip", ipAddr).
			Msg("Invalid IP address format")
		return &LocationInfo{CountryCode: "XX"}
	}

	record, err := g.reader.City(ip)
	if err != nil {
		log.Error().
			Err(err).
			Str("ip", ipAddr).
			Msg("Error looking up IP location")
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
		if err := g.reader.Close(); err != nil {
			log.Error().
				Err(err).
				Msg("Failed to close GeoIP database")
		}
	}
}
