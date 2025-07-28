package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/anibaldeboni/zero-paper/atmosbyte/bme280"
)

// BME280SensorWorker é responsável por ler dados do sensor BME280 e enviá-los para a fila
type BME280SensorWorker struct {
	sensor   *bme280.Sensor
	queue    *MeasurementQueue
	interval time.Duration
	stopCh   chan struct{}
}

// NewBME280SensorWorker cria um novo worker do sensor BME280
func NewBME280SensorWorker(sensor *bme280.Sensor, queue *MeasurementQueue, interval time.Duration) *BME280SensorWorker {
	return &BME280SensorWorker{
		sensor:   sensor,
		queue:    queue,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start inicia a leitura contínua do sensor
func (w *BME280SensorWorker) Start(ctx context.Context) error {
	log.Printf("Starting BME280 sensor worker (reading every %v)", w.interval)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("BME280 sensor worker stopped by context")
			return ctx.Err()

		case <-w.stopCh:
			log.Println("BME280 sensor worker stopped")
			return nil

		case <-ticker.C:
			if err := w.readAndEnqueue(); err != nil {
				log.Printf("Error reading from BME280 sensor: %v", err)
				// Continua tentando mesmo com erro
			}
		}
	}
}

// Stop para o worker do sensor
func (w *BME280SensorWorker) Stop() {
	close(w.stopCh)
}

// readAndEnqueue lê uma medição do sensor e a envia para a fila
func (w *BME280SensorWorker) readAndEnqueue() error {
	measurement, err := w.sensor.Read()
	if err != nil {
		return fmt.Errorf("failed to read from BME280 sensor: %w", err)
	}

	if err := w.queue.Enqueue(*measurement); err != nil {
		return fmt.Errorf("failed to enqueue BME280 measurement: %w", err)
	}

	log.Printf("BME280 reading enqueued: temp=%.1f°C, humidity=%.1f%%, pressure=%d Pa",
		measurement.Temperature, measurement.Humidity, measurement.Pressure)

	return nil
}

// SimulatedSensorWorker simula um sensor BME280 quando hardware não está disponível
type SimulatedSensorWorker struct {
	queue    *MeasurementQueue
	interval time.Duration
	stopCh   chan struct{}
}

// NewSimulatedSensorWorker cria um worker que simula dados de sensor
func NewSimulatedSensorWorker(queue *MeasurementQueue, interval time.Duration) *SimulatedSensorWorker {
	return &SimulatedSensorWorker{
		queue:    queue,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start inicia a simulação de leituras do sensor
func (w *SimulatedSensorWorker) Start(ctx context.Context) error {
	log.Printf("Starting simulated sensor worker (generating data every %v)", w.interval)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Simulated sensor worker stopped by context")
			return ctx.Err()

		case <-w.stopCh:
			log.Println("Simulated sensor worker stopped")
			return nil

		case <-ticker.C:
			if err := w.generateAndEnqueue(); err != nil {
				log.Printf("Error generating simulated measurement: %v", err)
			}
		}
	}
}

// Stop para o worker simulado
func (w *SimulatedSensorWorker) Stop() {
	close(w.stopCh)
}

// generateAndEnqueue gera uma medição simulada e a envia para a fila
func (w *SimulatedSensorWorker) generateAndEnqueue() error {
	// Gera dados realistas variando com o tempo
	now := time.Now()

	// Temperatura: varia entre 20°C e 30°C
	temp := 25.0 + 5.0*float64(now.Second()%10)/10.0

	// Umidade: varia entre 40% e 70%
	humidity := 55.0 + 15.0*float64(now.Minute()%10)/10.0

	// Pressão: varia entre 1000 e 1020 hPa (100000 - 102000 Pa)
	pressure := int64(101000 + (now.Unix() % 2000))

	measurement := bme280.Measurement{
		Temperature: temp,
		Humidity:    humidity,
		Pressure:    pressure,
	}

	if err := w.queue.Enqueue(measurement); err != nil {
		return fmt.Errorf("failed to enqueue simulated measurement: %w", err)
	}

	log.Printf("Simulated reading enqueued: temp=%.1f°C, humidity=%.1f%%, pressure=%d Pa",
		measurement.Temperature, measurement.Humidity, measurement.Pressure)

	return nil
}

// SensorManager gerencia diferentes tipos de sensores
type SensorManager struct {
	worker SensorWorker
}

// SensorWorker interface para diferentes tipos de workers de sensor
type SensorWorker interface {
	Start(ctx context.Context) error
	Stop()
}

// NewSensorManager cria um gerenciador de sensor
func NewSensorManager(worker SensorWorker) *SensorManager {
	return &SensorManager{
		worker: worker,
	}
}

// Start inicia o gerenciamento do sensor
func (sm *SensorManager) Start(ctx context.Context) error {
	return sm.worker.Start(ctx)
}

// Stop para o gerenciamento do sensor
func (sm *SensorManager) Stop() {
	sm.worker.Stop()
}
