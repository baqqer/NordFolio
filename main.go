package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"

	"nordnet-analyzer/internal"
)

//go:embed static
var staticFiles embed.FS

func main() {
	port := flag.String("port", "8080", "Port to run the server on (or env PORT)")
	dbPath := flag.String("db", "db.json", "Path to the local JSON database")
	flag.Parse()

	if env := os.Getenv("PORT"); env != "" && !isFlagSet("port") {
		*port = env
	}

	storage, err := internal.NewStorage(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	static, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	internal.NewServer(storage).Register(mux)
	mux.Handle("/", http.FileServerFS(static))

	fmt.Printf("NordFolio running on http://localhost:%s (database: %s)\n", *port, *dbPath)
	log.Fatal(http.ListenAndServe(":"+*port, mux))
}

func isFlagSet(name string) bool {
	set := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			set = true
		}
	})
	return set
}
