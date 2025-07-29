package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/anibaldeboni/zero-paper/atmosbyte/bme280"
)

// SensorWorker é responsável por ler dados de qualquer sensor e enviá-los para a fila
type SensorWorker struct {
	sensor   bme280.Reader
	queue    *MeasurementQueue
	interval time.Duration
	stopCh   chan struct{}
	name     string // nome do sensor para logs
}

// NewSensorWorker cria um novo worker genérico de sensor
func NewSensorWorker(sensor bme280.Reader, queue *MeasurementQueue, interval time.Duration, name string) *SensorWorker {
	return &SensorWorker{
		sensor:   sensor,
		queue:    queue,
		interval: interval,
		stopCh:   make(chan struct{}),
		name:     name,
	}
}

// Start inicia a leitura contínua do sensor
func (w *SensorWorker) Start(ctx context.Context) error {
	log.Printf("Starting %s sensor worker (reading every %v)", w.name, w.interval)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("%s sensor worker stopped by context", w.name)
			return ctx.Err()

		case <-w.stopCh:
			log.Printf("%s sensor worker stopped", w.name)
			return nil

		case <-ticker.C:
			if err := w.readAndEnqueue(); err != nil {
				log.Printf("Error reading from %s sensor: %v", w.name, err)
			}
		}
	}
}

// Stop para o worker do sensor
func (w *SensorWorker) Stop() {
	close(w.stopCh)
}

// readAndEnqueue lê uma medição do sensor e a envia para a fila
func (w *SensorWorker) readAndEnqueue() error {
	measurement, err := w.sensor.Read()
	if err != nil {
		return fmt.Errorf("failed to read from %s sensor: %w", w.name, err)
	}

	if err := w.queue.Enqueue(measurement); err != nil {
		return fmt.Errorf("failed to enqueue %s measurement: %w", w.name, err)
	}

	log.Printf("%s reading enqueued: temp=%.1f°C, humidity=%.1f%%, pressure=%d Pa",
		w.name, measurement.Temperature, measurement.Humidity, measurement.Pressure)

	return nil
}
