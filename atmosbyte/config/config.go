// Package config provides centralized configuration management for the Atmosbyte application.
// It supports loading configuration from YAML files with fallback to sensible defaults.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// AppConfig represents the complete application configuration
type AppConfig struct {
	// Web server configuration
	Web WebConfig `yaml:"web"`

	// Queue processing configuration
	Queue QueueConfig `yaml:"queue"`

	// Sensor configuration
	Sensor SensorConfig `yaml:"sensor"`

	// OpenWeather API configuration
	OpenWeather OpenWeatherConfig `yaml:"openweather"`

	// Timeouts and shutdown configuration
	Timeouts TimeoutConfig `yaml:"timeouts"`
}

// WebConfig contains HTTP server configuration
type WebConfig struct {
	Port         int           `yaml:"port"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	IdleTimeout  time.Duration `yaml:"idle_timeout"`
}

// QueueConfig contains queue processing configuration
type QueueConfig struct {
	Workers    int           `yaml:"workers"`
	BufferSize int           `yaml:"buffer_size"`
	Retry      RetryConfig   `yaml:"retry"`
	Circuit    CircuitConfig `yaml:"circuit_breaker"`
}

// RetryConfig contains retry policy configuration
type RetryConfig struct {
	MaxRetries int           `yaml:"max_retries"`
	BaseDelay  time.Duration `yaml:"base_delay"`
}

// CircuitConfig contains circuit breaker configuration
type CircuitConfig struct {
	FailureThreshold int           `yaml:"failure_threshold"`
	Timeout          time.Duration `yaml:"timeout"`
}

// SensorConfig contains sensor configuration
type SensorConfig struct {
	Type         string        `yaml:"type"`          // "hardware" or "simulated"
	ReadInterval time.Duration `yaml:"read_interval"` // Reading interval
	BME280       BME280Config  `yaml:"bme280"`        // Hardware sensor config
	Simulation   SimConfig     `yaml:"simulation"`    // Simulation config
}

// BME280Config contains BME280 hardware sensor configuration
type BME280Config struct {
	I2CAddress uint16 `yaml:"i2c_address"`
	I2CBus     string `yaml:"i2c_bus"`
}

// SimConfig contains simulation parameters
type SimConfig struct {
	MinTemperature float64 `yaml:"min_temperature"`
	MaxTemperature float64 `yaml:"max_temperature"`
	MinHumidity    float64 `yaml:"min_humidity"`
	MaxHumidity    float64 `yaml:"max_humidity"`
	MinPressure    int64   `yaml:"min_pressure"`
	MaxPressure    int64   `yaml:"max_pressure"`
}

// OpenWeatherConfig contains OpenWeather API configuration
type OpenWeatherConfig struct {
	Timeout time.Duration `yaml:"timeout"`
}

// TimeoutConfig contains various timeout configurations
type TimeoutConfig struct {
	ShutdownTimeout      time.Duration `yaml:"shutdown_timeout"`
	QueueShutdownTimeout time.Duration `yaml:"queue_shutdown_timeout"`
	WebShutdownTimeout   time.Duration `yaml:"web_shutdown_timeout"`
	ProcessingTimeout    time.Duration `yaml:"processing_timeout"`
}

// ConfigLoader handles loading and caching of configuration
type ConfigLoader struct {
	config *AppConfig
	mu     sync.RWMutex
	loaded bool
}

// Global instance - thread-safe singleton
var globalLoader = &ConfigLoader{}

// Load loads configuration from file or uses defaults
// Thread-safe: uses sync.RWMutex for concurrent access protection
func Load(configPath string) (*AppConfig, error) {
	globalLoader.mu.Lock()
	defer globalLoader.mu.Unlock()

	if globalLoader.loaded {
		// Return cached config
		return globalLoader.config, nil
	}

	config, err := loadConfigFromFile(configPath)
	if err != nil {
		// File not found or invalid, use defaults
		config = defaultConfig()
	}

	// Cache the loaded config
	globalLoader.config = config
	globalLoader.loaded = true

	return config, nil
}

// Get returns the cached configuration (must call Load first)
// Thread-safe: uses sync.RWMutex for read access
func Get() *AppConfig {
	globalLoader.mu.RLock()
	defer globalLoader.mu.RUnlock()

	if !globalLoader.loaded {
		panic("Configuration not loaded. Call config.Load() first.")
	}

	return globalLoader.config
}

// loadConfigFromFile attempts to load configuration from a YAML file
func loadConfigFromFile(configPath string) (*AppConfig, error) {
	// Try to find config file in various locations
	paths := []string{
		configPath,
		"atmosbyte.yaml",
		"atmosbyte.yml",
		"config/atmosbyte.yaml",
		"config/atmosbyte.yml",
		"/etc/atmosbyte/atmosbyte.yaml",
		"/etc/atmosbyte.yaml",
	}

	var configFile string
	var found bool

	for _, path := range paths {
		if path == "" {
			continue
		}

		if _, err := os.Stat(path); err == nil {
			configFile = path
			found = true
			break
		}
	}

	if !found {
		return nil, errors.New("configuration file not found in any of the expected locations")
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configFile, err)
	}

	var config AppConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", configFile, err)
	}

	// Validate and apply defaults for missing values
	applyDefaults(&config)

	return &config, nil
}

// defaultConfig returns a configuration with sensible defaults
func defaultConfig() *AppConfig {
	config := &AppConfig{}
	applyDefaults(config)
	return config
}

// applyDefaults fills in missing configuration values with sensible defaults
func applyDefaults(config *AppConfig) {
	// Web defaults
	if config.Web.Port == 0 {
		config.Web.Port = 8080
	}
	if config.Web.ReadTimeout == 0 {
		config.Web.ReadTimeout = 10 * time.Second
	}
	if config.Web.WriteTimeout == 0 {
		config.Web.WriteTimeout = 10 * time.Second
	}
	if config.Web.IdleTimeout == 0 {
		config.Web.IdleTimeout = 120 * time.Second
	}

	// Queue defaults
	if config.Queue.Workers == 0 {
		config.Queue.Workers = 2
	}
	if config.Queue.BufferSize == 0 {
		config.Queue.BufferSize = 120
	}
	if config.Queue.Retry.MaxRetries == 0 {
		config.Queue.Retry.MaxRetries = 10
	}
	if config.Queue.Retry.BaseDelay == 0 {
		config.Queue.Retry.BaseDelay = 5 * time.Second
	}
	if config.Queue.Circuit.FailureThreshold == 0 {
		config.Queue.Circuit.FailureThreshold = 5
	}
	if config.Queue.Circuit.Timeout == 0 {
		config.Queue.Circuit.Timeout = 60 * time.Second
	}

	// Sensor defaults
	if config.Sensor.Type == "" {
		config.Sensor.Type = "simulated"
	}
	if config.Sensor.ReadInterval == 0 {
		if config.Sensor.Type == "simulated" {
			config.Sensor.ReadInterval = 10 * time.Second
		} else {
			config.Sensor.ReadInterval = 60 * time.Second
		}
	}

	// BME280 defaults
	if config.Sensor.BME280.I2CAddress == 0 {
		config.Sensor.BME280.I2CAddress = 0x76
	}

	// Simulation defaults
	if config.Sensor.Simulation.MinTemperature == 0 && config.Sensor.Simulation.MaxTemperature == 0 {
		config.Sensor.Simulation.MinTemperature = 15.0
		config.Sensor.Simulation.MaxTemperature = 35.0
		config.Sensor.Simulation.MinHumidity = 30.0
		config.Sensor.Simulation.MaxHumidity = 80.0
		config.Sensor.Simulation.MinPressure = 98000
		config.Sensor.Simulation.MaxPressure = 102000
	}

	// OpenWeather defaults
	if config.OpenWeather.Timeout == 0 {
		config.OpenWeather.Timeout = 30 * time.Second
	}

	// Timeout defaults
	if config.Timeouts.ShutdownTimeout == 0 {
		config.Timeouts.ShutdownTimeout = 10 * time.Second
	}
	if config.Timeouts.QueueShutdownTimeout == 0 {
		config.Timeouts.QueueShutdownTimeout = 30 * time.Second
	}
	if config.Timeouts.WebShutdownTimeout == 0 {
		config.Timeouts.WebShutdownTimeout = 30 * time.Second
	}
	if config.Timeouts.ProcessingTimeout == 0 {
		config.Timeouts.ProcessingTimeout = 5 * time.Second
	}
}

// GenerateExampleConfig creates an example configuration file
func GenerateExampleConfig(outputPath string) error {
	config := defaultConfig()

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file %s: %w", outputPath, err)
	}

	return nil
}
