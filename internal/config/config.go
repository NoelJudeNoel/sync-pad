package config

import "time"

const (
	Port            = 8080
	MaxMessageSize  = 64 * 1024 // 64KB
	MaxConnections  = 500
	RoomTTL         = 30 * time.Minute
	WriteTimeout    = 10 * time.Second
	PongWait        = 6 * time.Second
	PingPeriod      = 3 * time.Second
	CleanupInterval = 5 * time.Minute

	RateLimitConnsPerMin    = 10
	RateLimitMessagesPerMin = 100

	WebDir = "/var/www/sync-text"
)
