package config

import (
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string
	JWTTTL      time.Duration
	// SwaggerEnabled mounts the API docs at /swagger/. Off by default so
	// production stays hidden unless explicitly turned on.
	SwaggerEnabled bool
	// SeedDemoUsers creates admin@medika.com, cs@medika.com and
	// perawat@medika.com on startup. Off by default: their password is public.
	SeedDemoUsers bool

	// n8n AI workflow (complaint analysis). Empty N8NWebhookURL leaves the
	// complaint API working but marks new complaints failed until it is set.
	N8NWebhookURL    string        // N8N_WEBHOOK_URL
	N8NWebhookSecret string        // N8N_WEBHOOK_SECRET, sent as X-Webhook-Secret both ways
	N8NTimeout       time.Duration // N8N_TIMEOUT, per attempt (default 30s)
	N8NMaxRetries    int           // N8N_MAX_RETRIES, attempts in total (default 3)
	// PublicBaseURL is how n8n reaches this backend, e.g. https://api.example.com;
	// the callback URL sent to n8n is PublicBaseURL + /api/n8n/complaints/callback.
	PublicBaseURL string

	// DeepSeek API for Stok Farmasi > Analitik (summary and chat). Empty
	// DeepSeekAPIKey leaves the analytics working without the assistant.
	DeepSeekAPIKey  string        // DEEPSEEK_API_KEY
	DeepSeekBaseURL string        // DEEPSEEK_BASE_URL (default https://api.deepseek.com)
	AIModel         string        // AI_MODEL (default deepseek-chat)
	AITimeout       time.Duration // AI_TIMEOUT, per question including tool calls (default 120s)
}

func Load() (Config, error) {
	cfg := Config{
		Port:        getenv("PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
	}
	if cfg.DatabaseURL == "" {
		return cfg, errors.New("DATABASE_URL is required")
	}
	if len(cfg.JWTSecret) < 32 {
		return cfg, errors.New("JWT_SECRET is required and must be at least 32 characters")
	}
	ttl, err := time.ParseDuration(getenv("JWT_TTL", "15m"))
	if err != nil || ttl <= 0 {
		return cfg, errors.New("JWT_TTL must be a positive duration, e.g. 15m")
	}
	cfg.JWTTTL = ttl
	swagger, err := strconv.ParseBool(getenv("SWAGGER_ENABLED", "false"))
	if err != nil {
		return cfg, errors.New("SWAGGER_ENABLED must be true or false")
	}
	cfg.SwaggerEnabled = swagger
	seed, err := strconv.ParseBool(getenv("SEED_DEMO_USERS", "false"))
	if err != nil {
		return cfg, errors.New("SEED_DEMO_USERS must be true or false")
	}
	cfg.SeedDemoUsers = seed

	cfg.N8NWebhookURL = strings.TrimSpace(os.Getenv("N8N_WEBHOOK_URL"))
	cfg.N8NWebhookSecret = os.Getenv("N8N_WEBHOOK_SECRET")
	cfg.PublicBaseURL = strings.TrimRight(strings.TrimSpace(os.Getenv("PUBLIC_BASE_URL")), "/")
	for key, v := range map[string]string{"N8N_WEBHOOK_URL": cfg.N8NWebhookURL, "PUBLIC_BASE_URL": cfg.PublicBaseURL} {
		if v == "" {
			continue
		}
		if u, err := url.Parse(v); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return cfg, errors.New(key + " must be an http(s) URL")
		}
	}
	if cfg.N8NWebhookSecret != "" && len(cfg.N8NWebhookSecret) < 16 {
		return cfg, errors.New("N8N_WEBHOOK_SECRET must be at least 16 characters")
	}
	if cfg.N8NWebhookURL != "" && cfg.N8NWebhookSecret == "" {
		return cfg, errors.New("N8N_WEBHOOK_SECRET is required when N8N_WEBHOOK_URL is set")
	}
	n8nTimeout, err := time.ParseDuration(getenv("N8N_TIMEOUT", "30s"))
	if err != nil || n8nTimeout <= 0 {
		return cfg, errors.New("N8N_TIMEOUT must be a positive duration, e.g. 30s")
	}
	cfg.N8NTimeout = n8nTimeout
	retries, err := strconv.Atoi(getenv("N8N_MAX_RETRIES", "3"))
	if err != nil || retries < 1 || retries > 10 {
		return cfg, errors.New("N8N_MAX_RETRIES must be a number from 1 to 10")
	}
	cfg.N8NMaxRetries = retries

	cfg.DeepSeekAPIKey = strings.TrimSpace(os.Getenv("DEEPSEEK_API_KEY"))
	cfg.DeepSeekBaseURL = strings.TrimRight(strings.TrimSpace(getenv("DEEPSEEK_BASE_URL", "https://api.deepseek.com")), "/")
	if u, err := url.Parse(cfg.DeepSeekBaseURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return cfg, errors.New("DEEPSEEK_BASE_URL must be an http(s) URL")
	}
	cfg.AIModel = strings.TrimSpace(getenv("AI_MODEL", "deepseek-chat"))
	aiTimeout, err := time.ParseDuration(getenv("AI_TIMEOUT", "120s"))
	if err != nil || aiTimeout <= 0 {
		return cfg, errors.New("AI_TIMEOUT must be a positive duration, e.g. 120s")
	}
	cfg.AITimeout = aiTimeout
	return cfg, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
