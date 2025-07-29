package main

import (
	"context"
	"time"

	"github.com/anibaldeboni/zero-paper/atmosbyte/bme280"
	"github.com/anibaldeboni/zero-paper/atmosbyte/openweather"
	"github.com/anibaldeboni/zero-paper/atmosbyte/queue"
)

// OpenWeatherWorker implementa um worker para processar mensagens usando OpenWeather API
type OpenWeatherWorker struct {
	client    *openweather.OpenWeatherClient
	stationID string
}

// NewOpenWeatherWorker cria um novo worker para OpenWeather
func NewOpenWeatherWorker(client *openweather.OpenWeatherClient, stationID string) *OpenWeatherWorker {
	return &OpenWeatherWorker{
		client:    client,
		stationID: stationID,
	}
}

// Process implementa a interface queue.Worker[bme280.Measurement]
func (w *OpenWeatherWorker) Process(ctx context.Context, msg queue.Message[bme280.Measurement]) error {
	// Converte os dados da fila para o formato Measurement da API
	measurement := openweather.Measurement{
		StationID:   w.stationID,
		Dt:          time.Now().Unix(),
		Temperature: msg.Data.Temperature,
		Pressure:    msg.Data.Pressure,
		Humidity:    msg.Data.Humidity,
		WindSpeed:   0,
		WindGust:    0,
		Rain1h:      0,
		Clouds:      []openweather.Clouds{},
	}

	// Envia a medição para a API
	err := w.client.SendMeasurement(ctx, measurement)
	if err != nil {
		// Se for um erro HTTP, retorna um HTTPError para o sistema de retry
		if httpErr, ok := err.(*openweather.HTTPError); ok {
			return httpErr
		}
		// Para outros tipos de erro, ainda retorna HTTPError se conseguir extrair o código
		return openweather.NewHTTPError(500, err.Error())
	}

	return nil
}
