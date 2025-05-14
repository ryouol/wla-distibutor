package models

import (
	"time"
)

// LogLevel represents the severity of a log message
type LogLevel string

// Log levels
const (
	Debug   LogLevel = "DEBUG"
	Info    LogLevel = "INFO"
	Warning LogLevel = "WARNING"
	Error   LogLevel = "ERROR"
	Fatal   LogLevel = "FATAL"
)

// LogMessage represents a single log entry
type LogMessage struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Level     LogLevel               `json:"level"`
	Source    string                 `json:"source"`
	Message   string                 `json:"message"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// LogPacket represents a collection of log messages sent in a single request
type LogPacket struct {
	PacketID    string                 `json:"packet_id"`
	AgentID     string                 `json:"agent_id"`
	SentAt      time.Time              `json:"sent_at"`
	ReceivedAt  time.Time              `json:"received_at"`
	LogMessages []LogMessage           `json:"log_messages"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}
