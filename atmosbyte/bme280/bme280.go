package bme280

import (
	"errors"
	"fmt"
	"log"
	"sync"

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

// DefaultConfig retorna uma configuração padrão para o sensor
func DefaultConfig() *Config {
	return &Config{
		Address: 0x76,
		BusName: "",
		Options: &bmxx80.DefaultOpts,
	}
}

// NewSensor cria uma nova instância do sensor BME280
func NewSensor(config *Config) (*Sensor, error) {
	if config == nil {
		config = DefaultConfig()
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
func (s *Sensor) Read() (*Measurement, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.device == nil {
		return nil, errors.New("sensor not initialized")
	}

	var env physic.Env
	if err := s.device.Sense(&env); err != nil {
		return nil, fmt.Errorf("failed to read sensor data: %w", err)
	}

	// Converte os valores para as unidades desejadas
	measurement := &Measurement{
		Temperature: float64(env.Temperature) / float64(physic.Celsius), // Celsius
		Humidity:    float64(env.Humidity) / float64(physic.PercentRH),  // Porcentagem
		Pressure:    int64(env.Pressure) / int64(physic.Pascal),         // Pascal
	}

	return measurement, nil
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

// ExampleUsage demonstra como usar o package bme280
func ExampleUsage() {
	// Cria sensor com configuração padrão
	sensor, err := NewSensor(nil)
	if err != nil {
		log.Printf("Failed to create sensor: %v", err)
		return
	}
	defer sensor.Close()

	// Realiza uma leitura
	measurement, err := sensor.Read()
	if err != nil {
		log.Printf("Failed to read sensor: %v", err)
		return
	}

	// Exibe os resultados
	fmt.Printf("Sensor reading: %s\n", measurement)
	fmt.Printf("Temperature: %.2f°C\n", measurement.Temperature)
	fmt.Printf("Humidity: %.2f%%\n", measurement.Humidity)
	fmt.Printf("Pressure: %d Pa (%.2f hPa)\n", measurement.Pressure, float64(measurement.Pressure)/100.0)
}
