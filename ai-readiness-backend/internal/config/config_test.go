// internal/config/config_test.go
package config

import (
	"os"
	"testing"
)

func TestLoad_MissingMongoURI(t *testing.T) {
	os.Unsetenv("MONGO_URI")
	_, err := Load()
	if err == nil {
		t.Error("expected error when MONGO_URI is missing")
	}
}

func TestLoad_Defaults(t *testing.T) {
	os.Setenv("MONGO_URI", "mongodb://localhost:27017")
	defer os.Unsetenv("MONGO_URI")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != "8080" {
		t.Errorf("expected default port 8080, got %q", cfg.Port)
	}
	if cfg.MongoDB != "ai_readiness" {
		t.Errorf("expected default db ai_readiness, got %q", cfg.MongoDB)
	}
	if cfg.RateLimitRPM != 60 {
		t.Errorf("expected default 60 RPM, got %d", cfg.RateLimitRPM)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected default log level info, got %q", cfg.LogLevel)
	}
}

func TestLoad_CustomValues(t *testing.T) {
	os.Setenv("MONGO_URI", "mongodb://mongo:27017")
	os.Setenv("MONGO_DB", "custom_db")
	os.Setenv("PORT", "9090")
	os.Setenv("ENV", "production")
	os.Setenv("RATE_LIMIT_RPM", "120")
	os.Setenv("CORS_ORIGINS", "https://app.example.com,https://admin.example.com")
	os.Setenv("LOG_LEVEL", "debug")
	defer func() {
		for _, k := range []string{"MONGO_URI","MONGO_DB","PORT","ENV","RATE_LIMIT_RPM","CORS_ORIGINS","LOG_LEVEL"} {
			os.Unsetenv(k)
		}
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MongoDB != "custom_db" {
		t.Errorf("expected custom_db, got %q", cfg.MongoDB)
	}
	if cfg.Port != "9090" {
		t.Errorf("expected port 9090, got %q", cfg.Port)
	}
	if !cfg.IsProduction() {
		t.Error("expected IsProduction()=true for ENV=production")
	}
	if cfg.RateLimitRPM != 120 {
		t.Errorf("expected 120 RPM, got %d", cfg.RateLimitRPM)
	}
	if len(cfg.CORSOrigins) != 2 {
		t.Errorf("expected 2 CORS origins, got %d", len(cfg.CORSOrigins))
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected debug log level, got %q", cfg.LogLevel)
	}
}

func TestLoad_InvalidRateLimitRPM(t *testing.T) {
	os.Setenv("MONGO_URI", "mongodb://localhost:27017")
	os.Setenv("RATE_LIMIT_RPM", "not-a-number")
	defer os.Unsetenv("MONGO_URI")
	defer os.Unsetenv("RATE_LIMIT_RPM")

	_, err := Load()
	if err == nil {
		t.Error("expected error for non-numeric RATE_LIMIT_RPM")
	}
}

func TestLoad_ZeroRateLimitRPM(t *testing.T) {
	os.Setenv("MONGO_URI", "mongodb://localhost:27017")
	os.Setenv("RATE_LIMIT_RPM", "0")
	defer os.Unsetenv("MONGO_URI")
	defer os.Unsetenv("RATE_LIMIT_RPM")

	_, err := Load()
	if err == nil {
		t.Error("expected error for RATE_LIMIT_RPM=0")
	}
}

func TestSplitTrim(t *testing.T) {
	cases := []struct {
		input    string
		sep      string
		expected []string
	}{
		{"a,b,c", ",", []string{"a", "b", "c"}},
		{" a , b , c ", ",", []string{"a", "b", "c"}},
		{"single", ",", []string{"single"}},
		{"", ",", []string{}},
		{"a,,b", ",", []string{"a", "b"}},
	}
	for _, tc := range cases {
		got := splitTrim(tc.input, tc.sep)
		if len(got) != len(tc.expected) {
			t.Errorf("splitTrim(%q) = %v, want %v", tc.input, got, tc.expected)
			continue
		}
		for i := range got {
			if got[i] != tc.expected[i] {
				t.Errorf("splitTrim(%q)[%d] = %q, want %q", tc.input, i, got[i], tc.expected[i])
			}
		}
	}
}
