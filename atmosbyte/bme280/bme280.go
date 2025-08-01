package bme280

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"periph.io/x/conn/v3/i2c"
	"periph.io/x/conn/v3/i2c/i2creg"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/devices/v3/bmxx80"
	"periph.io/x/host/v3"
)

// Measurement representa uma leitura do sensor BME280
type Measurement struct {
	Temperature float64 `json:"temperature"` // Temperatura em Celsius
	Humidity    float64 `json:"humidity"`    // Umidade relativa em %
	Pressure    int64   `json:"pressure"`    // Pressão em Pascal
}

// Sensor representa um sensor BME280 conectado via I2C
type Sensor struct {
	device   *bmxx80.Dev
	bus      i2c.BusCloser
	address  uint16
	options  *bmxx80.Opts
	initOnce sync.Once
	mu       sync.RWMutex
}

// Config contém as configurações para o sensor BME280
type Config struct {
	Address uint16       // Endereço I2C do sensor (padrão: 0x76)
	BusName string       // Nome do barramento I2C (vazio para padrão)
	Options *bmxx80.Opts // Opções avançadas (nil para padrão)
}

// defaultConfig retorna uma configuração padrão para o sensor (interno)
func defaultConfig() *Config {
	return &Config{
		Address: 0x76,
		BusName: "",
		Options: &bmxx80.DefaultOpts,
	}
}

// NewSensor cria uma nova instância do sensor BME280
func NewSensor(config *Config) (*Sensor, error) {
	if config == nil {
		config = defaultConfig()
	}

	if config.Options == nil {
		config.Options = &bmxx80.DefaultOpts
	}

	sensor := &Sensor{
		address: config.Address,
		options: config.Options,
	}

	// Inicializa os drivers do periph.io
	if _, err := host.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize periph.io drivers: %w", err)
	}

	// Abre o barramento I2C
	bus, err := i2creg.Open(config.BusName)
	if err != nil {
		return nil, fmt.Errorf("failed to open I2C bus '%s': %w", config.BusName, err)
	}
	sensor.bus = bus

	// Conecta ao dispositivo BME280
	dev, err := bmxx80.NewI2C(bus, config.Address, config.Options)
	if err != nil {
		bus.Close()
		return nil, fmt.Errorf("failed to initialize BME280 at address 0x%02X: %w", config.Address, err)
	}
	sensor.device = dev

	return sensor, nil
}

// Read realiza uma leitura do sensor e retorna os dados de medição
func (s *Sensor) Read() (Measurement, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.device == nil {
		return Measurement{}, errors.New("sensor not initialized")
	}

	var env physic.Env
	if err := s.device.Sense(&env); err != nil {
		return Measurement{}, fmt.Errorf("failed to read sensor data: %w", err)
	}

	// Converte os valores para as unidades desejadas
	measurement := Measurement{
		Temperature: float64(env.Temperature) / float64(physic.Celsius), // Celsius
		Humidity:    float64(env.Humidity) / float64(physic.PercentRH),  // Porcentagem
		Pressure:    int64(env.Pressure) / int64(physic.Pascal),         // Pascal
	}

	return measurement, nil
}

// Name returns the sensor type name
func (s *Sensor) Name() string {
	return "BME280"
}

// Close fecha a conexão com o sensor e libera os recursos
func (s *Sensor) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var errs []error

	if s.device != nil {
		if err := s.device.Halt(); err != nil {
			errs = append(errs, fmt.Errorf("failed to halt device: %w", err))
		}
		s.device = nil
	}

	if s.bus != nil {
		if err := s.bus.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close I2C bus: %w", err))
		}
		s.bus = nil
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors during close: %v", errs)
	}

	return nil
}

// String retorna uma representação em string da medição
func (m *Measurement) String() string {
	return fmt.Sprintf("Temperature: %.2f°C, Humidity: %.2f%%, Pressure: %d Pa",
		m.Temperature, m.Humidity, m.Pressure)
}

// Provider defines the interface for BME280 measurement providers
// This allows for both hardware and simulated implementations
type Provider interface {
	Close() error
}

// Reader defines the interface for reading measurements
// Hardware sensor returns pointer, simulated returns value
type Reader interface {
	Read() (Measurement, error)
	Name() string
}

// SimulatedSensor provides simulated BME280 sensor data for testing and development
type SimulatedSensor struct {
	config *SimulatedConfig
	rand   *rand.Rand
	mu     sync.RWMutex
	closed bool
}

// SimulatedConfig holds configuration for the simulated sensor
type SimulatedConfig struct {
	// Temperature range in Celsius
	MinTemp float64
	MaxTemp float64
	// Humidity range in percentage
	MinHumidity float64
	MaxHumidity float64
	// Pressure range in Pascal
	MinPressure int64
	MaxPressure int64
}

// DefaultSimulatedConfig returns sensible defaults for simulated sensor
func DefaultSimulatedConfig() *SimulatedConfig {
	return &SimulatedConfig{
		MinTemp:     15.0,   // Minimum temperature in Celsius
		MaxTemp:     35.0,   // Maximum temperature in Celsius
		MinHumidity: 30.0,   // Minimum humidity percentage
		MaxHumidity: 80.0,   // Maximum humidity percentage
		MinPressure: 98000,  // Minimum pressure in Pa
		MaxPressure: 102000, // Maximum pressure in Pa
	}
}

// NewSimulatedSensor creates a new simulated BME280 sensor
func NewSimulatedSensor(config *SimulatedConfig) *SimulatedSensor {
	if config == nil {
		config = DefaultSimulatedConfig()
	}

	// Always use time-based seed for randomness
	seed := time.Now().UnixNano()

	return &SimulatedSensor{
		config: config,
		rand:   rand.New(rand.NewSource(seed)),
	}
}

// newSimulatedSensorWithSeed creates a sensor with fixed seed (for testing)
func newSimulatedSensorWithSeed(config *SimulatedConfig, seed int64) *SimulatedSensor {
	if config == nil {
		config = DefaultSimulatedConfig()
	}

	return &SimulatedSensor{
		config: config,
		rand:   rand.New(rand.NewSource(seed)),
	}
}

// Read implements Provider interface with simulated data
func (s *SimulatedSensor) Read() (Measurement, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return Measurement{}, errors.New("simulated sensor is closed")
	}

	// Generate realistic readings with some variation
	tempRange := s.config.MaxTemp - s.config.MinTemp
	temp := s.config.MinTemp + s.rand.Float64()*tempRange

	humidityRange := s.config.MaxHumidity - s.config.MinHumidity
	humidity := s.config.MinHumidity + s.rand.Float64()*humidityRange

	pressureRange := float64(s.config.MaxPressure - s.config.MinPressure)
	pressure := s.config.MinPressure + int64(s.rand.Float64()*pressureRange)

	return Measurement{
		Temperature: temp,
		Humidity:    humidity,
		Pressure:    pressure,
	}, nil
}

// Name returns the sensor type name
func (s *SimulatedSensor) Name() string {
	return "Simulated"
}

// Close implements Provider interface
func (s *SimulatedSensor) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true
	return nil
}

// Ensure both implementations satisfy their respective interfaces
var (
	_ Provider = (*Sensor)(nil)
	_ Provider = (*SimulatedSensor)(nil)
	_ Reader   = (*Sensor)(nil)
	_ Reader   = (*SimulatedSensor)(nil)
)
