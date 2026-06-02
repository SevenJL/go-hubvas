// Package main is the entry point for the WebSocket collaboration server.
//
// The WS server handles:
//   - WebSocket upgrade and connection management
//   - Room lifecycle (create, run, idle-timeout)
//   - CRDT sync relay between clients in the same room
//   - Presence and awareness broadcast
//   - Cross-node fan-out via NATS pub/sub
//
// In production, this runs as a separate deployment from the API server
// to allow independent scaling of long-lived connections.
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/hubvas/pkg/config"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting Hubvas WebSocket Server...")

	cfg := config.Config{
		Server: config.ServerConfig{
			WSHost: "0.0.0.0",
			WSPort: 8081,
		},
	}

	// TODO: Wire up dependencies.
	//
	// Infrastructure:
	//   snapshotRepo := minio.NewSnapshotRepo(...)
	//   presenceRepo := redis.NewPresenceRepo(...)
	//   pubsub       := nats.NewPubSub(...)
	//
	// Hub:
	//   hub := ws.NewHub(snapshotRepo)
	//
	// Gateway:
	//   gateway := ws.NewGateway(hub, jwtSvc, permissionSvc)
	//   http.Handle("/ws", gateway)
	//   http.ListenAndServe(addr, nil)

	addr := fmt.Sprintf("%s:%d", cfg.Server.WSHost, cfg.Server.WSPort)
	log.Printf("WS server would listen on %s (skeleton — dependencies not wired)", addr)

	// Wait for shutdown signal.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Printf("Received signal %v, draining connections and shutting down...", sig)
}
