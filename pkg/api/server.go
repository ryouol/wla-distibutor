package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/yourusername/log-distributor/pkg/analyzer"
	"github.com/yourusername/log-distributor/pkg/distributor"
	"github.com/yourusername/log-distributor/pkg/models"
)

// Server represents the HTTP API server
type Server struct {
	router       *mux.Router
	httpServer   *http.Server
	distributor  *distributor.LogDistributor
	analyzerPool *analyzer.AnalyzerPool
}

// NewServer creates a new API server
func NewServer(
	addr string,
	distributor *distributor.LogDistributor,
	analyzerPool *analyzer.AnalyzerPool,
) *Server {
	router := mux.NewRouter()

	server := &Server{
		router:       router,
		distributor:  distributor,
		analyzerPool: analyzerPool,
		httpServer: &http.Server{
			Addr:         addr,
			Handler:      router,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
	}

	server.setupRoutes()
	return server
}

// setupRoutes configures the API routes
func (s *Server) setupRoutes() {
	s.router.HandleFunc("/api/v1/logs", s.handleLogPacket).Methods(http.MethodPost)
	s.router.HandleFunc("/api/v1/analyzers", s.handleAddAnalyzer).Methods(http.MethodPost)
	s.router.HandleFunc("/api/v1/analyzers/{id}", s.handleDeleteAnalyzer).Methods(http.MethodDelete)
	s.router.HandleFunc("/api/v1/metrics", s.handleGetMetrics).Methods(http.MethodGet)
	s.router.HandleFunc("/health", s.handleHealthCheck).Methods(http.MethodGet)
}

// Start starts the HTTP server
func (s *Server) Start() {
	go func() {
		log.Printf("Starting HTTP server on %s\n", s.httpServer.Addr)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()
}

// Stop gracefully stops the HTTP server
func (s *Server) Stop(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// handleLogPacket handles incoming log packets
func (s *Server) handleLogPacket(w http.ResponseWriter, r *http.Request) {
	var packet models.LogPacket

	// Decode JSON request
	if err := json.NewDecoder(r.Body).Decode(&packet); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Set received timestamp
	packet.ReceivedAt = time.Now()

	// Enqueue packet for processing
	success := s.distributor.EnqueuePacket(&packet)
	if !success {
		http.Error(w, "Server is at capacity, try again later", http.StatusServiceUnavailable)
		return
	}

	// Return success
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "accepted",
		"message": "Log packet queued for processing",
	})
}

// handleAddAnalyzer handles adding a new analyzer
func (s *Server) handleAddAnalyzer(w http.ResponseWriter, r *http.Request) {
	var analyzer struct {
		ID     string  `json:"id"`
		URL    string  `json:"url"`
		Weight float64 `json:"weight"`
	}

	// Decode JSON request
	if err := json.NewDecoder(r.Body).Decode(&analyzer); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if analyzer.ID == "" || analyzer.URL == "" || analyzer.Weight <= 0 {
		http.Error(w, "Invalid analyzer configuration", http.StatusBadRequest)
		return
	}

	// Add analyzer to pool
	s.analyzerPool.AddAnalyzer(analyzer.ID, analyzer.URL, analyzer.Weight)

	// Return success
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "created",
		"message": "Analyzer added successfully",
	})
}

// handleDeleteAnalyzer handles removing an analyzer
func (s *Server) handleDeleteAnalyzer(w http.ResponseWriter, r *http.Request) {
	// Get analyzer ID from URL path
	vars := mux.Vars(r)
	id, ok := vars["id"]
	if !ok {
		http.Error(w, "Missing analyzer ID", http.StatusBadRequest)
		return
	}

	// Remove analyzer from pool
	s.analyzerPool.RemoveAnalyzer(id)

	// Return success
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "deleted",
		"message": "Analyzer removed successfully",
	})
}

// handleGetMetrics handles retrieving distribution metrics
func (s *Server) handleGetMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := s.distributor.GetMetrics()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

// handleHealthCheck handles health check requests
func (s *Server) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
	})
}
