package config

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestConfigConcurrentAccess verifica se não há condições de corrida
func TestConfigConcurrentAccess(t *testing.T) {
	// Reset global state
	globalLoader = &ConfigLoader{}

	// Load config in the main goroutine
	cfg, err := Load("../test-config.yaml")
	if err != nil {
		t.Fatalf("Failed to load test config: %v", err)
	}

	// Verify some values were loaded correctly
	if cfg.Web.Port != 9090 {
		t.Errorf("Expected port 9090, got %d", cfg.Web.Port)
	}

	if cfg.Queue.Workers != 1 {
		t.Errorf("Expected 1 worker, got %d", cfg.Queue.Workers)
	}

	// Test concurrent access
	const numGoroutines = 100
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Multiple concurrent reads
			for j := 0; j < 10; j++ {
				cfg := Get()
				if cfg.Web.Port != 9090 {
					errors <- fmt.Errorf("race condition detected: wrong port %d", cfg.Web.Port)
					return
				}

				// Small delay to increase chance of race condition
				time.Sleep(time.Microsecond)
			}
		}()
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		t.Error(err)
	}
}

// TestConfigDefaults verifica valores padrão
func TestConfigDefaults(t *testing.T) {
	// Reset global state
	globalLoader = &ConfigLoader{}

	// Load with non-existent file to trigger defaults
	cfg, err := Load("non-existent-file.yaml")
	if err != nil {
		t.Fatalf("Load should not fail when using defaults: %v", err)
	}

	// Test default values
	if cfg.Web.Port != 8080 {
		t.Errorf("Expected default port 8080, got %d", cfg.Web.Port)
	}

	if cfg.Queue.Workers != 2 {
		t.Errorf("Expected default workers 2, got %d", cfg.Queue.Workers)
	}

	if cfg.Sensor.Type != "simulated" {
		t.Errorf("Expected default sensor type simulated, got %s", cfg.Sensor.Type)
	}

	if cfg.Timeouts.ShutdownTimeout != 10*time.Second {
		t.Errorf("Expected default shutdown timeout 10s, got %v", cfg.Timeouts.ShutdownTimeout)
	}
}

// TestConfigAdapters verifica se os adaptadores funcionam corretamente
func TestConfigAdapters(t *testing.T) {
	// Reset global state
	globalLoader = &ConfigLoader{}

	cfg, err := Load("../test-config.yaml")
	if err != nil {
		t.Fatalf("Failed to load test config: %v", err)
	}

	// Test web config adapter
	webConfig := cfg.WebConfig()
	if webConfig.Port != 9090 {
		t.Errorf("Web adapter failed: expected port 9090, got %d", webConfig.Port)
	}

	// Test queue config adapter
	queueConfig := cfg.QueueConfig()
	if queueConfig.Workers != 1 {
		t.Errorf("Queue adapter failed: expected 1 worker, got %d", queueConfig.Workers)
	}

	// Test BME280 config adapter
	bmeConfig := cfg.BME280Config()
	if bmeConfig.Address != 0x76 {
		t.Errorf("BME280 adapter failed: expected address 0x76, got 0x%x", bmeConfig.Address)
	}

	// Test simulated config adapter
	simConfig := cfg.SimulatedConfig()
	if simConfig.MinTemp != 15.0 {
		t.Errorf("Simulated adapter failed: expected min temp 15.0, got %f", simConfig.MinTemp)
	}
}
