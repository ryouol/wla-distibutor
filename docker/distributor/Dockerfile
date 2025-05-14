FROM golang:1.20-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum* ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o distributor ./cmd/distributor

# Start a new stage from scratch
FROM alpine:latest

WORKDIR /app

# Install curl for health checks
RUN apk --no-cache add curl

# Copy the binary from the builder stage
COPY --from=builder /app/distributor .

# Expose port
EXPOSE 8080

# Command to run the executable
CMD ["./distributor"] 