package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/oliverlambson/personal-site/internal/server"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		healthz()
		return
	}

	hostIp := os.Getenv("HOST_IP")
	addr := fmt.Sprintf("%s:1960", hostIp)
	srv := server.NewServer(addr)

	fmt.Println("Starting server on", srv.Addr)
	if err := srv.ListenAndServe(); err != nil {
		fmt.Println("Error starting server:", err)
	}
}

func healthz() {
	resp, err := http.Get("http://localhost:1960/healthz")
	if err != nil {
		log.Fatal("Error:", err)
	}
	defer resp.Body.Close()

	if !(resp.StatusCode >= 200 && resp.StatusCode < 300) {
		log.Fatalf("Health check failed with status code: %d", resp.StatusCode)
	}
}
