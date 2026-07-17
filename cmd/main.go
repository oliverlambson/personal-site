package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/oliverlambson/personal-site/internal/server"
	"github.com/oliverlambson/personal-site/web"
)

const (
	defaultAddr = "127.0.0.1:1960"
	addrEnv     = "PERSONAL_SITE_ADDR"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Printf("personal-site: %v", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) > 0 && args[0] == "healthcheck" {
		return healthz()
	}

	srv, err := server.NewServer(listenAddress(), web.Files)
	if err != nil {
		return fmt.Errorf("initialize server: %w", err)
	}

	log.Println("Starting server on", srv.Addr)
	if err := srv.ListenAndServe(); err != nil {
		return fmt.Errorf("serve HTTP: %w", err)
	}
	return errors.New("HTTP server stopped unexpectedly")
}

func listenAddress() string {
	if addr := os.Getenv(addrEnv); addr != "" {
		return addr
	}
	return defaultAddr
}

func healthz() error {
	resp, err := http.Get("http://127.0.0.1:1960/healthz")
	if err != nil {
		return fmt.Errorf("request health check: %w", err)
	}
	defer resp.Body.Close()

	if !(resp.StatusCode >= 200 && resp.StatusCode < 300) {
		return fmt.Errorf("health check failed with status code: %d", resp.StatusCode)
	}
	return nil
}
