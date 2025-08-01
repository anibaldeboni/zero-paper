# Atmosbyte - Weather Data Processing System

A comprehensive weather data collection, processing, and visualization system built in Go, featuring centralized YAML configuration, BME280 sensor integration, generic queue system with retry/circuit breaker patterns, OpenWeather API client, and a real-time web interface.

## 🎯 Key Features

### **Centralized Configuration System**

- **YAML Configuration**: Flexible, file-based configuration with sensible defaults
- **Thread-Safe**: Singleton pattern with concurrent access protection
- **Hot Configuration**: Change settings without recompilation
- **Multiple Environments**: Support for development, staging, and production configs
- **Auto-Discovery**: Automatic config file location detection

### **Modular Architecture**

- **Config Package**: Centralized configuration management with type-safe adapters
- **BME280 Package**: Clean hardware sensor abstraction with I2C communication
- **Queue Package**: Generic queue system with retry logic and circuit breaker
- **OpenWeather Package**: HTTP client for OpenWeather API integration
- **Web Package**: Real-time web interface with configurable endpoints and timeouts
- **Central Coordination**: `main.go` orchestrates all components with clean separation

### **Flexible Data Sources**

- **Real Hardware**: BME280 sensor via I2C for temperature, humidity, and pressure
- **Simulated Sensor**: Realistic data generation when hardware is unavailable
- **Automatic Fallback**: Detects hardware and gracefully switches to simulation

### **Robust Queue System**

- **Smart Retry**: `RetryableError` interface for custom retry logic
- **Circuit Breaker**: Protection against external API failures
- **Parallel Processing**: Configurable worker pools
- **Graceful Shutdown**: Clean termination with pending message processing

### **Real-time Web Interface**

- **Live Dashboard**: HTML interface with automatic data refresh
- **REST API**: JSON endpoints for programmatic access
- **Responsive Design**: Mobile-friendly interface with modern styling
- **Status Monitoring**: Real-time system health and sensor status

## 📁 Project Structure

```
atmosbyte/
├── config/                   # Centralized configuration system
│   ├── config.go            # Core configuration loading and defaults
│   ├── adapters.go          # Type-safe configuration adapters
│   ├── testing.go           # Configuration helpers for tests
│   └── config_test.go       # Configuration system tests
├── bme280/                   # BME280 sensor package
│   ├── bme280.go            # Core sensor implementation with I2C communication
│   └── bme280_test.go       # Comprehensive unit tests and benchmarks
├── queue/                    # Generic queue system with retry logic
│   ├── queue.go             # Core queue implementation with circuit breaker
│   └── queue_test.go        # Comprehensive test suite with retry scenarios
├── openweather/              # OpenWeather API client package
│   └── openweather.go       # HTTP client for weather station API
├── web/                      # Real-time web interface package
│   ├── server.go            # HTTP server and configuration
│   ├── handlers.go          # API endpoint handlers
│   ├── responses.go         # Response type definitions
│   ├── types.go             # Web-specific type definitions
│   ├── config.go            # Web server configuration
│   ├── middleware.go        # HTTP middleware components
│   ├── web_test.go          # HTTP handler tests and API validation
│   └── templates/           # HTML templates for web interface
│       └── index.html       # Main dashboard with live data and queue status
├── main.go                   # Application coordinator and OpenWeather worker
├── sensor_reader.go          # Sensor data collection workers (BME280/simulated)
├── queue_worker.go           # Queue worker implementation for measurement processing
├── atmosbyte.yaml.example    # Example configuration file with all options
├── test-config.yaml          # Test configuration for automated testing
├── go.mod                    # Go module definition and dependencies
├── go.sum                    # Dependency checksums
├── .env.example              # Environment variables template
└── README.md                 # This comprehensive documentation
```

## 🚀 Quick Start

### 1. Installation

```bash
# Clone the repository
git clone https://github.com/anibaldeboni/zero-paper.git
cd zero-paper/atmosbyte

# Install BME280 hardware dependencies (for real hardware)
go get periph.io/x/conn/v3/...
go get periph.io/x/devices/v3/...
go get periph.io/x/host/v3

# Build the project
go build .
```

### 2. Configuration Setup

#### Generate Example Configuration

```bash
# Generate a complete configuration example
./atmosbyte --generate-config
```

This creates an `atmosbyte.yaml.example` file with all available parameters and their default values.

#### Create Your Configuration

```bash
# Copy and customize the example configuration
cp atmosbyte.yaml.example atmosbyte.yaml

# Edit configuration for your environment
vim atmosbyte.yaml
```

#### Configuration File Locations

The application searches for configuration files in the following order:

1. Path specified via `--config` flag
2. `atmosbyte.yaml` (current directory)
3. `atmosbyte.yml` (current directory)
4. `config/atmosbyte.yaml`
5. `config/atmosbyte.yml`
6. `/etc/atmosbyte/atmosbyte.yaml`
7. `/etc/atmosbyte.yaml`

### 3. Environment Variables

```bash
# Copy example configuration
cp .env.example .env

# Set your credentials
export OPENWEATHER_API_KEY="your_api_key"
export STATION_ID="your_weather_station_id"

# Use simulated sensor for development (default)
export USE_SIMULATED_SENSOR="true"

# Use real BME280 hardware (Raspberry Pi)
export USE_SIMULATED_SENSOR="false"
```

### 4. Running the System

```bash
# Development mode with custom config
./atmosbyte --config=dev-config.yaml

# Production mode with default config discovery
./atmosbyte

# Using environment override for sensor type
USE_SIMULATED_SENSOR=true ./atmosbyte

# Development with Go run
go run . --config=atmosbyte.yaml
```

### 5. Access the Web Interface

Once running, open your browser to:

- **Web Dashboard**: http://localhost:8080
- **API Endpoints**:
  - http://localhost:8080/measurements (JSON)
  - http://localhost:8080/health (JSON)
  - http://localhost:8080/queue (JSON)

## 💻 Usage Examples

### Configuration-Based Usage

```bash
# Generate example configuration
$ ./atmosbyte --generate-config
Configuration example written to atmosbyte.yaml.example

# Development with simulated sensor
$ ./atmosbyte --config=dev-config.yaml
2025/08/01 12:00:00 Loading configuration from dev-config.yaml
2025/08/01 12:00:00 Starting Atmosbyte - Weather Data Processing System
2025/08/01 12:00:00 Using simulated sensor data
2025/08/01 12:00:00 🌐 Starting HTTP server on :8080
2025/08/01 12:00:00 Starting simulated sensor worker (generating data every 5s)
```

### Simulated Sensor (Development)

**Configuration (dev-config.yaml):**

```yaml
sensor:
  type: "simulated"
  read_interval: 10s
  simulation:
    min_temperature: 20.0
    max_temperature: 30.0
```

**Output:**

```bash
$ ./atmosbyte --config=dev-config.yaml
2025/08/01 12:00:00 Starting Atmosbyte - Weather Data Processing System
2025/08/01 12:00:00 Using simulated sensor data
2025/08/01 12:00:00 🌐 Starting HTTP server on :8080
2025/08/01 12:00:10 Simulated reading enqueued: temp=27.5°C, humidity=62.3%, pressure=101250 Pa
```

### BME280 Hardware (Production)

**Configuration (prod-config.yaml):**

```yaml
sensor:
  type: "hardware"
  read_interval: 60s
  bme280:
    i2c_address: 0x76
    i2c_bus: ""
```

**Output:**

```bash
$ ./atmosbyte --config=prod-config.yaml
2025/08/01 12:00:00 Loading configuration from prod-config.yaml
2025/08/01 12:00:00 Starting Atmosbyte - Weather Data Processing System
2025/08/01 12:00:00 Attempting to use BME280 hardware sensor
2025/08/01 12:00:00 BME280 sensor initialized successfully
2025/08/01 12:00:00 🌐 Starting HTTP server on :8080
2025/08/01 12:00:00 Starting BME280 sensor worker (reading every 1m)
```

### API Usage

```bash
# Get current measurements
curl http://localhost:8080/measurements

# Check system health
curl http://localhost:8080/health

# Get queue processing status
curl http://localhost:8080/queue
```

### Command Line Options

```bash
# Display version information
./atmosbyte --version

# Generate example configuration file
./atmosbyte --generate-config

# Use specific configuration file
./atmosbyte --config=my-config.yaml

# Use default configuration (searches standard locations)
./atmosbyte

# Development with Go run
go run . --config=dev-config.yaml
```

## 🌐 Web Interface Features

### **Real-time Dashboard**

- **Live Data Display**: Temperature, humidity, and pressure with auto-refresh
- **Sensor Status**: Visual indicators for hardware/simulated sensor status
- **Queue Monitoring**: Real-time queue status with circuit breaker state
- **System Monitoring**: Real-time system health and last update timestamps
- **Responsive Design**: Works on desktop, tablet, and mobile devices

### **REST API Endpoints**

| Endpoint        | Method | Description                            | Response Format |
| --------------- | ------ | -------------------------------------- | --------------- |
| `/`             | GET    | Web dashboard (HTML)                   | HTML            |
| `/measurements` | GET    | Current sensor readings                | JSON            |
| `/health`       | GET    | System health status                   | JSON            |
| `/queue`        | GET    | Queue processing status and statistics | JSON            |

### **API Response Examples**

**Measurements Endpoint:**

```json
{
  "timestamp": "2025-07-28T14:30:00Z",
  "temperature": 23.5,
  "humidity": 58.2,
  "pressure": 101325.0,
  "source": "BME280"
}
```

**Health Endpoint:**

```json
{
  "status": "healthy",
  "timestamp": "2025-07-28T14:30:00Z",
  "sensor": "connected"
}
```

**Queue Endpoint:**

```json
{
  "queue_size": 0,
  "retry_queue_size": 0,
  "circuit_breaker_state": 0,
  "workers": 2,
  "timestamp": "2025-07-29T14:30:00Z"
}
```

## 🔧 Configuration System

### YAML Configuration

The Atmosbyte system uses a comprehensive YAML-based configuration system that allows you to customize all aspects of the application without recompilation.

#### Complete Configuration Example

```yaml
# Web server configuration
web:
  port: 8080
  read_timeout: 10s
  write_timeout: 10s
  idle_timeout: 120s

# Queue processing configuration
queue:
  workers: 2
  buffer_size: 120
  retry:
    max_retries: 10
    base_delay: 5s
  circuit_breaker:
    failure_threshold: 5
    timeout: 60s

# Sensor configuration
sensor:
  type: "simulated" # "hardware" or "simulated"
  read_interval: 10s
  bme280:
    i2c_address: 0x76 # 0x76 or 0x77
    i2c_bus: "" # Empty for default
  simulation:
    min_temperature: 15.0
    max_temperature: 35.0
    min_humidity: 30.0
    max_humidity: 80.0
    min_pressure: 98000
    max_pressure: 102000

# OpenWeather API configuration
openweather:
  timeout: 30s

# Timeout configurations
timeouts:
  shutdown_timeout: 10s
  queue_shutdown_timeout: 30s
  web_shutdown_timeout: 30s
  processing_timeout: 5s
```

### Thread-Safe Configuration

The configuration system is **completely thread-safe**:

- ✅ **Singleton Pattern**: Single instance loaded at startup
- ✅ **sync.RWMutex**: Concurrent access protection
- ✅ **Immutable after Load**: Configuration doesn't change after loading
- ✅ **Race Detector Tested**: Tested with `go test -race`

### Configuration Defaults

If no configuration file is found or values are missing, the application uses sensible defaults:

| Parameter                   | Default Value | Description             |
| --------------------------- | ------------- | ----------------------- |
| `web.port`                  | 8080          | HTTP server port        |
| `queue.workers`             | 2             | Number of queue workers |
| `sensor.type`               | "simulated"   | Use simulated sensor    |
| `sensor.read_interval`      | 10s           | Sensor reading interval |
| `timeouts.shutdown_timeout` | 10s           | Graceful shutdown time  |

### Advanced Configuration

### Environment Variables

Environment variables are primarily used for sensitive data and runtime overrides:

| Variable               | Description          | Default | Required | Notes                            |
| ---------------------- | -------------------- | ------- | -------- | -------------------------------- |
| `OPENWEATHER_API_KEY`  | OpenWeather API key  | -       | ✅       | Keep secure, don't commit to git |
| `STATION_ID`           | Weather station ID   | -       | ✅       | Your weather station identifier  |
| `USE_SIMULATED_SENSOR` | Override sensor type | `false` | ❌       | Overrides YAML `sensor.type`     |

**Note**: For most configuration options, prefer YAML files over environment variables for better organization and documentation.

### BME280 Configuration

```yaml
sensor:
  type: "hardware"
  bme280:
    i2c_address: 0x77 # Alternative I2C address
    i2c_bus: "/dev/i2c-1" # Specific I2C bus
```

### Queue Configuration

```yaml
queue:
  workers: 4 # 4 parallel workers
  buffer_size: 100 # 100 message buffer
  retry:
    max_retries: 5 # Maximum 5 retries
    base_delay: 10s # 10 second base delay
  circuit_breaker:
    failure_threshold: 3 # Open after 3 failures
    timeout: 30s # Retry after 30 seconds
```

### Web Server Configuration

```yaml
web:
  port: 8080 # HTTP port
  read_timeout: 10s # Request timeout
  write_timeout: 10s # Response timeout
  idle_timeout: 120s # Keep-alive timeout
timeouts:
  web_shutdown_timeout: 30s # Graceful shutdown time
```

### Development vs Production Configs

**Development (dev-config.yaml):**

```yaml
sensor:
  type: "simulated"
  read_interval: 5s # Faster updates for development
queue:
  workers: 1 # Single worker for easier debugging
```

**Production (prod-config.yaml):**

```yaml
sensor:
  type: "hardware"
  read_interval: 60s # Standard reading interval
queue:
  workers: 4 # More workers for production load
```

## 🧪 Testing

```bash
# Run all tests
go test ./...

# Verbose testing
go test -v ./...

# Package-specific tests
go test -v ./queue
go test -v ./bme280
go test -v ./web

# Benchmarks
go test -bench=. ./...

# Test coverage
go test -cover ./...
```

### Test Coverage

- **BME280 Package**: Unit tests and benchmarks
- **Queue System**: Comprehensive test suite with retry scenarios
- **Web Interface**: HTTP handler tests and API validation
- **Integration Tests**: End-to-end testing with simulated data

## 📊 Monitoring and Observability

### System Logs

The system provides structured logging with emojis for easy visual parsing:

```
2025/07/28 14:30:00 🌐 Starting HTTP server on :8080
2025/07/28 14:30:10 📊 Queue stats - Size: 2, Retry: 0, CircuitBreaker: Closed, Workers: 2
2025/07/28 14:30:15 🌡️ BME280 reading: temp=23.4°C, humidity=58.2%, pressure=101325 Pa
```

### Available Metrics

- **QueueSize**: Messages in the main queue
- **RetryQueueSize**: Messages awaiting retry
- **CircuitBreakerState**: Circuit breaker status (Closed/Open/HalfOpen)
- **Workers**: Number of active workers
- **HTTP Requests**: Web interface access logs

### Web Dashboard Monitoring

- **Real-time Updates**: Automatic refresh every 30 seconds
- **Error Handling**: Visual error messages for API failures
- **Status Indicators**: Color-coded sensor and system status
- **Performance**: Optimized for continuous monitoring

## 🛠️ Development

### Adding New Sensors

1. Implement the `SensorWorker` interface:

```go
type CustomSensorWorker struct {
    // sensor-specific fields
}

func (w *CustomSensorWorker) Start(ctx context.Context) error {
    // continuous reading logic
}

func (w *CustomSensorWorker) Stop() {
    // cleanup resources
}
```

2. Create a web adapter:

```go
type CustomWebAdapter struct {
    sensor *CustomSensor
}

func (a *CustomWebAdapter) Read() (bme280.Measurement, error) {
    // adapt sensor data to web interface format
}
```

3. Register in `main.go`:

```go
customWorker := NewCustomSensorWorker(params)
// Use the worker directly without SensorManager
```

### Adding New Data Destinations

1. Implement the `queue.Worker[bme280.Measurement]` interface:

```go
type CustomDestinationWorker struct{}

func (w *CustomDestinationWorker) Process(ctx context.Context, msg queue.Message[bme280.Measurement]) error {
    // process measurement data
    return nil
}
```

2. Create adapter if needed and register with the queue.

### Extending the Web Interface

1. Add new templates to `web/templates/`
2. Implement new handlers in `web/web.go`
3. Register routes in `setupRoutes()` method
4. Add corresponding tests in `web/web_test.go`

## 🔍 Troubleshooting

### BME280 Sensor Not Found

```
Failed to initialize BME280 sensor: failed to open I2C bus
```

**Solutions:**

1. Enable I2C: `sudo raspi-config`
2. Check physical connections
3. Use `i2cdetect -y 1` to scan for devices
4. System will automatically fallback to simulation

### OpenWeather API Errors

```
Worker 0: Error processing message: HTTP 401: Invalid API key
```

**Solutions:**

1. Verify `OPENWEATHER_API_KEY` is correct
2. Ensure API key has access to the required endpoints
3. Check `STATION_ID` is valid

### Web Interface Not Accessible

```
Failed to start server: listen tcp :8080: bind: address already in use
```

**Solutions:**

1. Check if port 8080 is already in use: `lsof -i :8080`
2. Change port in web configuration
3. Stop conflicting services

### Circuit Breaker Open

```
Queue stats - CircuitBreaker: Open
```

**Solutions:**

1. Wait for automatic recovery timeout
2. Check network connectivity to OpenWeather API
3. Review detailed error logs

## 🏗️ Architecture Details

### Data Flow

```
[BME280/Simulated] → [SensorWorker] → [Queue] → [OpenWeatherWorker] → [OpenWeather API]
                         ↓
                   [Web Interface] ← [Direct Sensor Access] ← [Live Data]
                         ↓
                   [Queue Status API] ← [Queue Monitoring] ← [Queue Stats]
```

### Component Interaction

1. **Sensor Layer**: BME280 or simulated sensor provides real-time measurements
2. **Collection Layer**: SensorWorker collects data at configurable intervals
3. **Queue Layer**: Generic queue system with retry logic and circuit breaker protection
4. **Processing Layer**: OpenWeatherWorker (in main.go) processes queue messages and sends to API
5. **Web Layer**: Real-time interface with direct sensor access and queue monitoring
6. **API Layer**: RESTful endpoints for measurements, health, and queue status

### Design Principles

- ✅ **Single Responsibility**: Each package has one clear purpose
- ✅ **Open/Closed**: Extensible without modifying existing code
- ✅ **Dependency Inversion**: Depends on interfaces, not implementations
- ✅ **Interface Segregation**: Small, focused interfaces
- ✅ **Separation of Concerns**: Clean boundaries between layers

## 📋 Production Deployment Checklist

### Configuration Setup

- [ ] Generate production configuration file
- [ ] Set appropriate log levels for production
- [ ] Configure production-ready timeouts
- [ ] Set correct sensor type and intervals
- [ ] Optimize queue workers for expected load
- [ ] Configure web server ports and timeouts

### Hardware Requirements

- [ ] Raspberry Pi 3B+ or newer with I2C enabled
- [ ] BME280 sensor properly connected
- [ ] Stable power supply
- [ ] Network connectivity

### Software Setup

- [ ] Go 1.18+ installed
- [ ] periph.io dependencies installed
- [ ] Configuration file in place (`/etc/atmosbyte/atmosbyte.yaml`)
- [ ] I2C permissions set correctly
- [ ] Systemd service configured (optional)

### API Configuration

- [ ] Valid OpenWeather API key in environment
- [ ] Correct Station ID configured
- [ ] API rate limits verified
- [ ] Network connectivity tested

### Security Considerations

- [ ] API keys stored securely (environment variables)
- [ ] Configuration files have appropriate permissions
- [ ] Web interface access controls (if needed)
- [ ] Firewall configuration
- [ ] HTTPS setup (for public deployment)

## 🔗 References

- [OpenWeather API Documentation](https://openweathermap.org/api)
- [periph.io - Go Hardware Library](https://periph.io/)
- [BME280 Datasheet](https://www.bosch-sensortec.com/products/environmental-sensors/humidity-sensors-bme280/)
- [Go Templates Documentation](https://pkg.go.dev/html/template)

## 📈 Recent Improvements & Roadmap

### ✅ Recently Implemented

- **Centralized Configuration System**: YAML-based configuration with thread-safe loading
- **Type-Safe Adapters**: Clean conversion between config and package-specific types
- **Removal of Redundant Defaults**: Eliminated duplicate default functions across packages
- **Idiomatic Go Naming**: Refactored method names to follow Go conventions
- **Simplified Configuration**: Removed unused fields (Environment, Seed) for cleaner config
- **Enhanced Testing**: Comprehensive test coverage with configuration helpers

### Near Term

- [ ] **Metrics Integration**: Prometheus/Grafana support
- [ ] **Database Storage**: SQLite for local persistence
- [ ] **Alert System**: Threshold-based notifications
- [ ] **Configuration Validation**: Schema validation for YAML files

### Medium Term

- [ ] **Docker Support**: Containerization for easy deployment
- [ ] **Multiple Sensors**: Support for additional sensor types
- [ ] **Data Export**: CSV/JSON export functionality
- [ ] **Historical Data**: Trends and historical views

### Long Term

- [ ] **Cloud Integration**: AWS/Azure/GCP deployment
- [ ] **Machine Learning**: Predictive analytics
- [ ] **Mobile App**: Native mobile applications
- [ ] **Distributed System**: Multi-node sensor networks

---

**Atmosbyte** provides a robust, extensible foundation for real-time weather data processing with centralized YAML configuration, modern web visualization, and production-ready reliability! 🌡️📊🌐⚙️
