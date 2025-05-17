#!/bin/bash

# Chaos test script for log distributor system
# This script randomly kills and restarts analyzer containers to test system resilience

echo "Starting chaos testing for log distributor system..."
echo "This will randomly kill and restart analyzer containers for 2 minutes"
echo "Press Ctrl+C to stop the test"

# Make sure docker-compose is running
if ! docker ps | grep -q "distributor"; then
  echo "Distributor not running. Starting docker-compose..."
  docker-compose up -d
  echo "Waiting for services to initialize..."
  sleep 15
fi

# Function to get analyzer metrics
function get_metrics() {
  curl -s http://localhost:8080/api/v1/metrics | jq .
}

# Function to kill a random analyzer
function kill_random_analyzer() {
  # Select a random analyzer (1-4)
  ANALYZER="analyzer$(( ( RANDOM % 4 ) + 1 ))"
  echo "$(date '+%Y-%m-%d %H:%M:%S') - Killing $ANALYZER"
  docker-compose stop "$ANALYZER"
  
  # Check the metrics after killing
  echo "Metrics after killing $ANALYZER:"
  get_metrics
  
  # Wait a random amount of time (5-15 seconds)
  SLEEP_TIME=$(( ( RANDOM % 10 ) + 5 ))
  echo "Waiting $SLEEP_TIME seconds before restarting..."
  sleep "$SLEEP_TIME"
  
  # Restart the analyzer
  echo "$(date '+%Y-%m-%d %H:%M:%S') - Restarting $ANALYZER"
  docker-compose start "$ANALYZER"
  
  # Wait for analyzer to become healthy
  sleep 5
  
  # Check the metrics after restarting
  echo "Metrics after restarting $ANALYZER:"
  get_metrics
}

# Function to kill multiple analyzers
function kill_multiple_analyzers() {
  # Kill 2 or 3 analyzers
  COUNT=$(( ( RANDOM % 2 ) + 2 ))
  ANALYZERS=()
  
  for i in $(seq 1 $COUNT); do
    # Select a random analyzer (1-4) that hasn't been selected yet
    while true; do
      ANALYZER="analyzer$(( ( RANDOM % 4 ) + 1 ))"
      if [[ ! " ${ANALYZERS[@]} " =~ " ${ANALYZER} " ]]; then
        ANALYZERS+=("$ANALYZER")
        break
      fi
    done
  done
  
  echo "$(date '+%Y-%m-%d %H:%M:%S') - Killing multiple analyzers: ${ANALYZERS[*]}"
  
  # Stop all selected analyzers
  for ANALYZER in "${ANALYZERS[@]}"; do
    docker-compose stop "$ANALYZER"
  done
  
  # Check the metrics after killing
  echo "Metrics after killing multiple analyzers:"
  get_metrics
  
  # Wait a random amount of time (10-20 seconds)
  SLEEP_TIME=$(( ( RANDOM % 10 ) + 10 ))
  echo "Waiting $SLEEP_TIME seconds before restarting..."
  sleep "$SLEEP_TIME"
  
  # Restart analyzers one by one
  for ANALYZER in "${ANALYZERS[@]}"; do
    echo "$(date '+%Y-%m-%d %H:%M:%S') - Restarting $ANALYZER"
    docker-compose start "$ANALYZER"
    sleep 3
  done
  
  # Wait for analyzers to become healthy
  sleep 5
  
  # Check the metrics after restarting
  echo "Metrics after restarting all analyzers:"
  get_metrics
}

# Run tests for 2 minutes
END_TIME=$(($(date +%s) + 120))

echo "Initial system metrics:"
get_metrics

while [ $(date +%s) -lt $END_TIME ]; do
  # Randomly choose between killing a single analyzer or multiple analyzers
  if [ $(( RANDOM % 3 )) -eq 0 ]; then
    kill_multiple_analyzers
  else
    kill_random_analyzer
  fi
  
  # Wait a random amount of time (10-20 seconds) between tests
  SLEEP_TIME=$(( ( RANDOM % 10 ) + 10 ))
  echo "Waiting $SLEEP_TIME seconds before next test..."
  sleep "$SLEEP_TIME"
done

echo "Chaos testing completed."
echo "Final system metrics:"
get_metrics 