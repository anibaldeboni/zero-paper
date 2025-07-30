package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/anibaldeboni/zero-paper/atmosbyte/bme280"
)

// SensorReader é responsável por ler dados de qualquer sensor e enviá-los para a fila
type SensorReader struct {
	sensor   bme280.Reader
	queue    *MeasurementQueue
	interval time.Duration
	name     string // nome do sensor para logs
}

// NewSensorReader cria um novo worker genérico de sensor
func NewSensorReader(sensor bme280.Reader, queue *MeasurementQueue, interval time.Duration, name string) *SensorReader {
	return &SensorReader{
		sensor:   sensor,
		queue:    queue,
		interval: interval,
		name:     name,
	}
}

// Start inicia a leitura contínua do sensor
func (w *SensorReader) Start(ctx context.Context) error {
	log.Printf("Starting %s sensor worker (reading every %v)", w.name, w.interval)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("%s sensor worker stopped", w.name)
			return ctx.Err()

		case <-ticker.C:
			if err := w.readAndEnqueue(); err != nil {
				log.Printf("Error reading from %s sensor: %v", w.name, err)
			}
		}
	}
}

// readAndEnqueue lê uma medição do sensor e a envia para a fila
func (w *SensorReader) readAndEnqueue() error {
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
