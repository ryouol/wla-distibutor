package distributor

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/ryouol/log-distributor/pkg/analyzer"
	"github.com/ryouol/log-distributor/pkg/models"
)

// AnalyzerPoolInterface defines methods required by the log distributor
type AnalyzerPoolInterface interface {
	GetActiveAnalyzers() []*analyzer.Analyzer
	SendLogPacket(ctx context.Context, a *analyzer.Analyzer, p *models.LogPacket) error
	StartHealthCheck(ctx context.Context)
}

// DistributionMetrics tracks distribution metrics
type DistributionMetrics struct {
	TotalPacketsReceived int64
	TotalPacketsSent     int64
	PacketsDropped       int64
	PacketsByAnalyzer    map[string]int64
	mutex                sync.RWMutex
}

// LogDistributor distributes logs among analyzers based on their weights
type LogDistributor struct {
	analyzerPool  AnalyzerPoolInterface
	metrics       *DistributionMetrics
	workQueue     chan *models.LogPacket
	maxWorkers    int
	shutdownCh    chan struct{}
	workerWg      sync.WaitGroup
	retryQueue    chan *models.LogPacket
	maxRetries    int
	retryInterval time.Duration
}

// NewLogDistributor creates a new log distributor
func NewLogDistributor(
	pool AnalyzerPoolInterface,
	queueSize int,
	maxWorkers int,
	maxRetries int,
	retryInterval time.Duration,
) *LogDistributor {
	return &LogDistributor{
		analyzerPool:  pool,
		workQueue:     make(chan *models.LogPacket, queueSize),
		retryQueue:    make(chan *models.LogPacket, queueSize),
		maxWorkers:    maxWorkers,
		shutdownCh:    make(chan struct{}),
		maxRetries:    maxRetries,
		retryInterval: retryInterval,
		metrics: &DistributionMetrics{
			PacketsByAnalyzer: make(map[string]int64),
		},
	}
}

// Start starts the distributor workers
func (d *LogDistributor) Start(ctx context.Context) {
	// Start main workers
	for i := 0; i < d.maxWorkers; i++ {
		d.workerWg.Add(1)
		go d.worker(ctx)
	}

	// Start retry worker
	d.workerWg.Add(1)
	go d.retryWorker(ctx)
}

// Stop gracefully stops the distributor
func (d *LogDistributor) Stop() {
	close(d.shutdownCh)
	d.workerWg.Wait()
	close(d.workQueue)
	close(d.retryQueue)
}

// EnqueuePacket adds a log packet to the work queue
func (d *LogDistributor) EnqueuePacket(packet *models.LogPacket) bool {
	select {
	case d.workQueue <- packet:
		d.metrics.mutex.Lock()
		d.metrics.TotalPacketsReceived++
		d.metrics.mutex.Unlock()
		return true
	default:
		// Queue is full, packet is dropped
		d.metrics.mutex.Lock()
		d.metrics.PacketsDropped++
		d.metrics.mutex.Unlock()
		return false
	}
}

// GetMetrics returns the current distribution metrics
func (d *LogDistributor) GetMetrics() DistributionMetrics {
	d.metrics.mutex.RLock()
	defer d.metrics.mutex.RUnlock()

	// Make a copy to avoid race conditions
	packetsByAnalyzer := make(map[string]int64)
	for k, v := range d.metrics.PacketsByAnalyzer {
		packetsByAnalyzer[k] = v
	}

	return DistributionMetrics{
		TotalPacketsReceived: d.metrics.TotalPacketsReceived,
		TotalPacketsSent:     d.metrics.TotalPacketsSent,
		PacketsDropped:       d.metrics.PacketsDropped,
		PacketsByAnalyzer:    packetsByAnalyzer,
	}
}

// worker processes packets from the work queue
func (d *LogDistributor) worker(ctx context.Context) {
	defer d.workerWg.Done()

	for {
		select {
		case <-d.shutdownCh:
			return
		case <-ctx.Done():
			return
		case packet, ok := <-d.workQueue:
			if !ok {
				return
			}
			d.processPacket(ctx, packet, 0)
		}
	}
}

// retryWorker handles failed packets that need to be retried
func (d *LogDistributor) retryWorker(ctx context.Context) {
	defer d.workerWg.Done()

	ticker := time.NewTicker(d.retryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.shutdownCh:
			return
		case <-ctx.Done():
			return
		case packet, ok := <-d.retryQueue:
			if !ok {
				return
			}
			// Wait for retry interval before processing
			<-ticker.C
			d.processPacket(ctx, packet, packet.Metadata["retryCount"].(int))
		}
	}
}

// processPacket processes a single log packet and sends it to an analyzer
func (d *LogDistributor) processPacket(ctx context.Context, packet *models.LogPacket, retryCount int) {
	// Get active analyzers
	activeAnalyzers := d.analyzerPool.GetActiveAnalyzers()
	if len(activeAnalyzers) == 0 {
		// No active analyzers, put in retry queue if under retry limit
		if retryCount < d.maxRetries {
			// Add retry count to metadata
			if packet.Metadata == nil {
				packet.Metadata = make(map[string]interface{})
			}
			packet.Metadata["retryCount"] = retryCount + 1

			select {
			case d.retryQueue <- packet:
				// Successfully queued for retry
			default:
				// Retry queue full, packet dropped
				d.metrics.mutex.Lock()
				d.metrics.PacketsDropped++
				d.metrics.mutex.Unlock()
			}
		} else {
			// Max retries reached, packet dropped
			d.metrics.mutex.Lock()
			d.metrics.PacketsDropped++
			d.metrics.mutex.Unlock()
		}
		return
	}

	// Select analyzer using weighted random selection
	selectedAnalyzer := d.selectAnalyzerRandom(activeAnalyzers)

	// Send packet to selected analyzer
	err := d.analyzerPool.SendLogPacket(ctx, selectedAnalyzer, packet)
	if err != nil {
		// Failed to send, retry if under retry limit
		if retryCount < d.maxRetries {
			// Add retry count to metadata
			if packet.Metadata == nil {
				packet.Metadata = make(map[string]interface{})
			}
			packet.Metadata["retryCount"] = retryCount + 1

			select {
			case d.retryQueue <- packet:
				// Successfully queued for retry
			default:
				// Retry queue full, packet dropped
				d.metrics.mutex.Lock()
				d.metrics.PacketsDropped++
				d.metrics.mutex.Unlock()
			}
		} else {
			// Max retries reached, packet dropped
			d.metrics.mutex.Lock()
			d.metrics.PacketsDropped++
			d.metrics.mutex.Unlock()
		}
		return
	}

	// Update metrics
	d.metrics.mutex.Lock()
	d.metrics.TotalPacketsSent++
	d.metrics.PacketsByAnalyzer[selectedAnalyzer.ID]++
	d.metrics.mutex.Unlock()
}

// selectAnalyzerRandom selects an analyzer randomly based on weights
func (d *LogDistributor) selectAnalyzerRandom(analyzers []*analyzer.Analyzer) *analyzer.Analyzer {
	if len(analyzers) == 1 {
		return analyzers[0]
	}

	// Calculate total weight of active analyzers
	totalWeight := 0.0
	for _, a := range analyzers {
		totalWeight += a.Weight
	}

	// Generate random value between 0 and total weight
	r := rand.Float64() * totalWeight

	// Find the analyzer that corresponds to this random value
	currentWeight := 0.0
	for _, a := range analyzers {
		currentWeight += a.Weight
		if r <= currentWeight {
			return a
		}
	}

	// Fallback to first analyzer (should never happen unless weights are 0)
	return analyzers[0]
}
