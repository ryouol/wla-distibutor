.PHONY: build test clean run-distributor run-analyzers load-test chaos-test unit-test all

# Variables
GO := go
GOFMT := gofmt
BINDIR := ./bin
GOFLAGS := -ldflags="-s -w"

# Go source files
SOURCES := $(shell find . -name "*.go" -not -path "./vendor/*")

# Binary files
DISTRIBUTOR := $(BINDIR)/distributor
ANALYZER := $(BINDIR)/analyzer
GENERATOR := $(BINDIR)/generator

# Default target
all: build

# Build all binaries
build: $(DISTRIBUTOR) $(ANALYZER) $(GENERATOR)

# Build distributor binary
$(DISTRIBUTOR): $(SOURCES)
	@mkdir -p $(BINDIR)
	$(GO) build $(GOFLAGS) -o $(DISTRIBUTOR) ./cmd/distributor

# Build analyzer binary
$(ANALYZER): $(SOURCES)
	@mkdir -p $(BINDIR)
	$(GO) build $(GOFLAGS) -o $(ANALYZER) ./cmd/analyzer

# Build generator binary
$(GENERATOR): $(SOURCES)
	@mkdir -p $(BINDIR)
	$(GO) build $(GOFLAGS) -o $(GENERATOR) ./cmd/generator

# Run unit tests
unit-test:
	$(GO) test -v ./pkg/...

# Format go code
fmt:
	$(GOFMT) -w $(SOURCES)

# Check code formatting
fmt-check:
	@test -z "$$($(GOFMT) -l $(SOURCES))" || (echo "Code is not formatted properly. Run 'make fmt'"; exit 1)

# Clean build artifacts
clean:
	rm -rf $(BINDIR)

# Run distributor locally
run-distributor: $(DISTRIBUTOR)
	$(DISTRIBUTOR) --http-addr=:8080 --queue-size=10000 --workers=10

# Run analyzers locally
run-analyzers: $(ANALYZER)
	$(ANALYZER) -id analyzer1 -port 8081 -weight 0.4 & \
	$(ANALYZER) -id analyzer2 -port 8082 -weight 0.3 & \
	$(ANALYZER) -id analyzer3 -port 8083 -weight 0.2 & \
	$(ANALYZER) -id analyzer4 -port 8084 -weight 0.1

# Register analyzers with distributor
register-analyzers:
	curl -X POST -H "Content-Type: application/json" -d '{"id":"analyzer1","url":"http://localhost:8081","weight":0.4}' http://localhost:8080/api/v1/analyzers
	curl -X POST -H "Content-Type: application/json" -d '{"id":"analyzer2","url":"http://localhost:8082","weight":0.3}' http://localhost:8080/api/v1/analyzers
	curl -X POST -H "Content-Type: application/json" -d '{"id":"analyzer3","url":"http://localhost:8083","weight":0.2}' http://localhost:8080/api/v1/analyzers
	curl -X POST -H "Content-Type: application/json" -d '{"id":"analyzer4","url":"http://localhost:8084","weight":0.1}' http://localhost:8080/api/v1/analyzers

# Run load test
load-test: $(GENERATOR)
	@chmod +x scripts/load_test.sh
	scripts/load_test.sh

# Run chaos test
chaos-test:
	@chmod +x scripts/chaos_test.sh
	scripts/chaos_test.sh

# Run in Docker with docker-compose
docker-run:
	docker-compose up -d

# Stop docker-compose
docker-stop:
	docker-compose down

# View Docker logs
docker-logs:
	docker-compose logs -f

# Docker build only
docker-build:
	docker-compose build

# Get metrics from running distributor
metrics:
	curl -s http://localhost:8080/api/v1/metrics | jq .

# Help
help:
	@echo "Available targets:"
	@echo "  all             - Build all binaries"
	@echo "  build           - Build all binaries"
	@echo "  unit-test       - Run unit tests"
	@echo "  fmt             - Format Go code"
	@echo "  fmt-check       - Check code formatting"
	@echo "  clean           - Remove build artifacts"
	@echo "  run-distributor - Run distributor locally"
	@echo "  run-analyzers   - Run analyzers locally"
	@echo "  register-analyzers - Register analyzers with distributor"
	@echo "  load-test       - Run load test script"
	@echo "  chaos-test      - Run chaos test script"
	@echo "  docker-run      - Run with docker-compose"
	@echo "  docker-stop     - Stop docker-compose"
	@echo "  docker-logs     - View docker-compose logs"
	@echo "  docker-build    - Build docker images"
	@echo "  metrics         - Get metrics from running distributor" 