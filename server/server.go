package server

import (
	"fmt"
	"log"
	"net/http"

	"github.com/aryans1319/devdoctor/config"
	githubapp "github.com/aryans1319/devdoctor/github"
)

// Server holds all dependencies
type Server struct {
	cfg     *config.Config
	gitApp  *githubapp.GitHubApp
	handler *githubapp.WebhookHandler
}

// New creates and configures the server
func New(cfg *config.Config) (*Server, error) {
	// Load GitHub App
	gitApp, err := githubapp.NewGitHubApp(
		cfg.GitHubAppID,
		cfg.GitHubPrivateKeyPath,
	)
	if err != nil {
		return nil, fmt.Errorf("could not initialize GitHub App: %w", err)
	}

	handler := githubapp.NewWebhookHandler(cfg, gitApp)

	return &Server{
		cfg:     cfg,
		gitApp:  gitApp,
		handler: handler,
	}, nil
}

// Start registers routes and starts the HTTP server
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Webhook endpoint — GitHub posts events here
	mux.HandleFunc("/webhook", s.handler.Handle)

	// Health check — Render uses this to verify the service is up
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","service":"devdoctor"}`))
	})

	addr := ":" + s.cfg.Port
	log.Printf("🩺 DevDoctor server starting on %s", addr)

	return http.ListenAndServe(addr, mux)
}