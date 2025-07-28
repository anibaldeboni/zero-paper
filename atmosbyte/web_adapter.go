package main

import (
	"math/rand"
	"time"

	"github.com/anibaldeboni/zero-paper/atmosbyte/bme280"
	"github.com/anibaldeboni/zero-paper/atmosbyte/web"
)

// BME280WebAdapter adapts BME280 sensor to work with web.SensorProvider interface
type BME280WebAdapter struct {
	sensor *bme280.Sensor
}

// NewBME280WebAdapter creates a new adapter for BME280 sensor
func NewBME280WebAdapter(sensor *bme280.Sensor) web.SensorProvider {
	return &BME280WebAdapter{sensor: sensor}
}

// Read implements web.SensorProvider interface
func (a *BME280WebAdapter) Read() (bme280.Measurement, error) {
	measurement, err := a.sensor.Read()
	if err != nil {
		return bme280.Measurement{}, err
	}
	return *measurement, nil
}

// SimulatedWebSensor provides simulated sensor data for web endpoints
type SimulatedWebSensor struct {
	rand *rand.Rand
}

// NewSimulatedWebSensor creates a new simulated sensor for web endpoints
func NewSimulatedWebSensor() web.SensorProvider {
	return &SimulatedWebSensor{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Read implements web.SensorProvider interface with simulated data
func (s *SimulatedWebSensor) Read() (bme280.Measurement, error) {
	// Generate realistic sensor readings with some variation
	baseTemp := 22.0 + (s.rand.Float64()-0.5)*10.0               // 17-27°C
	baseHumidity := 50.0 + (s.rand.Float64()-0.5)*30.0           // 35-65%
	basePressure := 101325 + int64((s.rand.Float64()-0.5)*10000) // ±5000 Pa

	return bme280.Measurement{
		Temperature: baseTemp,
		Humidity:    baseHumidity,
		Pressure:    basePressure,
	}, nil
}
