package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/ryouol/log-distributor/pkg/models"
)

var (
	sources = []string{
		"web-server", "api-gateway", "database", "auth-service",
		"payment-service", "user-service", "notification-service",
	}
	logMessages = []string{
		"User logged in successfully",
		"Failed login attempt",
		"Database connection timeout",
		"Payment processed successfully",
		"Invalid request parameters",
		"Cache miss",
		"Rate limit exceeded",
		"Resource not found",
		"Permission denied",
		"Operation completed successfully",
	}
)

// Generator generates and sends log packets
type Generator struct {
	distributorURL string
	agentID        string
	rate           int
	batchSize      int
	client         *http.Client
}

// NewGenerator creates a new log generator
func NewGenerator(distributorURL, agentID string, rate, batchSize int) *Generator {
	return &Generator{
		distributorURL: distributorURL,
		agentID:        agentID,
		rate:           rate,
		batchSize:      batchSize,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Run starts generating and sending logs
func (g *Generator) Run(duration time.Duration) {
	// Calculate total number of packets to send
	totalPackets := int(duration.Seconds()) * g.rate

	// Channel to collect results
	resultCh := make(chan bool, totalPackets)

	log.Printf("Starting log generator: %d packets/sec, %d logs/packet, for %v\n",
		g.rate, g.batchSize, duration)

	// Start time
	startTime := time.Now()

	// Create wait group for workers
	var wg sync.WaitGroup

	// Start generator workers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go g.generatorWorker(&wg, totalPackets/10, resultCh)
	}

	// Wait for all workers to finish
	wg.Wait()
	close(resultCh)

	// Calculate statistics
	successCount := 0
	failCount := 0
	for success := range resultCh {
		if success {
			successCount++
		} else {
			failCount++
		}
	}

	// Print results
	elapsed := time.Since(startTime)
	log.Printf("Generator completed in %v\n", elapsed)
	log.Printf("Total packets sent: %d (Success: %d, Failed: %d)\n",
		successCount+failCount, successCount, failCount)
	log.Printf("Average rate: %.2f packets/sec\n", float64(successCount+failCount)/elapsed.Seconds())
	log.Printf("Total log messages: %d\n", (successCount+failCount)*g.batchSize)
}

// generatorWorker generates and sends log packets
func (g *Generator) generatorWorker(wg *sync.WaitGroup, count int, resultCh chan<- bool) {
	defer wg.Done()

	// Process count packets
	for i := 0; i < count; i++ {
		// Add some randomization to send rate
		if rand.Float64() < 0.1 {
			time.Sleep(time.Duration(rand.Intn(20)) * time.Millisecond)
		}

		// Generate and send packet
		success := g.generateAndSendPacket()
		resultCh <- success
	}
}

// generateAndSendPacket generates and sends a single log packet
func (g *Generator) generateAndSendPacket() bool {
	// Generate packet
	packet := g.generateLogPacket()

	// Marshal to JSON
	payload, err := json.Marshal(packet)
	if err != nil {
		log.Printf("Error marshaling packet: %v\n", err)
		return false
	}

	// Send to distributor
	req, err := http.NewRequest("POST", g.distributorURL+"/api/v1/logs", bytes.NewBuffer(payload))
	if err != nil {
		log.Printf("Error creating request: %v\n", err)
		return false
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		log.Printf("Error sending request: %v\n", err)
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusAccepted
}

// generateLogPacket generates a random log packet
func (g *Generator) generateLogPacket() *models.LogPacket {
	// Create packet
	packet := &models.LogPacket{
		PacketID:    uuid.New().String(),
		AgentID:     g.agentID,
		SentAt:      time.Now(),
		LogMessages: make([]models.LogMessage, g.batchSize),
	}

	// Generate log messages
	for i := 0; i < g.batchSize; i++ {
		// Random timestamp within last minute
		timestamp := time.Now().Add(-time.Duration(rand.Intn(60)) * time.Second)

		// Random level
		var level models.LogLevel
		r := rand.Float64()
		switch {
		case r < 0.6:
			level = models.Info
		case r < 0.8:
			level = models.Warning
		case r < 0.95:
			level = models.Error
		default:
			level = models.Fatal
		}

		// Random source
		source := sources[rand.Intn(len(sources))]

		// Random message
		message := logMessages[rand.Intn(len(logMessages))]

		// Create log message
		packet.LogMessages[i] = models.LogMessage{
			ID:        uuid.New().String(),
			Timestamp: timestamp,
			Level:     level,
			Source:    source,
			Message:   message,
			Metadata: map[string]interface{}{
				"request_id": uuid.New().String(),
				"user_id":    fmt.Sprintf("user-%d", rand.Intn(1000)),
			},
		}
	}

	return packet
}

func main() {
	// Parse command-line flags
	var (
		distributorURL = flag.String("url", "http://localhost:8080", "Distributor URL")
		agentID        = flag.String("agent", "test-agent", "Agent ID")
		rate           = flag.Int("rate", 10, "Packets per second")
		batchSize      = flag.Int("batch", 5, "Log messages per packet")
		duration       = flag.Duration("duration", 30*time.Second, "Test duration")
	)
	flag.Parse()

	// Seed random number generator
	rand.Seed(time.Now().UnixNano())

	// Create and run generator
	generator := NewGenerator(*distributorURL, *agentID, *rate, *batchSize)
	generator.Run(*duration)
}
