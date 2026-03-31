// internal/config/config.go
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	Port         string
	Env          string
	MongoURI     string
	MongoDB      string
	CORSOrigins  []string
	RateLimitRPM int
	PDFTmpDir    string
	LogLevel     string
}

// Load reads .env (if present) then environment variables. Returns error if required vars are missing.
func Load() (*Config, error) {
	// Best-effort load of .env — ignore if file not present
	_ = godotenv.Load()

	cfg := &Config{
		Port:        getEnv("PORT", "8080"),
		Env:         getEnv("ENV", "development"),
		MongoURI:    getEnv("MONGO_URI", ""),
		MongoDB:     getEnv("MONGO_DB", "ai_readiness"),
		PDFTmpDir:   getEnv("PDF_TMP_DIR", "/tmp/ai-readiness-pdfs"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
	}

	if cfg.MongoURI == "" {
		return nil, fmt.Errorf("MONGO_URI is required")
	}

	// CORS origins
	origins := getEnv("CORS_ORIGINS", "http://localhost:3000")
	cfg.CORSOrigins = splitTrim(origins, ",")

	// Rate limit
	rpmStr := getEnv("RATE_LIMIT_RPM", "60")
	rpm, err := strconv.Atoi(rpmStr)
	if err != nil || rpm <= 0 {
		return nil, fmt.Errorf("RATE_LIMIT_RPM must be a positive integer, got %q", rpmStr)
	}
	cfg.RateLimitRPM = rpm

	// Ensure PDF tmp dir exists
	if err := os.MkdirAll(cfg.PDFTmpDir, 0o755); err != nil {
		return nil, fmt.Errorf("cannot create PDF_TMP_DIR %q: %w", cfg.PDFTmpDir, err)
	}

	return cfg, nil
}

func (c *Config) IsProduction() bool { return c.Env == "production" }

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func splitTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
