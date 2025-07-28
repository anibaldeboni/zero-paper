package bme280

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

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
	config := DefaultConfig()
	if config == nil {
		t.Error("DefaultConfig should not return nil")
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
				config = DefaultConfig()
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
		_ = DefaultConfig()
	}
}
