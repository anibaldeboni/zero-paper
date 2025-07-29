package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/anibaldeboni/zero-paper/atmosbyte/bme280"
	"github.com/anibaldeboni/zero-paper/atmosbyte/openweather"
	"github.com/anibaldeboni/zero-paper/atmosbyte/queue"
	"github.com/anibaldeboni/zero-paper/atmosbyte/web"
)

// BuildInfo contém informações de versão da aplicação
type BuildInfo struct {
	Version   string
	Commit    string
	Date      string
	GoVersion string
	Module    string
}

// GetBuildInfo extrai informações de build usando debug.BuildInfo
func GetBuildInfo() BuildInfo {
	info := BuildInfo{
		Version:   "dev",
		Commit:    "unknown",
		Date:      "unknown",
		GoVersion: "unknown",
		Module:    "unknown",
	}

	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		info.GoVersion = buildInfo.GoVersion
		info.Module = buildInfo.Main.Path

		// Se a versão do módulo principal estiver disponível
		if buildInfo.Main.Version != "(devel)" && buildInfo.Main.Version != "" {
			info.Version = buildInfo.Main.Version
		}

		// Extrai informações de VCS se disponíveis
		for _, setting := range buildInfo.Settings {
			switch setting.Key {
			case "vcs.revision":
				if len(setting.Value) >= 7 {
					info.Commit = setting.Value[:7] // Short commit hash
				} else {
					info.Commit = setting.Value
				}
			case "vcs.time":
				info.Date = setting.Value
			}
		}
	}

	return info
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
	// Adiciona flag para mostrar versão
	var showVersion = flag.Bool("version", false, "Show version information")
	var showVersionShort = flag.Bool("v", false, "Show version information (short)")
	flag.Parse()

	if *showVersion || *showVersionShort {
		PrintVersion()
		return
	}

	buildInfo := GetBuildInfo()
	log.Printf("Starting Atmosbyte %s %s %s", buildInfo.Version, buildInfo.Date, buildInfo.GoVersion)

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

	// Cria o worker OpenWeather diretamente
	worker := NewOpenWeatherWorker(client, stationID)

	// Configuração da fila
	config := queue.DefaultQueueConfig()
	config.Workers = 2
	config.BufferSize = 50
	config.RetryPolicy.MaxRetries = 3
	config.RetryPolicy.BaseDelay = 5 * time.Second

	// Configura graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cria a fila para processar measurements (agora com contexto)
	q := NewMeasurementQueue(ctx, worker, config)

	// Cria o worker do sensor
	var (
		sensorReader        *SensorReader
		environmentalSensor bme280.Reader
	)

	if useSimulated {
		log.Println("Using simulated sensor data")
		simSensor := bme280.NewSimulatedSensor(nil)
		sensorReader = NewSensorReader(simSensor, q, 10*time.Second, "Simulated")
		// Usa o mesmo sensor simulado para o web server
		environmentalSensor = simSensor
	} else {
		log.Println("Attempting to use BME280 hardware sensor")
		sensor, err := bme280.NewSensor(bme280.DefaultConfig())
		if err != nil {
			log.Fatalf("Failed to initialize BME280 sensor: %v", err)
		} else {
			log.Println("BME280 sensor initialized successfully")
			sensorReader = NewSensorReader(sensor, q, time.Minute, "BME280")
			environmentalSensor = sensor
			defer sensor.Close()
		}
	}

	webServer := web.NewServer(environmentalSensor, nil, q)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Inicia o sensor em uma goroutine
	go func() {
		if err := sensorReader.Start(ctx); err != nil && err != context.Canceled {
			log.Printf("Sensor worker error: %v", err)
		}
	}()

	// Inicia o servidor web em uma goroutine
	go func() {
		if err := webServer.Start(); err != nil {
			log.Printf("Web server error: %v", err)
		}
	}()

	// Aguarda sinal de shutdown
	<-sigChan
	log.Println("Shutdown signal received...")

	// Para o sensor worker
	sensorReader.Stop()

	// Para o servidor web graciosamente
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := webServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("Error shutting down web server: %v", err)
	}

	cancel() // cancela o context para parar todas as goroutines

	// Aguarda um pouco para processar mensagens pendentes
	log.Println("Waiting for pending messages...")
	time.Sleep(5 * time.Second)

	// Fecha a fila graciosamente
	log.Println("Closing queue...")
	if err := q.Close(); err != nil {
		log.Printf("Error closing queue: %v", err)
	}

	log.Println("Shutdown completed")
}
