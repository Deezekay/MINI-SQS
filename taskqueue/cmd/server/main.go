package main

import (
	"log"
	"net/http"
	"time"

	"taskqueue/internal/api"
	"taskqueue/internal/clock"
	"taskqueue/internal/config"
	"taskqueue/internal/queue"
	"taskqueue/internal/wal"
)

func main() {
	cfg := config.Config{
		ListenAddr:             ":8080",
		VisibilityTimeout:      30 * time.Second,
		MaxPayloadBytes:        64 * 1024, // 64KB
		MaxTasksInMemory:       10000,
		WALFilePath:            "durable.wal",
		WALSyncOnWrite:         true,
		VisibilityScanInterval: 1 * time.Second,
	}

	clk := clock.RealClock{}
	mgr := queue.NewManager(cfg, clk)

	// Setup WAL for durability
	w, err := wal.NewWAL(cfg.WALFilePath, cfg.WALSyncOnWrite)
	if err != nil {
		log.Fatalf("Failed to initialize WAL: %v", err)
	}
	defer w.Close()

	mgr.SetWAL(w)

	log.Println("Replaying WAL...")
	if err := wal.Replay(cfg.WALFilePath, mgr); err != nil {
		log.Fatalf("Failed to replay WAL: %v", err)
	}
	log.Println("WAL Replay complete.")

	mgr.StartVisibilityScanner()

	mux := http.NewServeMux()
	mux.HandleFunc("/enqueue", api.EnqueueHandler(mgr))
	mux.HandleFunc("/poll", api.PollHandler(mgr))
	mux.HandleFunc("/ack", api.AckHandler(mgr))
	mux.HandleFunc("/metrics", api.MetricsHandler(mgr))

	log.Printf("Server starting on %s", cfg.ListenAddr)
	if err := http.ListenAndServe(cfg.ListenAddr, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
