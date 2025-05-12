package main

import (
	"context"
	"log"
	"os"

	"github.com/clipper-camera/clipper-server/internal/api"
	"github.com/clipper-camera/clipper-server/internal/config"
)

func main() {
	logger := log.New(os.Stdout, "http: ", log.LstdFlags)

	cfg := &config.Config{}
	err := cfg.Load()
	if err != nil {
		log.Fatal(err)
	}

	server := api.NewServer(context.Background(), cfg, logger)
	logger.Printf("Starting server on %s", server.Addr)
	logger.Fatal(server.ListenAndServe())
}
