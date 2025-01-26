package logger

import (
	"fmt"
	"github.com/mattn/go-isatty"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"os"
	"regexp"
	"strings"
)

type ColorScheme struct {
	Reset  string
	Red    string
	Green  string
	Yellow string
	Blue   string
	Purple string
	Cyan   string
	Gray   string
	Bold   string
}

var (
	// ANSI color codes
	colors = ColorScheme{
		Reset:  "\033[0m",
		Red:    "\033[31m",
		Green:  "\033[32m",
		Yellow: "\033[33m",
		Blue:   "\033[34m",
		Purple: "\033[35m",
		Cyan:   "\033[36m",
		Gray:   "\033[37m",
		Bold:   "\033[1m",
	}

	// No colors
	noColors = ColorScheme{
		Reset:  "",
		Red:    "",
		Green:  "",
		Yellow: "",
		Blue:   "",
		Purple: "",
		Cyan:   "",
		Gray:   "",
		Bold:   "",
	}

	// Regex für HTTP Status Codes (exakt 3 Ziffern)
	statusCodeRegex = regexp.MustCompile(`^[2-5]\d{2}$`)
)

func Init(env string) {
	// Detect if we're running in a terminal
	scheme := noColors
	if isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd()) {
		scheme = colors
	}

	output := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: "02.01.2006 15:04:05",
		NoColor:    scheme == noColors,
		FormatLevel: func(i interface{}) string {
			level := strings.ToUpper(fmt.Sprintf("%s", i))
			switch level {
			case "INFO":
				return fmt.Sprintf("%s●%s", scheme.Blue, scheme.Reset)
			case "WARN":
				return fmt.Sprintf("%s●%s", scheme.Yellow, scheme.Reset)
			case "ERROR":
				return fmt.Sprintf("%s●%s", scheme.Red, scheme.Reset)
			default:
				return level
			}
		},
		FormatMessage: func(i interface{}) string {
			msg := fmt.Sprintf("%-35s", i)

			if strings.Contains(msg, "Request completed") {
				return fmt.Sprintf("%s%s%s", scheme.Gray, msg, scheme.Reset)
			}
			if strings.Contains(msg, "Request started") {
				return fmt.Sprintf("%s%s%s", scheme.Bold, msg, scheme.Reset)
			}

			return msg
		},
		FormatFieldName: func(i interface{}) string {
			return fmt.Sprintf("%s%s%s=", scheme.Cyan, i, scheme.Reset)
		},
		FormatFieldValue: func(i interface{}) string {
			val := fmt.Sprintf("%s", i)

			// HTTP Methods
			switch val {
			case "GET", "POST", "PUT", "DELETE", "PATCH":
				return fmt.Sprintf("%s%s%s", scheme.Purple, val, scheme.Reset)
			}

			// Status Codes - nur exakte 3-stellige Zahlen zwischen 200-599
			if statusCodeRegex.MatchString(val) {
				code := val[0] // erste Ziffer
				switch code {
				case '2':
					return fmt.Sprintf("%s%s%s", scheme.Green, val, scheme.Reset)
				case '3':
					return fmt.Sprintf("%s%s%s", scheme.Yellow, val, scheme.Reset)
				case '4', '5':
					return fmt.Sprintf("%s%s%s", scheme.Red, val, scheme.Reset)
				}
			}

			return val
		},
	}

	log.Logger = zerolog.New(output).
		With().
		Timestamp().
		Str("env", env).
		Logger()

	switch env {
	case "development":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "production":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
}
