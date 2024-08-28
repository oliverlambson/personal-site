package main

import (
	"fmt"
	"os"

	"github.com/oliverlambson/personal-site/internal/server"
)

func main() {
	hostIp := os.Getenv("HOST_IP")
	addr := fmt.Sprintf("%s:1960", hostIp)
	srv := server.NewServer(addr)

	fmt.Println("Starting server on", srv.Addr)
	if err := srv.ListenAndServe(); err != nil {
		fmt.Println("Error starting server:", err)
	}
}
