// Package config loads and validates the environment-based configuration for main.go.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/robdesbois/golang-stock-ticker/alphavantage"
)

// ErrMissingEnv is returned when a required environment variable is absent or empty.
var ErrMissingEnv = errors.New("config: missing environment variable")

// ErrInvalidEnv is returned when an environment variable is present but has an invalid value.
var ErrInvalidEnv = errors.New("config: invalid environment variable")

// Config holds the service's startup configuration.
type Config struct {
	Symbol string
	NDays  int
	APIKey string
}

// LookupEnv matches the signature of os.LookupEnv, allowing Load to be tested without
// mutating the real process environment.
type LookupEnv func(string) (string, bool)

// LoadEnv loads Config from the real process environment.
func LoadEnv() (Config, error) {
	return Load(os.LookupEnv)
}

// Load reads and validates SYMBOL, NDAYS and APIKEY using lookup.
func Load(lookup LookupEnv) (Config, error) {
	symbol, ok := lookup("SYMBOL")
	if !ok || symbol == "" {
		return Config{}, fmt.Errorf("%w: SYMBOL", ErrMissingEnv)
	}

	apiKey, ok := lookup("APIKEY")
	if !ok || apiKey == "" {
		return Config{}, fmt.Errorf("%w: APIKEY", ErrMissingEnv)
	}

	ndaysStr, ok := lookup("NDAYS")
	if !ok || ndaysStr == "" {
		return Config{}, fmt.Errorf("%w: NDAYS", ErrMissingEnv)
	}
	ndays, err := strconv.Atoi(ndaysStr)
	if err != nil {
		return Config{}, fmt.Errorf("%w: NDAYS must be an integer, got %q", ErrInvalidEnv, ndaysStr)
	}
	if ndays < 1 || ndays > alphavantage.MaxDays {
		return Config{}, fmt.Errorf("%w: NDAYS must be >= 1 and <= %d, got %d", ErrInvalidEnv, alphavantage.MaxDays, ndays)
	}

	return Config{Symbol: symbol, NDays: ndays, APIKey: apiKey}, nil
}
