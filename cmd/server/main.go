package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strconv"

	"github.com/joho/godotenv"

	"github.com/Alberto-jhen/supabase-insert-helper/internal/api"
	"github.com/Alberto-jhen/supabase-insert-helper/internal/storage"
	"github.com/Alberto-jhen/supabase-insert-helper/internal/workers"
)

func main() {

	err1 := godotenv.Load()
	if err1 != nil {
		slog.Warn("No .env file found, reading environment variables from the system")
	}

	// Load environment configuration.
	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseServiceKey := os.Getenv("SUPABASE_SERVICE_ROLE_KEY")

	if supabaseURL == "" || supabaseServiceKey == "" {
		slog.Error("Missing required environment variables: SUPABASE_URL, SUPABASE_SERVICE_ROLE_KEY")
		os.Exit(1)
	}
	slog.Info("Credentials loaded successfully")

	// Initialize the Supabase singleton with the loaded credentials.
	client := storage.GetClient(supabaseURL, supabaseServiceKey)

	ctx := context.Background()

	// Start the worker pool and a background result collector.
	workerCount := 10
	if v := os.Getenv("WORKER_COUNT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			workerCount = n
		} else {
			slog.Warn("invalid WORKER_COUNT, using default", "value", v, "default", workerCount)
		}
	}
	slog.Info("worker pool configured", "workers", workerCount)

	pool := workers.NewPool(workerCount, workers.NewStorageProcessor(client))
	pool.Start(ctx)
	workers.StartResultCollector(ctx, pool.Results())

	mux := http.NewServeMux()

	mux.HandleFunc("POST /ingest/images", api.ImageIngestHandler(pool))

	mux.HandleFunc("POST /ingest/records", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Data ingest endpoint reached, now working")
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	addr := ":" + port
	fmt.Printf("Server listening on port %s...\n", port)

	err := http.ListenAndServe(addr, mux)

	if err != nil {
		log.Fatalf("The server has crashed: %v", err)
	}
}
