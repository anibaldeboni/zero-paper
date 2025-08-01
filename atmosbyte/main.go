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

// SensorSetup encapsula os componentes do sensor
type SensorSetup struct {
	dev     bme280.Reader
	reader  *SensorReader
	cleanup func() error // função para cleanup (close do sensor se necessário)
}

// createSensorSetup cria o sensor e seus componentes baseado na configuração
func createSensorSetup(cfg *config.AppConfig, q *queue.Queue[bme280.Measurement]) (*SensorSetup, error) {
	var sensor bme280.Reader
	var cleanup func() error

	switch cfg.Sensor.Type {
	case "simulated":
		log.Println("Using simulated sensor data")
		simSensor := bme280.NewSimulatedSensor(cfg.SimulatedConfig())
		sensor = simSensor
		cleanup = simSensor.Close

	default: // hardware BME280
		log.Println("Attempting to use BME280 hardware sensor")
		hwSensor, err := bme280.NewSensor(cfg.BME280Config())
		if err != nil {
			return nil, fmt.Errorf("failed to initialize BME280 sensor: %w", err)
		}
		log.Println("BME280 sensor initialized successfully")
		sensor = hwSensor
		cleanup = hwSensor.Close
	}

	sensorReader := NewSensorReader(sensor, q, cfg.Sensor.ReadInterval)

	return &SensorSetup{
		dev:     sensor,
		reader:  sensorReader,
		cleanup: cleanup,
	}, nil
}

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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	worker := NewOpenWeatherWorker(client, stationID)

	q := queue.NewQueue(ctx, worker, cfg.QueueConfig())

	sensor, err := createSensorSetup(cfg, q)
	if err != nil {
		log.Fatalf("Failed to setup sensor: %v", err)
	}
	defer func() {
		if err := sensor.cleanup(); err != nil {
			log.Printf("Error during sensor cleanup: %v", err)
		}
	}()

	webServer := web.NewServer(ctx, sensor.dev, cfg.WebConfig(), q)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := q.Start(); err != nil && err != context.Canceled {
			log.Printf("Queue error: %v", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := sensor.reader.Start(ctx); err != nil && err != context.Canceled {
			log.Printf("Sensor worker error: %v", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := webServer.Start(); err != nil && err != context.Canceled {
			log.Printf("Web server error: %v", err)
		}
	}()

	// Wait for shutdown
	<-sigChan
	log.Println("Shutdown signal received")
	// Cancel the main context to signal all components to stop
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
