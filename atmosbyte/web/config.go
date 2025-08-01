package web

import "time"

// Config holds configuration options for the web server
type Config struct {
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration // Timeout for graceful shutdown
}
