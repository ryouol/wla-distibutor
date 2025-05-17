package distributor

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ryouol/log-distributor/pkg/analyzer"
	"github.com/ryouol/log-distributor/pkg/models"
)

// MockAnalyzerPool implements the AnalyzerPoolInterface for testing
type MockAnalyzerPool struct {
	activeAnalyzers []*analyzer.Analyzer
	sentPackets     map[string][]*models.LogPacket
	errorOnSend     bool
	mutex           sync.Mutex
	totalWeight     float64
}

func NewMockAnalyzerPool() *MockAnalyzerPool {
	return &MockAnalyzerPool{
		activeAnalyzers: make([]*analyzer.Analyzer, 0),
		sentPackets:     make(map[string][]*models.LogPacket),
	}
}

func (m *MockAnalyzerPool) GetActiveAnalyzers() []*analyzer.Analyzer {
	return m.activeAnalyzers
}

func (m *MockAnalyzerPool) SendLogPacket(ctx context.Context, a *analyzer.Analyzer, p *models.LogPacket) error {
	if m.errorOnSend {
		return errors.New("simulated send error")
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.sentPackets[a.ID] == nil {
		m.sentPackets[a.ID] = make([]*models.LogPacket, 0)
	}
	m.sentPackets[a.ID] = append(m.sentPackets[a.ID], p)
	return nil
}

func (m *MockAnalyzerPool) AddAnalyzer(id string, weight float64) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.activeAnalyzers = append(m.activeAnalyzers, &analyzer.Analyzer{
		ID:     id,
		Weight: weight,
		Active: true,
	})
	m.recalculateTotalWeight()
}

func (m *MockAnalyzerPool) SetAnalyzerActive(id string, active bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for i, a := range m.activeAnalyzers {
		if a.ID == id {
			m.activeAnalyzers[i].Active = active
			break
		}
	}
	m.recalculateTotalWeight()
}

func (m *MockAnalyzerPool) recalculateTotalWeight() {
	total := 0.0
	for _, a := range m.activeAnalyzers {
		if a.Active {
			total += a.Weight
		}
	}
	m.totalWeight = total
}

func (m *MockAnalyzerPool) GetPacketCount(id string) int {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return len(m.sentPackets[id])
}

// StartHealthCheck is a no-op for tests
func (m *MockAnalyzerPool) StartHealthCheck(ctx context.Context) {}

// RemoveAnalyzer removes an analyzer from the pool
func (m *MockAnalyzerPool) RemoveAnalyzer(id string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for i, a := range m.activeAnalyzers {
		if a.ID == id {
			m.activeAnalyzers = append(m.activeAnalyzers[:i], m.activeAnalyzers[i+1:]...)
			m.recalculateTotalWeight()
			break
		}
	}
}

// AddAnalyzerWithURL adds an analyzer with a URL for testing
func (m *MockAnalyzerPool) AddAnalyzerWithURL(id, url string, weight float64) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.activeAnalyzers = append(m.activeAnalyzers, &analyzer.Analyzer{
		ID:     id,
		URL:    url,
		Weight: weight,
		Active: true,
	})
	m.recalculateTotalWeight()
}

// TestNewLogDistributor tests creation of a new log distributor
func TestNewLogDistributor(t *testing.T) {
	pool := NewMockAnalyzerPool()
	distributor := NewLogDistributor(
		pool,
		100,
		5,
		3,
		time.Second,
	)

	if distributor == nil {
		t.Fatal("Failed to create log distributor")
	}

	if distributor.workQueue == nil {
		t.Error("Work queue not initialized")
	}

	if distributor.retryQueue == nil {
		t.Error("Retry queue not initialized")
	}

	if distributor.metrics == nil {
		t.Error("Metrics not initialized")
	}
}

// TestDistributionWithNoAnalyzers tests behavior when no analyzers are available
func TestDistributionWithNoAnalyzers(t *testing.T) {
	pool := NewMockAnalyzerPool()
	distributor := NewLogDistributor(
		pool,
		100,
		5,
		3,
		time.Millisecond*10,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	distributor.Start(ctx)
	defer distributor.Stop()

	// Create a test packet
	packet := &models.LogPacket{
		PacketID: "test-packet",
		AgentID:  "test-agent",
		LogMessages: []models.LogMessage{
			{ID: "msg1", Message: "Test message"},
		},
	}

	// Send packet to distributor
	success := distributor.EnqueuePacket(packet)
	if !success {
		t.Fatal("Failed to enqueue packet")
	}

	// Wait for processing
	time.Sleep(time.Millisecond * 50)

	// Check metrics
	metrics := distributor.GetMetrics()
	if metrics.TotalPacketsReceived != 1 {
		t.Errorf("Expected 1 packet received, got %d", metrics.TotalPacketsReceived)
	}

	// Packet should be dropped after max retries
	if metrics.PacketsDropped == 0 {
		t.Error("Expected packet to be dropped when no analyzers available")
	}
}

// TestWeightedDistribution tests if distribution follows weights
func TestWeightedDistribution(t *testing.T) {
	pool := NewMockAnalyzerPool()
	pool.AddAnalyzer("analyzer1", 0.7)
	pool.AddAnalyzer("analyzer2", 0.3)

	distributor := NewLogDistributor(
		pool,
		100,
		5,
		3,
		time.Millisecond*10,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	distributor.Start(ctx)
	defer distributor.Stop()

	// Send many packets to test weight distribution
	numPackets := 1000
	for i := 0; i < numPackets; i++ {
		packet := &models.LogPacket{
			PacketID: "test-packet",
			AgentID:  "test-agent",
			LogMessages: []models.LogMessage{
				{ID: "msg1", Message: "Test message"},
			},
		}
		distributor.EnqueuePacket(packet)
	}

	// Wait for processing
	time.Sleep(time.Second)

	// Get counts
	count1 := pool.GetPacketCount("analyzer1")
	count2 := pool.GetPacketCount("analyzer2")
	total := count1 + count2

	// Check distribution roughly follows weights
	// Allow for a 10% margin of error due to randomness
	expectedCount1 := int(float64(total) * 0.7)
	margin := int(float64(total) * 0.1)

	if count1 < expectedCount1-margin || count1 > expectedCount1+margin {
		t.Errorf("Expected analyzer1 to receive ~%d packets (±%d), got %d",
			expectedCount1, margin, count1)
	}
}

// TestAnalyzerFailureAndRecovery tests if packets are rerouted when analyzers fail
func TestAnalyzerFailureAndRecovery(t *testing.T) {
	pool := NewMockAnalyzerPool()
	pool.AddAnalyzer("analyzer1", 0.5)
	pool.AddAnalyzer("analyzer2", 0.5)

	distributor := NewLogDistributor(
		pool,
		100,
		5,
		3,
		time.Millisecond*10,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	distributor.Start(ctx)
	defer distributor.Stop()

	// Phase 1: Both analyzers active
	for i := 0; i < 100; i++ {
		packet := &models.LogPacket{
			PacketID: "packet1-",
			AgentID:  "test-agent",
			LogMessages: []models.LogMessage{
				{ID: "msg1", Message: "Test message"},
			},
		}
		distributor.EnqueuePacket(packet)
	}

	time.Sleep(time.Millisecond * 100)

	initialCount1 := pool.GetPacketCount("analyzer1")
	initialCount2 := pool.GetPacketCount("analyzer2")

	// Phase 2: Simulate analyzer1 going down
	pool.SetAnalyzerActive("analyzer1", false)

	for i := 0; i < 100; i++ {
		packet := &models.LogPacket{
			PacketID: "packet2-",
			AgentID:  "test-agent",
			LogMessages: []models.LogMessage{
				{ID: "msg1", Message: "Test message"},
			},
		}
		distributor.EnqueuePacket(packet)
	}

	time.Sleep(time.Millisecond * 100)

	// All new packets should go to analyzer2
	midCount1 := pool.GetPacketCount("analyzer1")
	midCount2 := pool.GetPacketCount("analyzer2")

	if midCount1 != initialCount1 {
		t.Errorf("Expected analyzer1 to receive no new packets after failure")
	}

	if midCount2 <= initialCount2 {
		t.Errorf("Expected analyzer2 to receive all new packets after analyzer1 failure")
	}

	// Phase 3: Bring analyzer1 back online
	pool.SetAnalyzerActive("analyzer1", true)

	for i := 0; i < 100; i++ {
		packet := &models.LogPacket{
			PacketID: "packet3-",
			AgentID:  "test-agent",
			LogMessages: []models.LogMessage{
				{ID: "msg1", Message: "Test message"},
			},
		}
		distributor.EnqueuePacket(packet)
	}

	time.Sleep(time.Millisecond * 100)

	// Both analyzers should receive packets again
	finalCount1 := pool.GetPacketCount("analyzer1")
	finalCount2 := pool.GetPacketCount("analyzer2")

	if finalCount1 <= midCount1 {
		t.Errorf("Expected analyzer1 to receive packets after recovery")
	}

	if finalCount2 <= midCount2 {
		t.Errorf("Expected analyzer2 to continue receiving packets")
	}
}

// TestRetryMechanism tests if failed packets are retried
func TestRetryMechanism(t *testing.T) {
	pool := NewMockAnalyzerPool()
	pool.AddAnalyzer("analyzer1", 1.0)
	pool.errorOnSend = true // Force send failures

	distributor := NewLogDistributor(
		pool,
		100,
		5,
		2, // Set max retries to 2
		time.Millisecond*10,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	distributor.Start(ctx)
	defer distributor.Stop()

	// Send a packet
	packet := &models.LogPacket{
		PacketID: "test-packet",
		AgentID:  "test-agent",
		LogMessages: []models.LogMessage{
			{ID: "msg1", Message: "Test message"},
		},
	}
	distributor.EnqueuePacket(packet)

	// Wait for processing and retries
	time.Sleep(time.Millisecond * 100)

	// Check metrics
	metrics := distributor.GetMetrics()
	if metrics.TotalPacketsReceived != 1 {
		t.Errorf("Expected 1 packet received, got %d", metrics.TotalPacketsReceived)
	}

	// Packet should be dropped after max retries
	if metrics.PacketsDropped != 1 {
		t.Errorf("Expected 1 packet to be dropped after max retries, got %d", metrics.PacketsDropped)
	}

	// Now allow sends to succeed
	pool.errorOnSend = false
	distributor.EnqueuePacket(packet)

	// Wait for processing
	time.Sleep(time.Millisecond * 50)

	metrics = distributor.GetMetrics()
	if metrics.TotalPacketsSent != 1 {
		t.Errorf("Expected 1 packet sent after error resolved, got %d", metrics.TotalPacketsSent)
	}
}
