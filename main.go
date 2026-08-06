package main

import (
	"embed"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"nordnet-analyzer/internal"
)

// We can optionally embed static files using Go 1.16+ embed package,
// but for a simple local development web server, reading them from disk or embedding them is excellent.
// Let's use direct disk serving for local files so the user can easily modify/see the frontend code,
// or check if static directory exists first.
// Let's build a standard http.FileServer.

func main() {
	port := flag.Int("port", 8080, "Port to run the server on")
	dbPath := flag.String("db", "db.json", "Path to local JSON database")
	disableFX := flag.Bool("disable-fx-api", false, "Disable the external live FX currency rates API (offline mode)")
	flag.Parse()

	// Check environment variable fallback for easy Docker configuration
	if os.Getenv("DISABLE_FX_API") == "true" || os.Getenv("DISABLE_FX_API") == "1" {
		*disableFX = true
	}

	// Initialize storage
	storage, err := internal.NewStorage(*dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	server := internal.NewServer(storage, *disableFX)

	// API Routing
	http.HandleFunc("/api/portfolio", server.GetPortfolio)
	http.HandleFunc("/api/upload", server.UploadCSV)
	http.HandleFunc("/api/metadata", server.SaveMetadata)
	http.HandleFunc("/api/transactions", server.GetTransactions)
	http.HandleFunc("/api/reset", server.ResetDatabase)
	http.HandleFunc("/api/live-rates", server.GetLiveRates)

	// Static files serving
	// Serve static files from "./static" folder
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/", fs)

	// Ensure the static directory exists
	if _, err := os.Stat("static"); os.IsNotExist(err) {
		err = os.Mkdir("static", 0755)
		if err != nil {
			log.Fatalf("Failed to create static directory: %v", err)
		}
	}

	fmt.Printf("==================================================\n")
	fmt.Printf("  Nordnet Transaction Analyzer & Portfolio Tracker  \n")
	fmt.Printf("==================================================\n")
	fmt.Printf("Server starting on: http://localhost:%d\n", *port)
	fmt.Printf("Local JSON database: %s\n", *dbPath)
	fmt.Printf("==================================================\n")

	err = http.ListenAndServe(fmt.Sprintf(":%d", *port), nil)
	if err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// In case we want to support embedding later:
//
//go:embed static/*
var staticEmbed embed.FS
