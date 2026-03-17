package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	GeminiAPIKey           string
	GitHubAppID            string
	GitHubWebhookSecret    string
	GitHubPrivateKeyPath   string
	GitHubPrivateKeyBase64 string
	Port                   string
}

func Load() *Config {
	_ = godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return &Config{
		GeminiAPIKey:           os.Getenv("GEMINI_API_KEY"),
		GitHubAppID:            os.Getenv("GITHUB_APP_ID"),
		GitHubWebhookSecret:    os.Getenv("GITHUB_WEBHOOK_SECRET"),
		GitHubPrivateKeyPath:   os.Getenv("GITHUB_PRIVATE_KEY_PATH"),
		GitHubPrivateKeyBase64: os.Getenv("GITHUB_PRIVATE_KEY_BASE64"),
		Port:                   port,
	}
}