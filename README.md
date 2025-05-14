# Weighted Log Analyzer Distributor

A high-throughput log distributor system that receives log packets from multiple agents and distributes them to analyzers based on configurable weights. It handles analyzer failures gracefully and ensures proportional distribution according to weights.

## Features

- Weighted distribution of log messages to analyzer services
- High-throughput, non-blocking, thread-safe distribution
- Graceful handling of analyzer failures
- Auto-recovery when analyzers come back online
- Configurable worker pool and queue size
- Metrics tracking for log distribution
- Docker Compose setup for easy deployment and testing

## Architecture

The system consists of the following components:

1. **Log Distributor**: Central service that receives log packets and distributes them to analyzers
2. **Analyzer Pool**: Manages a set of analyzers with assigned weights
3. **Mock Analyzers**: Simulated services that process log packets
4. **Log Generator**: Test utility to generate and send log traffic

## Prerequisites

- Go 1.20 or higher
- Docker and Docker Compose (for containerized deployment)

## Running Locally

1. **Build the applications**:

   ```
   go build -o bin/distributor ./cmd/distributor
   go build -o bin/analyzer ./cmd/analyzer
   go build -o bin/generator ./cmd/generator
   ```

2. **Start the distributor**:

   ```
   ./bin/distributor
   ```

3. **Start multiple analyzers with different weights**:

   ```
   ./bin/analyzer -id analyzer1 -port 8081 -weight 0.4
   ./bin/analyzer -id analyzer2 -port 8082 -weight 0.3
   ./bin/analyzer -id analyzer3 -port 8083 -weight 0.2
   ./bin/analyzer -id analyzer4 -port 8084 -weight 0.1
   ```

4. **Register analyzers with the distributor**:

   ```
   curl -X POST -H "Content-Type: application/json" -d '{"id":"analyzer1","url":"http://localhost:8081","weight":0.4}' http://localhost:8080/api/v1/analyzers
   curl -X POST -H "Content-Type: application/json" -d '{"id":"analyzer2","url":"http://localhost:8082","weight":0.3}' http://localhost:8080/api/v1/analyzers
   curl -X POST -H "Content-Type: application/json" -d '{"id":"analyzer3","url":"http://localhost:8083","weight":0.2}' http://localhost:8080/api/v1/analyzers
   curl -X POST -H "Content-Type: application/json" -d '{"id":"analyzer4","url":"http://localhost:8084","weight":0.1}' http://localhost:8080/api/v1/analyzers
   ```

5. **Generate log traffic**:

   ```
   ./bin/generator -url http://localhost:8080 -rate 20 -batch 10 -duration 60s
   ```

## Docker Deployment

1. **Build and start the containers**:

   ```
   docker-compose up -d
   ```

   This will:
   - Start the distributor service
   - Start 4 analyzer services with weights 0.4, 0.3, 0.2, and 0.1
   - Register the analyzers with the distributor
   - Run the log generator to send traffic

2. **View logs**:

   ```
   docker-compose logs -f
   ```

3. **Check metrics**:

   ```
   curl http://localhost:8080/api/v1/metrics
   ```

4. **Test analyzer failure and recovery**:

   Stop an analyzer:
   ```
   docker-compose stop analyzer2
   ```

   Observe the logs to see how traffic is redistributed to the remaining analyzers.

   Restart the analyzer:
   ```
   docker-compose start analyzer2
   ```

   Observe how the analyzer gets reintegrated into the distribution.

## API Endpoints

- `POST /api/v1/logs` - Submit log packets
- `POST /api/v1/analyzers` - Register a new analyzer
- `DELETE /api/v1/analyzers/{id}` - Remove an analyzer
- `GET /api/v1/metrics` - Get distribution metrics
- `GET /health` - Health check endpoint

## Configuration

Configuration options can be set via command-line flags or through the config file at `config/config.json`.

## Design Decisions and Future Improvements

See the [WRITEUP.md](WRITEUP.md) document for additional considerations, improvements, and testing strategies.