package config

import (
	"github.com/anibaldeboni/zero-paper/atmosbyte/bme280"
	"github.com/anibaldeboni/zero-paper/atmosbyte/queue"
	"github.com/anibaldeboni/zero-paper/atmosbyte/web"
	"periph.io/x/devices/v3/bmxx80"
)

// WebConfig converts config to web.Config
func (c *AppConfig) WebConfig() *web.Config {
	return &web.Config{
		Port:            c.Web.Port,
		ReadTimeout:     c.Web.ReadTimeout,
		WriteTimeout:    c.Web.WriteTimeout,
		IdleTimeout:     c.Web.IdleTimeout,
		ShutdownTimeout: c.Timeouts.WebShutdownTimeout,
	}
}

// QueueConfig converts config to queue.QueueConfig
func (c *AppConfig) QueueConfig() queue.QueueConfig {
	return queue.QueueConfig{
		Workers:           c.Queue.Workers,
		BufferSize:        c.Queue.BufferSize,
		ShutdownTimeout:   c.Timeouts.QueueShutdownTimeout,
		ProcessingTimeout: c.Timeouts.ProcessingTimeout,
		RetryPolicy: queue.RetryPolicy{
			MaxRetries: c.Queue.Retry.MaxRetries,
			BaseDelay:  c.Queue.Retry.BaseDelay,
		},
		CircuitBreakerConfig: queue.CircuitBreakerConfig{
			FailureThreshold: c.Queue.Circuit.FailureThreshold,
			Timeout:          c.Queue.Circuit.Timeout,
		},
	}
}

// BME280Config converts config to bme280.Config
func (c *AppConfig) BME280Config() *bme280.Config {
	return &bme280.Config{
		Address: c.Sensor.BME280.I2CAddress,
		BusName: c.Sensor.BME280.I2CBus,
		Options: &bmxx80.DefaultOpts, // Use default options
	}
}

// SimulatedConfig converts config to bme280 simulation config
func (c *AppConfig) SimulatedConfig() *bme280.SimulatedConfig {
	return &bme280.SimulatedConfig{
		MinTemp:     c.Sensor.Simulation.MinTemperature,
		MaxTemp:     c.Sensor.Simulation.MaxTemperature,
		MinHumidity: c.Sensor.Simulation.MinHumidity,
		MaxHumidity: c.Sensor.Simulation.MaxHumidity,
		MinPressure: c.Sensor.Simulation.MinPressure,
		MaxPressure: c.Sensor.Simulation.MaxPressure,
	}
}
