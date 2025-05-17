package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ryouol/log-distributor/pkg/models"
)

// Analyzer represents a log analyzer service
type Analyzer struct {
	ID     string  `json:"id"`
	URL    string  `json:"url"`
	Weight float64 `json:"weight"`
	Active bool    `json:"active"`
}

// AnalyzerPool manages a pool of analyzers
type AnalyzerPool struct {
	analyzers           []*Analyzer
	totalWeight         float64
	mutex               sync.RWMutex
	healthCheckInterval time.Duration
	httpClient          *http.Client
}

// NewAnalyzerPool creates a new analyzer pool
func NewAnalyzerPool(healthCheckInterval time.Duration) *AnalyzerPool {
	return &AnalyzerPool{
		analyzers:           make([]*Analyzer, 0),
		healthCheckInterval: healthCheckInterval,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// AddAnalyzer adds a new analyzer to the pool
func (p *AnalyzerPool) AddAnalyzer(id, url string, weight float64) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	analyzer := &Analyzer{
		ID:     id,
		URL:    url,
		Weight: weight,
		Active: true,
	}

	p.analyzers = append(p.analyzers, analyzer)
	p.recalculateTotalWeight()
}

// RemoveAnalyzer removes an analyzer from the pool
func (p *AnalyzerPool) RemoveAnalyzer(id string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	for i, a := range p.analyzers {
		if a.ID == id {
			p.analyzers = append(p.analyzers[:i], p.analyzers[i+1:]...)
			p.recalculateTotalWeight()
			break
		}
	}
}

// GetActiveAnalyzers returns a list of active analyzers
func (p *AnalyzerPool) GetActiveAnalyzers() []*Analyzer {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	active := make([]*Analyzer, 0)
	for _, a := range p.analyzers {
		if a.Active {
			active = append(active, a)
		}
	}

	return active
}

// recalculateTotalWeight recalculates the total weight of active analyzers
func (p *AnalyzerPool) recalculateTotalWeight() {
	total := 0.0
	for _, a := range p.analyzers {
		if a.Active {
			total += a.Weight
		}
	}
	p.totalWeight = total
}

// SendLogPacket sends a log packet to the specified analyzer
func (p *AnalyzerPool) SendLogPacket(ctx context.Context, analyzer *Analyzer, packet *models.LogPacket) error {
	payload, err := json.Marshal(packet)
	if err != nil {
		return fmt.Errorf("failed to marshal log packet: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", analyzer.URL+"/analyze", strings.NewReader(string(payload)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		// Mark analyzer as inactive
		p.SetAnalyzerActive(analyzer.ID, false)
		return fmt.Errorf("failed to send log packet to analyzer %s: %w", analyzer.ID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("analyzer %s returned non-OK status: %d", analyzer.ID, resp.StatusCode)
	}

	return nil
}

// SetAnalyzerActive sets the active status of an analyzer
func (p *AnalyzerPool) SetAnalyzerActive(id string, active bool) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	for _, a := range p.analyzers {
		if a.ID == id {
			a.Active = active
			break
		}
	}

	p.recalculateTotalWeight()
}

// StartHealthCheck starts periodic health checks of all analyzers
func (p *AnalyzerPool) StartHealthCheck(ctx context.Context) {
	ticker := time.NewTicker(p.healthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.checkAllAnalyzers(ctx)
		}
	}
}

// checkAllAnalyzers checks the health of all analyzers
func (p *AnalyzerPool) checkAllAnalyzers(ctx context.Context) {
	p.mutex.RLock()
	analyzers := make([]*Analyzer, len(p.analyzers))
	copy(analyzers, p.analyzers)
	p.mutex.RUnlock()

	for _, a := range analyzers {
		go p.checkAnalyzerHealth(ctx, a)
	}
}

// checkAnalyzerHealth checks if an analyzer is healthy
func (p *AnalyzerPool) checkAnalyzerHealth(ctx context.Context, a *Analyzer) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", a.URL+"/health", nil)
	if err != nil {
		p.SetAnalyzerActive(a.ID, false)
		return
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		p.SetAnalyzerActive(a.ID, false)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		p.SetAnalyzerActive(a.ID, true)
	} else {
		p.SetAnalyzerActive(a.ID, false)
	}
}
