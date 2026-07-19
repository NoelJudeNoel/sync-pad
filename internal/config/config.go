package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Configuration is loaded once at startup from environment variables, with
// sensible defaults. Use config.Flags (parsed once in main) for any value that
// should not be reloaded after launch.
type Config struct {
	Port              string
	WebDir            string
	BasePath          string
	MaxMessageSize    int64
	MaxConnections    int
	RoomTTL           time.Duration
	WriteTimeout      time.Duration
	PongWait          time.Duration
	PingPeriod        time.Duration
	CleanupInterval   time.Duration
	RateLimitConns    int
	RateLimitMessages int
	AllowedOrigins    []string
}

// Default returns production-ready defaults. Override by setting the matching
// environment variable before calling Load().
func Default() Config {
	return Config{
		Port:              ":8080",
		WebDir:            "./web",
		BasePath:          "/s",
		MaxMessageSize:    64 * 1024,
		MaxConnections:    500,
		RoomTTL:           30 * time.Minute,
		WriteTimeout:      10 * time.Second,
		PongWait:          6 * time.Second,
		PingPeriod:        3 * time.Second,
		CleanupInterval:   5 * time.Minute,
		RateLimitConns:    10,
		RateLimitMessages: 100,
		AllowedOrigins:    []string{"http://localhost", "http://localhost:8080"},
	}
}

// Load reads SYNC_PAD_* environment variables and returns a Config. Any
// unset variable keeps its default. Invalid values fall back to defaults
// silently—startup log messages go through slog elsewhere.
func Load() Config {
	c := Default()

	if v := os.Getenv("SYNC_PAD_PORT"); v != "" {
		c.Port = v
	}
	if v := os.Getenv("SYNC_PAD_WEB_DIR"); v != "" {
		c.WebDir = v
	}
	if v := os.Getenv("SYNC_PAD_BASE_PATH"); v != "" {
		c.BasePath = strings.TrimRight(v, "/")
	}

	if v := os.Getenv("SYNC_PAD_MAX_MESSAGE_SIZE"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			c.MaxMessageSize = n
		}
	}
	if v := os.Getenv("SYNC_PAD_MAX_CONNECTIONS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			c.MaxConnections = n
		}
	}
	if v := os.Getenv("SYNC_PAD_ROOM_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			c.RoomTTL = d
		}
	}
	if v := os.Getenv("SYNC_PAD_PING_PERIOD"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			c.PingPeriod = d
		}
	}
	if v := os.Getenv("SYNC_PAD_PONG_WAIT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			c.PongWait = d
		}
	}
	if v := os.Getenv("SYNC_PAD_RATE_LIMIT_CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			c.RateLimitConns = n
		}
	}
	if v := os.Getenv("SYNC_PAD_RATE_LIMIT_MESSAGES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			c.RateLimitMessages = n
		}
	}
	if v := os.Getenv("SYNC_PAD_ALLOWED_ORIGINS"); v != "" {
		parts := strings.Split(v, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if s := strings.TrimSpace(p); s != "" {
				out = append(out, s)
			}
		}
		if len(out) > 0 {
			c.AllowedOrigins = out
		}
	}
	return c
}

// IsOriginAllowed reports whether the given Origin header is acceptable for
// WebSocket upgrade. Empty Origin (non-browser clients) is always allowed.
func (c Config) IsOriginAllowed(origin string) bool {
	if origin == "" {
		return true
	}
	for _, allowed := range c.AllowedOrigins {
		if origin == allowed {
			return true
		}
	}
	return false
}
