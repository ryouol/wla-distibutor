package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yourusername/log-distributor/pkg/analyzer"
	"github.com/yourusername/log-distributor/pkg/api"
	"github.com/yourusername/log-distributor/pkg/distributor"
)

func main() {
	// Parse command-line flags
	var (
		httpAddr            = flag.String("http-addr", ":8080", "HTTP server address")
		queueSize           = flag.Int("queue-size", 10000, "Size of the work queue")
		numWorkers          = flag.Int("workers", 10, "Number of worker goroutines")
		healthCheckInterval = flag.Duration("health-check-interval", 10*time.Second, "Interval for health checks")
		maxRetries          = flag.Int("max-retries", 3, "Maximum number of retries for failed packets")
		retryInterval       = flag.Duration("retry-interval", 5*time.Second, "Interval between retries")
	)
	flag.Parse()

	// Create analyzer pool
	analyzerPool := analyzer.NewAnalyzerPool(*healthCheckInterval)

	// Create log distributor
	logDistributor := distributor.NewLogDistributor(
		analyzerPool,
		distributor.WeightedRandom,
		*queueSize,
		*numWorkers,
		*maxRetries,
		*retryInterval,
	)

	// Create API server
	server := api.NewServer(*httpAddr, logDistributor, analyzerPool)

	// Context that will be canceled on shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the distributor workers
	logDistributor.Start(ctx)

	// Start health checks for analyzers
	go analyzerPool.StartHealthCheck(ctx)

	// Start the HTTP server
	server.Start()
	log.Printf("Log distributor started on %s\n", *httpAddr)

	// Wait for termination signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")

	// Create a timeout context for graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Stop the HTTP server
	if err := server.Stop(shutdownCtx); err != nil {
		log.Printf("Error during server shutdown: %v\n", err)
	}

	// Stop the distributor
	logDistributor.Stop()

	log.Println("Shutdown complete")
}
