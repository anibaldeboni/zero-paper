package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/anibaldeboni/zero-paper/atmosbyte/bme280"
	"github.com/anibaldeboni/zero-paper/atmosbyte/config"
	"github.com/anibaldeboni/zero-paper/atmosbyte/openweather"
	"github.com/anibaldeboni/zero-paper/atmosbyte/queue"
	"github.com/anibaldeboni/zero-paper/atmosbyte/web"
)

// PrintVersion exibe informações de versão formatadas
func PrintVersion() {
	buildInfo := GetBuildInfo()
	fmt.Printf("Atmosbyte Weather Data Processing System\n")
	fmt.Printf("Version: %s\n", buildInfo.Version)
	fmt.Printf("Commit: %s\n", buildInfo.Commit)
	fmt.Printf("Build Date: %s\n", buildInfo.Date)
	fmt.Printf("Go Version: %s\n", buildInfo.GoVersion)
	fmt.Printf("Module: %s\n", buildInfo.Module)
}

func main() {
	// Adiciona flags para configuração
	var showVersion = flag.Bool("version", false, "Show version information")
	var showVersionShort = flag.Bool("v", false, "Show version information (short)")
	var configPath = flag.String("config", "", "Path to configuration file")
	var generateConfig = flag.Bool("generate-config", false, "Generate example configuration file")
	flag.Parse()

	if *showVersion || *showVersionShort {
		PrintVersion()
		return
	}

	if *generateConfig {
		if err := config.GenerateExampleConfig("atmosbyte.yaml.example"); err != nil {
			log.Fatalf("Failed to generate config file: %v", err)
		}
		fmt.Println("Example configuration file generated: atmosbyte.yaml.example")
		return
	}

	// Carrega configuração
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Printf("Warning: Failed to load config file, using defaults: %v", err)
		cfg, _ = config.Load("") // Force defaults
	}

	buildInfo := GetBuildInfo()
	log.Printf("Starting Atmosbyte %s %s %s", buildInfo.Version, buildInfo.Date, buildInfo.GoVersion)
	log.Printf("Configuration loaded successfully")

	// Configuração da API OpenWeather
	appID := os.Getenv("OPENWEATHER_API_KEY")
	if appID == "" {
		log.Fatal("OPENWEATHER_API_KEY environment variable is required")
	}

	stationID := os.Getenv("STATION_ID")
	if stationID == "" {
		log.Fatal("STATION_ID environment variable is required")
	}

	// Cria o cliente OpenWeather com timeout configurado
	client, err := openweather.NewOpenWeatherClient(appID, openweather.WithTimeout(cfg.OpenWeather.Timeout))
	if err != nil {
		log.Fatal("Failed to create OpenWeather client:", err)
	}

	// Cria o worker OpenWeather
	worker := NewOpenWeatherWorker(client, stationID)

	// Configuração da fila usando config
	queueConfig := cfg.QueueConfig()

	// Configura graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cria a fila para processar measurements (sem iniciar ainda)
	q := queue.NewQueue(ctx, worker, queueConfig)

	// Cria o worker do sensor baseado na configuração
	var (
		sensorReader *SensorReader
		bme280Sensor bme280.Reader
	)

	if cfg.Sensor.Type == "simulated" {
		log.Println("Using simulated sensor data")
		simConfig := cfg.SimulatedConfig()
		simSensor := bme280.NewSimulatedSensor(simConfig)
		sensorReader = NewSensorReader(simSensor, q, cfg.Sensor.ReadInterval)
		bme280Sensor = simSensor
	} else {
		log.Println("Attempting to use BME280 hardware sensor")
		sensorConfig := cfg.BME280Config()
		sensor, err := bme280.NewSensor(sensorConfig)
		if err != nil {
			log.Fatalf("Failed to initialize BME280 sensor: %v", err)
		} else {
			log.Println("BME280 sensor initialized successfully")
			sensorReader = NewSensorReader(sensor, q, cfg.Sensor.ReadInterval)
			bme280Sensor = sensor
			defer sensor.Close()
		}
	}

	// Cria servidor web com configuração
	webConfig := cfg.WebConfig()
	webServer := web.NewServer(ctx, bme280Sensor, webConfig, q)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// WaitGroup para aguardar todos os components terminarem
	var wg sync.WaitGroup

	// Inicia a queue em uma goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := q.Start(); err != nil && err != context.Canceled {
			log.Printf("Queue error: %v", err)
		}
	}()

	// Inicia o sensor em uma goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := sensorReader.Start(ctx); err != nil && err != context.Canceled {
			log.Printf("Sensor worker error: %v", err)
		}
	}()

	// Inicia o servidor web em uma goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := webServer.Start(); err != nil && err != context.Canceled {
			log.Printf("Web server error: %v", err)
		}
	}()

	// Aguarda sinal de shutdown
	<-sigChan
	log.Println("Shutdown signal received")

	// Cancela o contexto principal (para sensor worker, queue e web server)
	cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		wg.Wait()
		log.Println("All components shutdown completed")
	}()

	select {
	case <-done:
		log.Println("All components shutdown successfully")
	case <-time.After(cfg.Timeouts.ShutdownTimeout):
		log.Println("Shutdown timeout reached, forcing exit")
	}

	log.Println("Shutdown completed")
}
