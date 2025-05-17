#!/bin/bash

# Load test script for log distributor system
# This script bombards the distributor with a high volume of log packets

# Default parameters
URL="http://localhost:8080/api/v1/logs"
CONCURRENT_CLIENTS=10
REQUESTS_PER_CLIENT=1000
LOG_MESSAGES_PER_PACKET=50
DELAY_MS=0

# Parse command line arguments
while getopts ":u:c:r:m:d:" opt; do
  case ${opt} in
    u )
      URL=$OPTARG
      ;;
    c )
      CONCURRENT_CLIENTS=$OPTARG
      ;;
    r )
      REQUESTS_PER_CLIENT=$OPTARG
      ;;
    m )
      LOG_MESSAGES_PER_PACKET=$OPTARG
      ;;
    d )
      DELAY_MS=$OPTARG
      ;;
    \? )
      echo "Invalid option: $OPTARG" 1>&2
      exit 1
      ;;
    : )
      echo "Invalid option: $OPTARG requires an argument" 1>&2
      exit 1
      ;;
  esac
done

echo "Starting load test with the following parameters:"
echo "URL: $URL"
echo "Concurrent clients: $CONCURRENT_CLIENTS"
echo "Requests per client: $REQUESTS_PER_CLIENT"
echo "Log messages per packet: $LOG_MESSAGES_PER_PACKET"
echo "Delay between requests (ms): $DELAY_MS"

# Generate a random string of specified length
generate_random_string() {
  local length=$1
  local chars="abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
  local result=""
  
  for (( i=0; i<$length; i++ )); do
    result+=${chars:$((RANDOM % ${#chars})):1}
  done
  
  echo "$result"
}

# Generate a random log message
generate_log_message() {
  local LEVEL=("DEBUG" "INFO" "WARNING" "ERROR" "FATAL")
  local SOURCE=("app-server" "db-server" "cache-server" "auth-service" "api-gateway")
  local MSG_PREFIX=("Connection" "Transaction" "User" "Database" "System" "Network" "Cache" "File")
  local MSG_ACTION=("created" "updated" "deleted" "failed" "succeeded" "initiated" "completed" "expired")
  
  # Generate random ID
  local ID=$(generate_random_string 8)
  
  # Select random level, source, and message components
  local LVL=${LEVEL[$((RANDOM % ${#LEVEL[@]}))]}
  local SRC=${SOURCE[$((RANDOM % ${#SOURCE[@]}))]}
  local PREFIX=${MSG_PREFIX[$((RANDOM % ${#MSG_PREFIX[@]}))]}
  local ACTION=${MSG_ACTION[$((RANDOM % ${#MSG_ACTION[@]}))]}
  
  # Create timestamp slightly in the past (0-60 seconds)
  local PAST_SEC=$((RANDOM % 60))
  local TIMESTAMP=$(date -u -v-${PAST_SEC}S +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u -d "-${PAST_SEC} seconds" +"%Y-%m-%dT%H:%M:%SZ")
  
  # Generate trace ID
  local TRACE_ID=$(generate_random_string 32)
  
  # Create JSON object for log message
  echo "{\"id\":\"$ID\",\"timestamp\":\"$TIMESTAMP\",\"level\":\"$LVL\",\"source\":\"$SRC\",\"message\":\"$PREFIX $ACTION\",\"metadata\":{\"trace_id\":\"$TRACE_ID\"}}"
}

# Generate a full log packet
generate_log_packet() {
  local PACKET_ID=$(generate_random_string 12)
  local AGENT_ID="load-test-agent-$((RANDOM % 10 + 1))"
  local SENT_AT=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
  
  # Start JSON packet
  echo -n "{\"packet_id\":\"$PACKET_ID\",\"agent_id\":\"$AGENT_ID\",\"sent_at\":\"$SENT_AT\",\"log_messages\":["
  
  # Generate log messages
  for i in $(seq 1 $LOG_MESSAGES_PER_PACKET); do
    if [ $i -gt 1 ]; then
      echo -n ","
    fi
    generate_log_message
  done
  
  # Close JSON packet
  echo -n "]}"
}

# Function to send requests for a single client
run_client() {
  local CLIENT_ID=$1
  local SUCCESS=0
  local FAILED=0
  
  echo "Client $CLIENT_ID starting..."
  
  for i in $(seq 1 $REQUESTS_PER_CLIENT); do
    # Generate a log packet
    PACKET=$(generate_log_packet)
    
    # Send the request
    RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" -X POST -H "Content-Type: application/json" -d "$PACKET" $URL)
    
    # Check response
    if [ "$RESPONSE" -eq 202 ] || [ "$RESPONSE" -eq 200 ]; then
      SUCCESS=$((SUCCESS + 1))
    else
      FAILED=$((FAILED + 1))
      echo "Client $CLIENT_ID request $i failed with code $RESPONSE"
    fi
    
    # Add delay if specified
    if [ $DELAY_MS -gt 0 ]; then
      sleep $(echo "scale=3; $DELAY_MS/1000" | bc)
    fi
    
    # Print progress every 100 requests
    if [ $((i % 100)) -eq 0 ]; then
      echo "Client $CLIENT_ID progress: $i/$REQUESTS_PER_CLIENT"
    fi
  done
  
  echo "Client $CLIENT_ID completed. Success: $SUCCESS, Failed: $FAILED"
  echo "$SUCCESS $FAILED" > /tmp/load_test_client_$CLIENT_ID.result
}

# Start the test
START_TIME=$(date +%s)

# Start all clients in background
for i in $(seq 1 $CONCURRENT_CLIENTS); do
  run_client $i &
done

# Wait for all clients to finish
echo "Waiting for all clients to complete..."
wait

# Calculate totals
SUCCESS_TOTAL=0
FAILED_TOTAL=0

for i in $(seq 1 $CONCURRENT_CLIENTS); do
  if [ -f /tmp/load_test_client_$i.result ]; then
    read SUCCESS FAILED < /tmp/load_test_client_$i.result
    SUCCESS_TOTAL=$((SUCCESS_TOTAL + SUCCESS))
    FAILED_TOTAL=$((FAILED_TOTAL + FAILED))
    rm /tmp/load_test_client_$i.result
  fi
done

END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

# Calculate statistics
TOTAL_REQUESTS=$((SUCCESS_TOTAL + FAILED_TOTAL))
TOTAL_LOG_MESSAGES=$((SUCCESS_TOTAL * LOG_MESSAGES_PER_PACKET))
THROUGHPUT=$(echo "scale=2; $TOTAL_REQUESTS / $DURATION" | bc)
LOG_THROUGHPUT=$(echo "scale=2; $TOTAL_LOG_MESSAGES / $DURATION" | bc)

# Print results
echo "Load test completed in $DURATION seconds"
echo "Total requests: $TOTAL_REQUESTS"
echo "Successful requests: $SUCCESS_TOTAL"
echo "Failed requests: $FAILED_TOTAL"
echo "Success rate: $(echo "scale=2; ($SUCCESS_TOTAL * 100) / $TOTAL_REQUESTS" | bc)%"
echo "Request throughput: $THROUGHPUT requests/second"
echo "Log message throughput: $LOG_THROUGHPUT log messages/second"

# Get system metrics
echo "Checking system metrics..."
curl -s http://localhost:8080/api/v1/metrics | jq . 