package main

import (
	"log"

	"github.com/aryans1319/devdoctor/config"
	"github.com/aryans1319/devdoctor/server"
)

func startServer() {
	cfg := config.Load()

	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("could not initialize server: %v", err)
	}

	if err := srv.Start(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}