package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"periph.io/x/conn/v3/i2c/i2creg"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/devices/v3/ina219"
	"periph.io/x/host/v3"
)

const (
	socketPath    = "/tmp/batmon.sock"
	i2cBus        = "/dev/i2c-1" // Adjust this if necessary
	ina219Address = 0x43         // Default I2C address for INA219
)

type BatteryRead struct {
	Voltage  float64 `json:"voltage"`
	Current  float64 `json:"current"`
	Power    float64 `json:"power"`
	Capacity float64 `json:"capacity"`
}

// setupSocketServer cria e configura o socket Unix
func setupSocketServer() (net.Listener, error) {
	// Remove socket if it already exists
	os.Remove(socketPath)

	// Create a Unix socket listener
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create socket listener: %v", err)
	}

	// Set permissions so other processes can connect
	if err := os.Chmod(socketPath, 0666); err != nil {
		listener.Close()
		return nil, fmt.Errorf("failed to set socket permissions: %v", err)
	}

	return listener, nil
}

// initSensor inicializa e configura o sensor INA219
func initSensor() (*ina219.Dev, error) {
	bus, err := i2creg.Open(i2cBus)
	if err != nil {
		return nil, fmt.Errorf("failed to open I²C: %v", err)
	}

	opts := ina219.DefaultOpts
	opts.Address = ina219Address
	sensor, err := ina219.New(bus, &opts)
	if err != nil {
		bus.Close()
		return nil, fmt.Errorf("failed to initialize sensor: %v", err)
	}

	return sensor, nil
}

// listenForConnections gerencia as conexões de clientes
func listenForConnections(listener net.Listener, done <-chan struct{}) <-chan net.Conn {
	connChan := make(chan net.Conn)

	go func() {
		defer close(connChan)

		// Create a channel for Accept() errors
		errorChan := make(chan error, 1)

		// Start a goroutine to accept connections
		acceptChan := make(chan net.Conn, 1)
		go func() {
			for {
				conn, err := listener.Accept()
				if err != nil {
					errorChan <- err
					return
				}
				acceptChan <- conn
			}
		}()

		for {
			select {
			case <-done:
				// Shutting down - exit the goroutine
				return

			case err := <-errorChan:
				// Handle Accept() error
				if err != nil {
					log.Printf("Error accepting connection: %v", err)
					return
				}

			case conn := <-acceptChan:
				// Process the new connection
				log.Printf("New client connected")
				connChan <- conn
			}
		}
	}()

	return connChan
}

// readBatteryStatus lê os dados do sensor e os converte em uma estrutura BatteryRead
func readBatteryStatus(sensor *ina219.Dev) (BatteryRead, error) {
	p, err := sensor.Sense()
	if err != nil {
		return BatteryRead{}, fmt.Errorf("sensor reading error: %v", err)
	}

	read := BatteryRead{
		Voltage:  float64(p.Voltage) / float64(physic.Volt),
		Current:  float64(p.Current) / float64(physic.Ampere),
		Power:    float64(p.Power) / float64(physic.Watt),
		Capacity: math.Round(((float64(p.Voltage)/float64(physic.Volt)-3)/1.2)*100.0*100) / 100,
	}

	return read, nil
}

// sendToClients envia os dados para todos os clientes conectados
func sendToClients(clients *[]net.Conn, data []byte) {
	for i := 0; i < len(*clients); i++ {
		_, err := (*clients)[i].Write(data)
		if err != nil {
			log.Printf("failed to write to client, removing: %v", err)
			(*clients)[i].Close()
			// Remove this client
			*clients = append((*clients)[:i], (*clients)[i+1:]...)
			i-- // Adjust index after removal
			continue
		}
		log.Printf("Sent data to client: %s", string(data))
	}
}

func main() {
	// Inicializa a biblioteca periph
	if _, err := host.Init(); err != nil {
		log.Fatalf("Cannot init host %v", err)
	}

	done := make(chan struct{})

	listener, err := setupSocketServer()
	if err != nil {
		log.Fatalf("cannot start batmon: %v", err)
	}
	defer func() {
		log.Printf("Closing listener...")
		listener.Close()
	}()
	log.Printf("Socket server listening at %s", socketPath)

	connChan := listenForConnections(listener, done)

	sensor, err := initSensor()
	if err != nil {
		log.Fatalf("Cannot initialize sensor: %v", err)
	}

	// Configura os temporizadores e sinais
	everySecond := time.NewTicker(time.Second).C
	var halt = make(chan os.Signal, 1)
	signal.Notify(halt, syscall.SIGTERM)
	signal.Notify(halt, syscall.SIGINT)

	// Lista de clientes conectados
	var clients []net.Conn

	log.Print("batmon started, reading values...")

	// Loop principal
	for {
		select {
		case newConn, ok := <-connChan:
			if !ok {
				// connChan was closed - connection listener is no longer running
				log.Printf("Connection channel closed")
				continue
			}
			// Add new connection to clients list
			clients = append(clients, newConn)

		case <-everySecond:
			// Lê os dados do sensor
			read, err := readBatteryStatus(sensor)
			if err != nil {
				log.Printf("Sensor reading error: %v", err)
				continue
			}

			// Converte para JSON
			jsonData, err := json.Marshal(read)
			if err != nil {
				log.Printf("failed to marshal data: %v", err)
				continue
			}

			log.Printf("Battery status: %s", string(jsonData))

			// Envia para os clientes
			sendToClients(&clients, jsonData)

		case <-halt:
			log.Print("shutting down...")
			// Sinaliza para outras goroutines que o programa está encerrando
			close(done)

			// Espera um momento para as goroutines finalizarem
			time.Sleep(100 * time.Millisecond)

			// Fecha todas as conexões de clientes
			for _, client := range clients {
				client.Close()
			}

			// Limpar o socket ao sair
			os.Remove(socketPath)
			return
		}
	}
}
