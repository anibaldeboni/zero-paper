package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/anibaldeboni/zero-paper/atmosbyte/bme280"
	"github.com/anibaldeboni/zero-paper/atmosbyte/openweather"
	"github.com/anibaldeboni/zero-paper/atmosbyte/queue"
)

func main() {
	log.Println("Starting Atmosbyte - Weather Data Processing System")

	// Configuração da API OpenWeather
	appID := os.Getenv("OPENWEATHER_API_KEY")
	if appID == "" {
		log.Fatal("OPENWEATHER_API_KEY environment variable is required")
	}

	stationID := os.Getenv("STATION_ID")
	if stationID == "" {
		log.Fatal("STATION_ID environment variable is required")
	}

	// Determina se deve usar sensor real ou simulado
	useSimulated := os.Getenv("USE_SIMULATED_SENSOR") == "true"

	// Cria o cliente OpenWeather
	client, err := openweather.NewOpenWeatherClient(appID)
	if err != nil {
		log.Fatal("Failed to create OpenWeather client:", err)
	}

	// Cria o worker OpenWeather
	owWorker := openweather.NewOpenWeatherWorker(client, stationID)
	worker := NewOpenWeatherWorkerAdapter(owWorker)

	// Configuração da fila
	config := queue.DefaultQueueConfig()
	config.Workers = 2
	config.BufferSize = 50
	config.RetryPolicy.MaxRetries = 3
	config.RetryPolicy.BaseDelay = 5 * time.Second

	// Cria a fila para processar measurements
	q := NewMeasurementQueue(worker, config)

	// Cria o gerenciador de sensor
	var sensorManager *SensorManager

	if useSimulated {
		log.Println("Using simulated sensor data")
		simWorker := NewSimulatedSensorWorker(q, 10*time.Second)
		sensorManager = NewSensorManager(simWorker)
	} else {
		log.Println("Attempting to use BME280 hardware sensor")
		sensor, err := bme280.NewSensor(bme280.DefaultConfig())
		if err != nil {
			log.Fatalf("Failed to initialize BME280 sensor: %v", err)
		} else {
			log.Println("BME280 sensor initialized successfully")
			bmeWorker := NewBME280SensorWorker(sensor, q, time.Minute)
			sensorManager = NewSensorManager(bmeWorker)
			defer sensor.Close()
		}
	}

	// Configura graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Inicia o sensor em uma goroutine
	go func() {
		if err := sensorManager.Start(ctx); err != nil && err != context.Canceled {
			log.Printf("Sensor manager error: %v", err)
		}
	}()

	// Monitora estatísticas da fila
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				stats := q.Stats()
				log.Printf("📊 Queue stats - Size: %d, Retry: %d, CircuitBreaker: %v, Workers: %d",
					stats.QueueSize, stats.RetryQueueSize, stats.CircuitBreakerState, stats.Workers)

			case <-ctx.Done():
				return
			}
		}
	}()

	// Aguarda sinal de shutdown
	<-sigChan
	log.Println("🛑 Shutdown signal received...")

	// Para o sensor manager
	sensorManager.Stop()
	cancel() // cancela o context para parar todas as goroutines

	// Aguarda um pouco para processar mensagens pendentes
	log.Println("⏳ Waiting for pending messages...")
	time.Sleep(5 * time.Second)

	// Fecha a fila graciosamente
	log.Println("🔄 Closing queue...")
	if err := q.Close(); err != nil {
		log.Printf("❌ Error closing queue: %v", err)
	}

	log.Println("✅ Shutdown completed")
}
