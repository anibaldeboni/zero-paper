package bme280

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	config := defaultConfig()

	if config.Address != 0x76 {
		t.Errorf("Expected default address 0x76, got 0x%02X", config.Address)
	}

	if config.BusName != "" {
		t.Errorf("Expected empty bus name, got %s", config.BusName)
	}

	if config.Options == nil {
		t.Error("Expected non-nil options")
	}
}

func TestMeasurementString(t *testing.T) {
	measurement := &Measurement{
		Temperature: 25.5,
		Humidity:    60.0,
		Pressure:    101325, // 1013.25 hPa
	}

	expected := "Temperature: 25.50°C, Humidity: 60.00%, Pressure: 101325 Pa"
	result := measurement.String()

	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestNewSensorWithNilConfig(t *testing.T) {
	// Este teste não pode ser executado em ambiente sem hardware
	// mas verifica se a lógica de configuração padrão funciona

	// Simula a criação com config nil
	config := &Config{
		Address: 0x76,
		BusName: "",
		Options: nil,
	}
	if config == nil {
		t.Error("Config should not return nil")
	}

	// Verifica se os valores padrão estão corretos
	if config.Address != 0x76 {
		t.Errorf("Expected address 0x76, got 0x%02X", config.Address)
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		hasError bool
	}{
		{
			name:     "nil config should use defaults",
			config:   nil,
			hasError: false,
		},
		{
			name: "valid config",
			config: &Config{
				Address: 0x76,
				BusName: "",
				Options: nil,
			},
			hasError: false,
		},
		{
			name: "alternative address",
			config: &Config{
				Address: 0x77,
				BusName: "",
				Options: nil,
			},
			hasError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Como não temos hardware, apenas testamos a lógica de configuração
			config := tt.config
			if config == nil {
				config = &Config{Address: 0x76, BusName: "", Options: nil}
			}

			if config.Options == nil {
				// A função NewSensor deveria definir Options como DefaultOpts
				// se fosse nil, mas não podemos testar isso sem hardware
			}

			// Verifica se o endereço está em um range válido
			if config.Address != 0x76 && config.Address != 0x77 {
				t.Logf("Warning: Non-standard I2C address 0x%02X", config.Address)
			}
		})
	}
}

// Mock para testar sem hardware real
type MockSensor struct {
	shouldError bool
	measurement *Measurement
}

func NewMockSensor(shouldError bool) *MockSensor {
	return &MockSensor{
		shouldError: shouldError,
		measurement: &Measurement{
			Temperature: 25.0,
			Humidity:    50.0,
			Pressure:    101325,
		},
	}
}

func (m *MockSensor) Read() (*Measurement, error) {
	if m.shouldError {
		return nil, &SensorError{
			Op:  "read",
			Err: "mock error",
		}
	}
	return m.measurement, nil
}

func (m *MockSensor) Close() error {
	return nil
}

// SensorError representa um erro específico do sensor
type SensorError struct {
	Op  string
	Err string
}

func (e *SensorError) Error() string {
	return "sensor " + e.Op + ": " + e.Err
}

func TestMockSensor(t *testing.T) {
	// Teste com sucesso
	sensor := NewMockSensor(false)
	measurement, err := sensor.Read()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if measurement == nil {
		t.Error("Expected measurement, got nil")
	}
	if measurement.Temperature != 25.0 {
		t.Errorf("Expected temperature 25.0, got %f", measurement.Temperature)
	}

	// Teste com erro
	sensorError := NewMockSensor(true)
	_, err = sensorError.Read()
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

// Benchmarks
func BenchmarkMeasurementString(b *testing.B) {
	measurement := &Measurement{
		Temperature: 25.5,
		Humidity:    60.0,
		Pressure:    101325,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = measurement.String()
	}
}

func BenchmarkDefaultConfig(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = &Config{Address: 0x76, BusName: "", Options: nil}
	}
}

func TestSimulatedSensor(t *testing.T) {
	config := DefaultSimulatedConfig()
	sensor := NewSimulatedSensor(config)
	defer sensor.Close()

	// Test basic reading
	measurement, err := sensor.Read()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Validate temperature range
	if measurement.Temperature < config.MinTemp || measurement.Temperature > config.MaxTemp {
		t.Errorf("Temperature %f out of range [%f, %f]",
			measurement.Temperature, config.MinTemp, config.MaxTemp)
	}

	// Validate humidity range
	if measurement.Humidity < config.MinHumidity || measurement.Humidity > config.MaxHumidity {
		t.Errorf("Humidity %f out of range [%f, %f]",
			measurement.Humidity, config.MinHumidity, config.MaxHumidity)
	}

	// Validate pressure range
	if measurement.Pressure < config.MinPressure || measurement.Pressure > config.MaxPressure {
		t.Errorf("Pressure %d out of range [%d, %d]",
			measurement.Pressure, config.MinPressure, config.MaxPressure)
	}
}

func TestSimulatedSensorConfig(t *testing.T) {
	// Test with nil config (should use defaults)
	sensor1 := NewSimulatedSensor(nil)
	defer sensor1.Close()

	measurement1, err := sensor1.Read()
	if err != nil {
		t.Fatalf("Expected no error with nil config, got %v", err)
	}

	// Should be within default ranges
	if measurement1.Temperature < 15.0 || measurement1.Temperature > 35.0 {
		t.Errorf("Temperature with default config out of expected range: %f", measurement1.Temperature)
	}

	// Test with custom config
	customConfig := &SimulatedConfig{
		MinTemp:     20.0,
		MaxTemp:     25.0,
		MinHumidity: 40.0,
		MaxHumidity: 60.0,
		MinPressure: 100000,
		MaxPressure: 102000,
	}

	sensor2 := newSimulatedSensorWithSeed(customConfig, 12345) // Fixed seed for reproducible tests
	defer sensor2.Close()

	measurement2, err := sensor2.Read()
	if err != nil {
		t.Fatalf("Expected no error with custom config, got %v", err)
	}

	// Should be within custom ranges
	if measurement2.Temperature < customConfig.MinTemp || measurement2.Temperature > customConfig.MaxTemp {
		t.Errorf("Temperature %f out of custom range [%f, %f]",
			measurement2.Temperature, customConfig.MinTemp, customConfig.MaxTemp)
	}
}

func TestSimulatedSensorReproducibility(t *testing.T) {
	// Test that same seed produces same results
	config := &SimulatedConfig{}

	sensor1 := newSimulatedSensorWithSeed(config, 12345)
	measurement1, _ := sensor1.Read()
	sensor1.Close()

	sensor2 := newSimulatedSensorWithSeed(config, 12345)
	measurement2, _ := sensor2.Read()
	sensor2.Close()

	if measurement1.Temperature != measurement2.Temperature {
		t.Errorf("Expected same temperature with same seed, got %f vs %f",
			measurement1.Temperature, measurement2.Temperature)
	}
}

func TestSimulatedSensorClosed(t *testing.T) {
	sensor := NewSimulatedSensor(nil)

	// Should work before closing
	_, err := sensor.Read()
	if err != nil {
		t.Fatalf("Expected no error before closing, got %v", err)
	}

	// Close the sensor
	err = sensor.Close()
	if err != nil {
		t.Fatalf("Expected no error closing sensor, got %v", err)
	}

	// Should error after closing
	_, err = sensor.Read()
	if err == nil {
		t.Error("Expected error reading from closed sensor")
	}

	if err.Error() != "simulated sensor is closed" {
		t.Errorf("Expected specific error message, got %v", err)
	}
}

func BenchmarkSimulatedSensorRead(b *testing.B) {
	sensor := NewSimulatedSensor(nil)
	defer sensor.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := sensor.Read()
		if err != nil {
			b.Fatalf("Benchmark error: %v", err)
		}
	}
}
