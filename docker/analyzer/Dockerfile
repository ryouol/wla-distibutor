FROM golang:1.20-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum* ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o analyzer ./cmd/analyzer

# Start a new stage from scratch
FROM alpine:latest

WORKDIR /app

# Install curl for health checks
RUN apk --no-cache add curl

# Copy the binary from the builder stage
COPY --from=builder /app/analyzer .

# Expose port
EXPOSE 8081 8082 8083 8084

# Command to run the executable
ENTRYPOINT ["./analyzer"] 