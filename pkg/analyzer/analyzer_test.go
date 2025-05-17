package analyzer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/ryouol/log-distributor/pkg/models"
)

// TestAddAnalyzer tests adding an analyzer to the pool
func TestAddAnalyzer(t *testing.T) {
	pool := NewAnalyzerPool(time.Second * 10)

	// Add an analyzer
	pool.AddAnalyzer("test-analyzer", "http://example.com", 0.5)

	// Check if added correctly
	pool.mutex.RLock()
	defer pool.mutex.RUnlock()

	if len(pool.analyzers) != 1 {
		t.Errorf("Expected 1 analyzer, got %d", len(pool.analyzers))
	}

	analyzer := pool.analyzers[0]
	if analyzer.ID != "test-analyzer" {
		t.Errorf("Expected ID to be 'test-analyzer', got '%s'", analyzer.ID)
	}

	if analyzer.URL != "http://example.com" {
		t.Errorf("Expected URL to be 'http://example.com', got '%s'", analyzer.URL)
	}

	if analyzer.Weight != 0.5 {
		t.Errorf("Expected weight to be 0.5, got %f", analyzer.Weight)
	}

	if !analyzer.Active {
		t.Error("Expected analyzer to be active")
	}

	if pool.totalWeight != 0.5 {
		t.Errorf("Expected total weight to be 0.5, got %f", pool.totalWeight)
	}
}

// TestRemoveAnalyzer tests removing an analyzer from the pool
func TestRemoveAnalyzer(t *testing.T) {
	pool := NewAnalyzerPool(time.Second * 10)

	// Add analyzers
	pool.AddAnalyzer("analyzer1", "http://example.com/1", 0.5)
	pool.AddAnalyzer("analyzer2", "http://example.com/2", 0.3)

	// Remove one analyzer
	pool.RemoveAnalyzer("analyzer1")

	// Check if removed correctly
	pool.mutex.RLock()
	defer pool.mutex.RUnlock()

	if len(pool.analyzers) != 1 {
		t.Errorf("Expected 1 analyzer, got %d", len(pool.analyzers))
	}

	analyzer := pool.analyzers[0]
	if analyzer.ID != "analyzer2" {
		t.Errorf("Expected remaining analyzer ID to be 'analyzer2', got '%s'", analyzer.ID)
	}

	if pool.totalWeight != 0.3 {
		t.Errorf("Expected total weight to be 0.3, got %f", pool.totalWeight)
	}
}

// TestGetActiveAnalyzers tests getting active analyzers
func TestGetActiveAnalyzers(t *testing.T) {
	pool := NewAnalyzerPool(time.Second * 10)

	// Add analyzers with different active states
	pool.AddAnalyzer("analyzer1", "http://example.com/1", 0.5)
	pool.AddAnalyzer("analyzer2", "http://example.com/2", 0.3)

	// Set one as inactive
	pool.SetAnalyzerActive("analyzer2", false)

	// Get active analyzers
	activeAnalyzers := pool.GetActiveAnalyzers()

	// Check result
	if len(activeAnalyzers) != 1 {
		t.Errorf("Expected 1 active analyzer, got %d", len(activeAnalyzers))
	}

	if activeAnalyzers[0].ID != "analyzer1" {
		t.Errorf("Expected active analyzer ID to be 'analyzer1', got '%s'", activeAnalyzers[0].ID)
	}
}

// TestSendLogPacket tests sending log packets to analyzers
func TestSendLogPacket(t *testing.T) {
	// Create a test HTTP server to act as analyzer
	var receivedPacket models.LogPacket
	var serverMutex sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/analyze" {
			// Decode the received packet
			serverMutex.Lock()
			defer serverMutex.Unlock()

			decoder := json.NewDecoder(r.Body)
			err := decoder.Decode(&receivedPacket)
			if err != nil {
				t.Errorf("Error decoding request body: %v", err)
				http.Error(w, "Bad request", http.StatusBadRequest)
				return
			}

			w.WriteHeader(http.StatusOK)
			return
		}

		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}

		http.Error(w, "Not found", http.StatusNotFound)
	}))
	defer server.Close()

	// Create analyzer pool
	pool := NewAnalyzerPool(time.Second * 10)
	pool.AddAnalyzer("test-analyzer", server.URL, 1.0)

	// Create test packet
	testPacket := &models.LogPacket{
		PacketID: "test-packet-id",
		AgentID:  "test-agent-id",
		SentAt:   time.Now(),
		LogMessages: []models.LogMessage{
			{
				ID:      "log1",
				Message: "Test message",
				Level:   models.Info,
			},
		},
	}

	// Send packet
	err := pool.SendLogPacket(context.Background(), pool.analyzers[0], testPacket)
	if err != nil {
		t.Fatalf("Failed to send log packet: %v", err)
	}

	// Verify packet was received correctly
	serverMutex.Lock()
	defer serverMutex.Unlock()

	if receivedPacket.PacketID != testPacket.PacketID {
		t.Errorf("Expected packet ID '%s', got '%s'", testPacket.PacketID, receivedPacket.PacketID)
	}

	if receivedPacket.AgentID != testPacket.AgentID {
		t.Errorf("Expected agent ID '%s', got '%s'", testPacket.AgentID, receivedPacket.AgentID)
	}

	if len(receivedPacket.LogMessages) != 1 {
		t.Fatalf("Expected 1 log message, got %d", len(receivedPacket.LogMessages))
	}

	if receivedPacket.LogMessages[0].ID != testPacket.LogMessages[0].ID {
		t.Errorf("Expected log message ID '%s', got '%s'", testPacket.LogMessages[0].ID, receivedPacket.LogMessages[0].ID)
	}
}

// TestHealthCheck tests the health check functionality
func TestHealthCheck(t *testing.T) {
	// Create a test HTTP server with controllable health status
	healthyServer := true
	var serverMutex sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			serverMutex.Lock()
			isHealthy := healthyServer
			serverMutex.Unlock()

			if isHealthy {
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusServiceUnavailable)
			}
			return
		}
		http.Error(w, "Not found", http.StatusNotFound)
	}))
	defer server.Close()

	// Create analyzer pool with short health check interval
	pool := NewAnalyzerPool(100 * time.Millisecond)
	pool.AddAnalyzer("test-analyzer", server.URL, 1.0)

	// Start health checks
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go pool.StartHealthCheck(ctx)

	// Initial state should be active
	time.Sleep(200 * time.Millisecond) // Wait for at least one health check
	activeAnalyzers := pool.GetActiveAnalyzers()
	if len(activeAnalyzers) != 1 {
		t.Fatalf("Expected 1 active analyzer, got %d", len(activeAnalyzers))
	}

	// Make server unhealthy
	serverMutex.Lock()
	healthyServer = false
	serverMutex.Unlock()

	// Wait for next health check
	time.Sleep(200 * time.Millisecond)
	activeAnalyzers = pool.GetActiveAnalyzers()
	if len(activeAnalyzers) != 0 {
		t.Fatalf("Expected 0 active analyzers after server becomes unhealthy, got %d", len(activeAnalyzers))
	}

	// Make server healthy again
	serverMutex.Lock()
	healthyServer = true
	serverMutex.Unlock()

	// Wait for next health check
	time.Sleep(200 * time.Millisecond)
	activeAnalyzers = pool.GetActiveAnalyzers()
	if len(activeAnalyzers) != 1 {
		t.Fatalf("Expected 1 active analyzer after server becomes healthy again, got %d", len(activeAnalyzers))
	}
}
