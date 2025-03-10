package realtime

import "time"

// API constants
const (
	DefaultModel = "gpt-4o-realtime-preview-2024-10-01"
	BaseURL      = "wss://api.openai.com/v1/realtime"
)

// Timeout constants
const (
	ConnectionTimeout = 45 * time.Second
	CloseTimeout      = 5 * time.Second
	PingWriteTimeout  = 10 * time.Second
	PongWait          = 60 * time.Second
	PingInterval      = (PongWait * 9) / 10
)
