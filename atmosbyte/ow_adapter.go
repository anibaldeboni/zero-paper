package main

import (
	"context"

	"github.com/anibaldeboni/zero-paper/atmosbyte/bme280"
	"github.com/anibaldeboni/zero-paper/atmosbyte/openweather"
	"github.com/anibaldeboni/zero-paper/atmosbyte/queue"
)

// OpenWeatherWorkerAdapter adapta o OpenWeatherWorker para a interface queue.Worker[bme280.Measurement]
type OpenWeatherWorkerAdapter struct {
	worker *openweather.OpenWeatherWorker
}

// NewOpenWeatherWorkerAdapter cria um novo adapter
func NewOpenWeatherWorkerAdapter(worker *openweather.OpenWeatherWorker) *OpenWeatherWorkerAdapter {
	return &OpenWeatherWorkerAdapter{
		worker: worker,
	}
}

// Process implementa a interface queue.Worker[bme280.Measurement]
func (a *OpenWeatherWorkerAdapter) Process(ctx context.Context, msg queue.Message[bme280.Measurement]) error {
	// Converte bme280.Measurement para openweather.QueueMessage
	queueMsg := openweather.QueueMessage{
		ID: msg.ID,
		Data: openweather.QueueMeasurementData{
			Temperature: msg.Data.Temperature,
			Humidity:    msg.Data.Humidity,
			Pressure:    msg.Data.Pressure,
		},
		Attempts:  msg.Attempts,
		MaxTries:  msg.MaxTries,
		CreatedAt: msg.CreatedAt,
		LastTry:   msg.LastTry,
	}

	return a.worker.Process(ctx, queueMsg)
}
