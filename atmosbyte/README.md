# Atmosbyte - Weather Data Processing System

A comprehensive weather data collection, processing, and visualization system built in Go, integrating BME280 sensor hardware, a generic queue system with retry/circuit breaker patterns, OpenWeather API client, and a real-time web interface.

## 🎯 Key Features

### **Modular Architecture**

- **BME280 Package**: Clean hardware sensor abstraction with I2C communication
- **Queue Package**: Generic queue system with retry logic and circuit breaker
- **OpenWeather Package**: HTTP client for OpenWeather API integration
- **Web Package**: Real-time web interface with live sensor data
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
├── bme280/                    # BME280 sensor package
│   ├── bme280.go             # Core implementation
│   ├── bme280_test.go        # Unit tests
│   └── example/              # Usage examples
├── queue/                     # Generic queue system
│   ├── queue.go              # Core queue implementation
│   └── queue_test.go         # Comprehensive tests
├── openweather/              # OpenWeather API client
│   └── openweather.go        # HTTP client and workers
├── web/                       # Web interface package
│   ├── web.go                # HTTP server and handlers
│   ├── web_test.go           # Web interface tests
│   └── templates/            # HTML templates
│       └── index.html        # Main dashboard template
├── sensor_worker.go          # Sensor data collection workers
├── web_adapters.go           # Adapters for web interface
├── ow_adapter.go             # OpenWeather adapter
├── main.go                   # Main application coordinator
├── go.mod                    # Go module definition
└── .env.example              # Environment configuration example
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

### 2. Configuration

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

### 3. Running the System

```bash
# Development mode (simulated sensor)
USE_SIMULATED_SENSOR=true go run .

# Production mode (real BME280)
USE_SIMULATED_SENSOR=false go run .

# Using the compiled binary
./atmosbyte
```

### 4. Access the Web Interface

Once running, open your browser to:

- **Web Dashboard**: http://localhost:8080
- **API Endpoints**:
  - http://localhost:8080/measurements (JSON)
  - http://localhost:8080/health (JSON)
  - http://localhost:8080/?format=json (API info)

## 💻 Usage Examples

### Simulated Sensor (Development)

```bash
$ USE_SIMULATED_SENSOR=true go run .
2025/07/28 11:45:00 Starting Atmosbyte - Weather Data Processing System
2025/07/28 11:45:00 Using simulated sensor data
2025/07/28 11:45:00 🌐 Starting HTTP server on :8080
2025/07/28 11:45:00 Starting simulated sensor worker (generating data every 10s)
2025/07/28 11:45:10 Simulated reading enqueued: temp=27.5°C, humidity=62.3%, pressure=101250 Pa
```

### BME280 Hardware (Production)

```bash
$ USE_SIMULATED_SENSOR=false go run .
2025/07/28 11:45:00 Starting Atmosbyte - Weather Data Processing System
2025/07/28 11:45:00 Attempting to use BME280 hardware sensor
2025/07/28 11:45:00 BME280 sensor initialized successfully
2025/07/28 11:45:00 🌐 Starting HTTP server on :8080
2025/07/28 11:45:00 Starting BME280 sensor worker (reading every 1m)
```

### API Usage

```bash
# Get current measurements
curl http://localhost:8080/measurements

# Check system health
curl http://localhost:8080/health

# Get API information
curl http://localhost:8080/?format=json
```

## 🌐 Web Interface Features

### **Real-time Dashboard**

- **Live Data Display**: Temperature, humidity, and pressure with auto-refresh
- **Sensor Status**: Visual indicators for hardware/simulated sensor status
- **System Monitoring**: Real-time system health and last update timestamps
- **Responsive Design**: Works on desktop, tablet, and mobile devices

### **REST API Endpoints**

| Endpoint        | Method | Description                             | Response Format |
| --------------- | ------ | --------------------------------------- | --------------- |
| `/`             | GET    | Web dashboard (HTML) or API info (JSON) | HTML/JSON       |
| `/measurements` | GET    | Current sensor readings                 | JSON            |
| `/health`       | GET    | System health status                    | JSON            |

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

## 🔧 Advanced Configuration

### Environment Variables

| Variable               | Description               | Default | Required |
| ---------------------- | ------------------------- | ------- | -------- |
| `OPENWEATHER_API_KEY`  | OpenWeather API key       | -       | ✅       |
| `STATION_ID`           | Weather station ID        | -       | ✅       |
| `USE_SIMULATED_SENSOR` | Use simulated sensor data | `false` | ❌       |

### BME280 Configuration

```go
// Custom sensor configuration
config := &bme280.Config{
    Address: 0x77,           // Alternative I2C address
    BusName: "/dev/i2c-1",   // Specific I2C bus
    Options: customOpts,     // Precision options
}
```

### Queue Configuration

```go
config := queue.DefaultQueueConfig()
config.Workers = 4                        // 4 parallel workers
config.BufferSize = 100                   // 100 message buffer
config.RetryPolicy.MaxRetries = 5         // Maximum 5 retries
config.RetryPolicy.BaseDelay = 10 * time.Second
```

### Web Server Configuration

```go
webConfig := web.DefaultConfig()
webConfig.Port = 8080                     // HTTP port
webConfig.ReadTimeout = 10 * time.Second  // Request timeout
webConfig.WriteTimeout = 10 * time.Second // Response timeout
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
sensorManager := NewSensorManager(customWorker)
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
[BME280/Simulated] → [SensorWorker] → [Queue] → [OpenWeatherWorker] → [API]
                         ↓
                   [Web Interface] ← [WebAdapter] ← [Live Data]
```

### Component Interaction

1. **Sensor Layer**: BME280 or simulated sensor provides measurements
2. **Collection Layer**: SensorWorker collects data at intervals
3. **Queue Layer**: Generic queue with retry/circuit breaker patterns
4. **Processing Layer**: OpenWeatherWorker sends data to external API
5. **Web Layer**: Real-time interface serves live data to users

### Design Principles

- ✅ **Single Responsibility**: Each package has one clear purpose
- ✅ **Open/Closed**: Extensible without modifying existing code
- ✅ **Dependency Inversion**: Depends on interfaces, not implementations
- ✅ **Interface Segregation**: Small, focused interfaces
- ✅ **Separation of Concerns**: Clean boundaries between layers

## 📋 Production Deployment Checklist

### Hardware Requirements

- [ ] Raspberry Pi 3B+ or newer with I2C enabled
- [ ] BME280 sensor properly connected
- [ ] Stable power supply
- [ ] Network connectivity

### Software Setup

- [ ] Go 1.18+ installed
- [ ] periph.io dependencies installed
- [ ] Environment variables configured
- [ ] I2C permissions set correctly
- [ ] Systemd service configured (optional)

### API Configuration

- [ ] Valid OpenWeather API key
- [ ] Correct Station ID
- [ ] API rate limits verified
- [ ] Network connectivity tested

### Security Considerations

- [ ] API keys stored securely
- [ ] Web interface access controls (if needed)
- [ ] Firewall configuration
- [ ] HTTPS setup (for public deployment)

## 🔗 References

- [OpenWeather API Documentation](https://openweathermap.org/api)
- [periph.io - Go Hardware Library](https://periph.io/)
- [BME280 Datasheet](https://www.bosch-sensortec.com/products/environmental-sensors/humidity-sensors-bme280/)
- [Go Templates Documentation](https://pkg.go.dev/html/template)

## 📈 Roadmap

### Near Term

- [ ] **Metrics Integration**: Prometheus/Grafana support
- [ ] **Configuration Files**: YAML/TOML configuration
- [ ] **Database Storage**: SQLite for local persistence
- [ ] **Alert System**: Threshold-based notifications

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

**Atmosbyte** provides a robust, extensible foundation for real-time weather data processing with modern web visualization! 🌡️📊🌐
