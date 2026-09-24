package config

import (
	"errors"
	"testing"
)

// lookupFrom returns a LookupEnv backed by a fixed map, for tests.
func lookupFrom(env map[string]string) LookupEnv {
	return func(key string) (string, bool) {
		v, ok := env[key]
		return v, ok
	}
}

func TestLoad(t *testing.T) {
	validEnv := map[string]string{
		"SYMBOL": "MSFT",
		"NDAYS":  "7",
		"APIKEY": "secret",
	}

	tests := []struct {
		name    string
		env     map[string]string
		want    Config
		wantErr error
	}{
		{
			name: "valid config",
			env:  validEnv,
			want: Config{Symbol: "MSFT", NDays: 7, APIKey: "secret"},
		},
		{
			name:    "missing SYMBOL",
			env:     map[string]string{"NDAYS": "7", "APIKEY": "secret"},
			wantErr: ErrMissingEnv,
		},
		{
			name:    "empty SYMBOL",
			env:     map[string]string{"SYMBOL": "", "NDAYS": "7", "APIKEY": "secret"},
			wantErr: ErrMissingEnv,
		},
		{
			name:    "missing APIKEY",
			env:     map[string]string{"SYMBOL": "MSFT", "NDAYS": "7"},
			wantErr: ErrMissingEnv,
		},
		{
			name:    "empty APIKEY",
			env:     map[string]string{"SYMBOL": "MSFT", "NDAYS": "7", "APIKEY": ""},
			wantErr: ErrMissingEnv,
		},
		{
			name:    "missing NDAYS",
			env:     map[string]string{"SYMBOL": "MSFT", "APIKEY": "secret"},
			wantErr: ErrMissingEnv,
		},
		{
			name:    "NDAYS non-numeric",
			env:     map[string]string{"SYMBOL": "MSFT", "NDAYS": "abc", "APIKEY": "secret"},
			wantErr: ErrInvalidEnv,
		},
		{
			name:    "NDAYS zero",
			env:     map[string]string{"SYMBOL": "MSFT", "NDAYS": "0", "APIKEY": "secret"},
			wantErr: ErrInvalidEnv,
		},
		{
			name:    "NDAYS negative",
			env:     map[string]string{"SYMBOL": "MSFT", "NDAYS": "-1", "APIKEY": "secret"},
			wantErr: ErrInvalidEnv,
		},
		{
			name:    "NDAYS too large",
			env:     map[string]string{"SYMBOL": "MSFT", "NDAYS": "101", "APIKEY": "secret"},
			wantErr: ErrInvalidEnv,
		},
		{
			name: "NDAYS max",
			env:  map[string]string{"SYMBOL": "MSFT", "NDAYS": "100", "APIKEY": "secret"},
			want: Config{Symbol: "MSFT", NDays: 100, APIKey: "secret"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Load(lookupFrom(tt.env))

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Load() error = %v, want error wrapping %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("Load() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("Load() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
