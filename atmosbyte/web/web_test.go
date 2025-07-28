package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/anibaldeboni/zero-paper/atmosbyte/bme280"
)

// MockSensorProvider implements SensorProvider for testing
type MockSensorProvider struct {
	measurement bme280.Measurement
	err         error
}

func (m *MockSensorProvider) Read() (bme280.Measurement, error) {
	return m.measurement, m.err
}

func TestNewServer(t *testing.T) {
	sensor := &MockSensorProvider{}
	config := DefaultConfig()

	server := NewServer(sensor, false, config)

	if server == nil {
		t.Fatal("Expected server to be created")
	}

	if server.sensor != sensor {
		t.Error("Expected sensor to be set correctly")
	}

	if server.useSimulated != false {
		t.Error("Expected useSimulated to be false")
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Port != 8080 {
		t.Errorf("Expected port 8080, got %d", config.Port)
	}

	if config.ReadTimeout != 10*time.Second {
		t.Errorf("Expected read timeout 10s, got %v", config.ReadTimeout)
	}
}

func TestHandleMeasurements_Success(t *testing.T) {
	measurement := bme280.Measurement{
		Temperature: 25.5,
		Humidity:    60.0,
		Pressure:    101325,
	}

	sensor := &MockSensorProvider{measurement: measurement}
	server := NewServer(sensor, false, nil)

	req := httptest.NewRequest(http.MethodGet, "/measurements", nil)
	w := httptest.NewRecorder()

	server.handleMeasurements(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response MeasurementResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatal("Failed to decode response:", err)
	}

	if response.Temperature != 25.5 {
		t.Errorf("Expected temperature 25.5, got %f", response.Temperature)
	}

	if response.Humidity != 60.0 {
		t.Errorf("Expected humidity 60.0, got %f", response.Humidity)
	}

	if response.Pressure != 101325.0 {
		t.Errorf("Expected pressure 101325.0, got %f", response.Pressure)
	}

	if response.Source != "BME280" {
		t.Errorf("Expected source 'BME280', got '%s'", response.Source)
	}
}

func TestHandleMeasurements_SimulatedSource(t *testing.T) {
	measurement := bme280.Measurement{
		Temperature: 22.0,
		Humidity:    55.0,
		Pressure:    100000,
	}

	sensor := &MockSensorProvider{measurement: measurement}
	server := NewServer(sensor, true, nil) // useSimulated = true

	req := httptest.NewRequest(http.MethodGet, "/measurements", nil)
	w := httptest.NewRecorder()

	server.handleMeasurements(w, req)

	var response MeasurementResponse
	json.NewDecoder(w.Body).Decode(&response)

	if response.Source != "Simulated" {
		t.Errorf("Expected source 'Simulated', got '%s'", response.Source)
	}
}

func TestHandleMeasurements_SensorError(t *testing.T) {
	sensor := &MockSensorProvider{err: errors.New("sensor read error")}
	server := NewServer(sensor, false, nil)

	req := httptest.NewRequest(http.MethodGet, "/measurements", nil)
	w := httptest.NewRecorder()

	server.handleMeasurements(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}

	var response ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatal("Failed to decode error response:", err)
	}

	if !strings.Contains(response.Error, "Failed to read sensor data") {
		t.Errorf("Expected error message about sensor, got '%s'", response.Error)
	}
}

func TestHandleMeasurements_MethodNotAllowed(t *testing.T) {
	sensor := &MockSensorProvider{}
	server := NewServer(sensor, false, nil)

	req := httptest.NewRequest(http.MethodPost, "/measurements", nil)
	w := httptest.NewRecorder()

	server.handleMeasurements(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleHealth(t *testing.T) {
	measurement := bme280.Measurement{Temperature: 25.0, Humidity: 50.0, Pressure: 101325}
	sensor := &MockSensorProvider{measurement: measurement}
	server := NewServer(sensor, false, nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	server.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatal("Failed to decode response:", err)
	}

	if response["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got '%v'", response["status"])
	}

	if response["sensor"] != "connected" {
		t.Errorf("Expected sensor 'connected', got '%v'", response["sensor"])
	}
}

func TestHandleHealth_SimulatedSensor(t *testing.T) {
	sensor := &MockSensorProvider{}
	server := NewServer(sensor, true, nil) // useSimulated = true

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	server.handleHealth(w, req)

	var response map[string]interface{}
	json.NewDecoder(w.Body).Decode(&response)

	if response["sensor"] != "simulated" {
		t.Errorf("Expected sensor 'simulated', got '%v'", response["sensor"])
	}
}

func TestHandleRoot(t *testing.T) {
	sensor := &MockSensorProvider{}
	server := NewServer(sensor, false, nil)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	server.handleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatal("Failed to decode response:", err)
	}

	if response["service"] != "Atmosbyte Weather API" {
		t.Errorf("Expected service name, got '%v'", response["service"])
	}

	endpoints, ok := response["endpoints"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected endpoints to be a map")
	}

	if endpoints["/measurements"] == nil {
		t.Error("Expected /measurements endpoint to be documented")
	}
}

func TestShutdown(t *testing.T) {
	sensor := &MockSensorProvider{}
	server := NewServer(sensor, false, nil)

	// Start server in background
	go func() {
		server.Start()
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Test shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := server.Shutdown(ctx)
	if err != nil {
		t.Errorf("Expected clean shutdown, got error: %v", err)
	}
}

func BenchmarkHandleMeasurements(b *testing.B) {
	measurement := bme280.Measurement{
		Temperature: 25.5,
		Humidity:    60.0,
		Pressure:    101325,
	}

	sensor := &MockSensorProvider{measurement: measurement}
	server := NewServer(sensor, false, nil)

	req := httptest.NewRequest(http.MethodGet, "/measurements", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		server.handleMeasurements(w, req)
	}
}
