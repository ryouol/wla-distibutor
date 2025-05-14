#!/bin/bash

# Create necessary directories
mkdir -p bin logs

# Build the binaries
echo "Building binaries..."
go build -o bin/distributor ./cmd/distributor
go build -o bin/analyzer ./cmd/analyzer
go build -o bin/generator ./cmd/generator

# Start distributor in background
echo "Starting distributor..."
bin/distributor > logs/distributor.log 2>&1 &
DISTRIBUTOR_PID=$!

# Wait for distributor to start
sleep 2

# Start analyzers in background
echo "Starting analyzers..."
bin/analyzer -id analyzer1 -port 8081 -weight 0.4 > logs/analyzer1.log 2>&1 &
ANALYZER1_PID=$!

bin/analyzer -id analyzer2 -port 8082 -weight 0.3 > logs/analyzer2.log 2>&1 &
ANALYZER2_PID=$!

bin/analyzer -id analyzer3 -port 8083 -weight 0.2 > logs/analyzer3.log 2>&1 &
ANALYZER3_PID=$!

bin/analyzer -id analyzer4 -port 8084 -weight 0.1 > logs/analyzer4.log 2>&1 &
ANALYZER4_PID=$!

# Wait for analyzers to start
sleep 2

# Register analyzers
echo "Registering analyzers..."
curl -X POST -H "Content-Type: application/json" -d '{"id":"analyzer1","url":"http://localhost:8081","weight":0.4}' http://localhost:8080/api/v1/analyzers
curl -X POST -H "Content-Type: application/json" -d '{"id":"analyzer2","url":"http://localhost:8082","weight":0.3}' http://localhost:8080/api/v1/analyzers
curl -X POST -H "Content-Type: application/json" -d '{"id":"analyzer3","url":"http://localhost:8083","weight":0.2}' http://localhost:8080/api/v1/analyzers
curl -X POST -H "Content-Type: application/json" -d '{"id":"analyzer4","url":"http://localhost:8084","weight":0.1}' http://localhost:8080/api/v1/analyzers

# Wait for a moment
sleep 1

# Run log generator
echo "Generating logs..."
bin/generator -url http://localhost:8080 -rate 20 -batch 10 -duration 30s

# Check metrics
echo "Checking metrics..."
curl -s http://localhost:8080/api/v1/metrics | jq .

# Simulate analyzer failure
echo "Simulating analyzer failure (stopping analyzer2)..."
kill $ANALYZER2_PID

# Generate more logs
echo "Generating more logs after failure..."
bin/generator -url http://localhost:8080 -rate 20 -batch 10 -duration 15s

# Check metrics again
echo "Checking metrics after failure..."
curl -s http://localhost:8080/api/v1/metrics | jq .

# Restart analyzer
echo "Restarting analyzer2..."
bin/analyzer -id analyzer2 -port 8082 -weight 0.3 > logs/analyzer2_restarted.log 2>&1 &
ANALYZER2_PID=$!

# Wait for analyzer to start
sleep 2

# Register analyzer again
curl -X POST -H "Content-Type: application/json" -d '{"id":"analyzer2","url":"http://localhost:8082","weight":0.3}' http://localhost:8080/api/v1/analyzers

# Generate more logs
echo "Generating more logs after recovery..."
bin/generator -url http://localhost:8080 -rate 20 -batch 10 -duration 15s

# Check metrics one more time
echo "Final metrics:"
curl -s http://localhost:8080/api/v1/metrics | jq .

# Cleanup
echo "Cleaning up..."
kill $DISTRIBUTOR_PID $ANALYZER1_PID $ANALYZER2_PID $ANALYZER3_PID $ANALYZER4_PID

echo "Test completed. Log files are in the logs directory." 