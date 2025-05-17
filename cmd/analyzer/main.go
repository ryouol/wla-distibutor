package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/ryouol/log-distributor/pkg/models"
)

// MockAnalyzer represents a mock log analyzer service
type MockAnalyzer struct {
	ID         string
	Port       int
	Weight     float64
	router     *mux.Router
	httpServer *http.Server
	logCount   int
}

// NewMockAnalyzer creates a new mock analyzer
func NewMockAnalyzer(id string, port int, weight float64) *MockAnalyzer {
	router := mux.NewRouter()

	analyzer := &MockAnalyzer{
		ID:     id,
		Port:   port,
		Weight: weight,
		router: router,
		httpServer: &http.Server{
			Addr:         fmt.Sprintf(":%d", port),
			Handler:      router,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
	}

	analyzer.setupRoutes()
	return analyzer
}

// setupRoutes configures the API routes
func (a *MockAnalyzer) setupRoutes() {
	a.router.HandleFunc("/analyze", a.handleAnalyze).Methods(http.MethodPost)
	a.router.HandleFunc("/health", a.handleHealth).Methods(http.MethodGet)
}

// Start starts the HTTP server
func (a *MockAnalyzer) Start() {
	go func() {
		log.Printf("Starting Mock Analyzer %s on port %d with weight %.2f\n", a.ID, a.Port, a.Weight)
		if err := a.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()
}

// Stop gracefully stops the HTTP server
func (a *MockAnalyzer) Stop(ctx context.Context) error {
	return a.httpServer.Shutdown(ctx)
}

// handleAnalyze handles analyzing log packets
func (a *MockAnalyzer) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	var packet models.LogPacket

	// Decode JSON request
	if err := json.NewDecoder(r.Body).Decode(&packet); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Process the logs (in this case, just count them)
	a.logCount += len(packet.LogMessages)

	log.Printf("[Analyzer %s] Received packet with %d logs (Total: %d)\n",
		a.ID, len(packet.LogMessages), a.logCount)

	// Return success
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "processed",
	})
}

// handleHealth handles health check requests
func (a *MockAnalyzer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "healthy",
		"id":       a.ID,
		"logCount": a.logCount,
	})
}

func main() {
	// Parse command-line flags
	var (
		id     = flag.String("id", "analyzer1", "Analyzer ID")
		port   = flag.Int("port", 8081, "HTTP server port")
		weight = flag.Float64("weight", 1.0, "Analyzer weight")
	)
	flag.Parse()

	// Create mock analyzer
	analyzer := NewMockAnalyzer(*id, *port, *weight)

	// Start the analyzer
	analyzer.Start()

	// Wait for termination signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")

	// Create a timeout context for graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Stop the HTTP server
	if err := analyzer.Stop(shutdownCtx); err != nil {
		log.Printf("Error during server shutdown: %v\n", err)
	}

	log.Println("Shutdown complete")
}
