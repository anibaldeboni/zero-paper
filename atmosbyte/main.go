package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/anibaldeboni/zero-paper/atmosbyte/openweather"
	"github.com/anibaldeboni/zero-paper/atmosbyte/queue"
)

func main() {
	// Configuração da API OpenWeather
	appID := os.Getenv("OPENWEATHER_API_KEY")
	if appID == "" {
		log.Fatal("OPENWEATHER_API_KEY environment variable is required")
	}

	stationID := os.Getenv("STATION_ID")
	if stationID == "" {
		log.Fatal("STATION_ID environment variable is required")
	}

	// Cria o cliente OpenWeather
	client, err := openweather.NewOpenWeatherClient(appID)
	if err != nil {
		log.Fatal("Failed to create OpenWeather client:", err)
	}

	// Cria o worker
	owWorker := openweather.NewOpenWeatherWorker(client, stationID)
	worker := NewOpenWeatherWorkerAdapter(owWorker)

	// Configuração da fila
	config := queue.DefaultQueueConfig()
	config.Workers = 2
	config.BufferSize = 50

	// Cria a fila usando o helper para Measurement
	q := queue.NewMeasurementQueue(worker, config)

	// Configura graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Simula o envio de algumas medições
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				measurement := queue.Measurement{
					Temperature: 25.5 + float64(time.Now().Second()%10), // Varia entre 25.5 e 34.5
					Humidity:    60.0 + float64(time.Now().Second()%30), // Varia entre 60 e 89
					Pressure:    1013 + int64(time.Now().Second()%20),   // Varia entre 1013 e 1032
				}

				if err := q.Enqueue(measurement); err != nil {
					log.Printf("Failed to enqueue measurement: %v", err)
				} else {
					log.Printf("Enqueued measurement: temp=%.1f, humidity=%.1f, pressure=%d",
						measurement.Temperature, measurement.Humidity, measurement.Pressure)
				}

			case <-sigChan:
				return
			}
		}
	}()

	// Monitora estatísticas da fila
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				stats := q.Stats()
				log.Printf("Queue stats - Size: %d, Retry: %d, CircuitBreaker: %v, Workers: %d",
					stats.QueueSize, stats.RetryQueueSize, stats.CircuitBreakerState, stats.Workers)

			case <-sigChan:
				return
			}
		}
	}()

	// Aguarda sinal de shutdown
	<-sigChan
	log.Println("Shutting down...")

	// Fecha a fila graciosamente
	if err := q.Close(); err != nil {
		log.Printf("Error closing queue: %v", err)
	}

	log.Println("Shutdown completed")
}
