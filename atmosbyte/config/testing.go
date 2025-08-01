package config

import (
	"github.com/anibaldeboni/zero-paper/atmosbyte/bme280"
	"github.com/anibaldeboni/zero-paper/atmosbyte/queue"
	"github.com/anibaldeboni/zero-paper/atmosbyte/web"
)

// TestConfig returns a configuration for testing purposes
func TestConfig() *AppConfig {
	return defaultConfig()
}

// TestWebConfig returns a web config for testing
func TestWebConfig() *web.Config {
	cfg := TestConfig()
	return cfg.WebConfig()
}

// TestQueueConfig returns a queue config for testing
func TestQueueConfig() queue.QueueConfig {
	cfg := TestConfig()
	return cfg.QueueConfig()
}

// TestBME280Config returns a BME280 config for testing
func TestBME280Config() *bme280.Config {
	cfg := TestConfig()
	return cfg.BME280Config()
}
