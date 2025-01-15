package uploader

import (
	"database/sql/driver"
	"fmt"
)

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
