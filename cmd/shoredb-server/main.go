package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/shoreweaver/shoredb/pkg/server"
)

func main() {
	portFlag := flag.String("port", "", "address to listen on, e.g. :6379 (env SHOREDB_PORT)")
	configFlag := flag.String("config", "", "path to redis.config (env SHOREDB_CONFIG)")
	flag.Parse()

	port := firstNonEmpty(*portFlag, os.Getenv("SHOREDB_PORT"), ":6379")
	configPath := firstNonEmpty(*configFlag, os.Getenv("SHOREDB_CONFIG"), "redis.config")

	srv := server.New(server.Options{ListenAddr: port, ConfigPath: configPath})

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("Starting server on %s", port)
		if err := srv.Start(); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	<-signalChan
	log.Println("Shutdown signal received, saving data...")

	if err := srv.Shutdown(); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	log.Println("Server stopped")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
