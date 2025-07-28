// Package web provides HTTP server functionality for the Atmosbyte weather monitoring system.
// It exposes endpoints to retrieve real-time sensor measurements and system status.
package web

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/anibaldeboni/zero-paper/atmosbyte/bme280"
)

//go:embed templates/index.html
var indexTemplate string

var (
	systemStartTime = time.Now()
)

// SensorProvider defines the interface for retrieving sensor measurements
type SensorProvider interface {
	Read() (bme280.Measurement, error)
}

// Server encapsulates the HTTP server configuration and dependencies
type Server struct {
	server       *http.Server
	sensor       SensorProvider
	useSimulated bool
	template     *template.Template
}

// Config holds configuration options for the web server
type Config struct {
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Port:         8080,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
}

// MeasurementResponse represents the JSON response for measurement endpoints
type MeasurementResponse struct {
	Timestamp   time.Time `json:"timestamp"`
	Temperature float64   `json:"temperature"`
	Humidity    float64   `json:"humidity"`
	Pressure    float64   `json:"pressure"`
	Source      string    `json:"source"`
}

// ErrorResponse represents the JSON response for errors
type ErrorResponse struct {
	Error string    `json:"error"`
	Code  int       `json:"code"`
	Time  time.Time `json:"timestamp"`
}

// TemplateData holds data passed to HTML templates
type TemplateData struct {
	SystemStartTime string
}

// NewServer creates a new HTTP server instance with the given sensor provider
func NewServer(sensor SensorProvider, useSimulated bool, config *Config) *Server {
	if config == nil {
		config = DefaultConfig()
	}

	// Compile the HTML template
	tmpl, err := template.New("index").Parse(indexTemplate)
	if err != nil {
		log.Fatalf("Failed to parse HTML template: %v", err)
	}

	s := &Server{
		sensor:       sensor,
		useSimulated: useSimulated,
		template:     tmpl,
	}

	mux := http.NewServeMux()
	s.setupRoutes(mux)

	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", config.Port),
		Handler:      s.loggingMiddleware(mux),
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		IdleTimeout:  config.IdleTimeout,
	}

	return s
}

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/measurements", s.handleMeasurements)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/", s.handleRoot)
}

// Start starts the HTTP server
func (s *Server) Start() error {
	log.Printf("🌐 Starting HTTP server on %s", s.server.Addr)

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}

// Shutdown gracefully shuts down the HTTP server
func (s *Server) Shutdown(ctx context.Context) error {
	log.Println("Shutting down HTTP server...")

	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown server: %w", err)
	}

	log.Println("HTTP server stopped")
	return nil
}

// handleMeasurements handles GET /measurements - returns current sensor reading
func (s *Server) handleMeasurements(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	measurement, err := s.sensor.Read()
	if err != nil {
		log.Printf("Failed to read sensor: %v", err)
		s.sendErrorResponse(w, "Failed to read sensor data", http.StatusInternalServerError)
		return
	}

	source := "BME280"
	if s.useSimulated {
		source = "Simulated"
	}

	response := MeasurementResponse{
		Timestamp:   time.Now(),
		Temperature: measurement.Temperature,
		Humidity:    measurement.Humidity,
		Pressure:    float64(measurement.Pressure),
		Source:      source,
	}

	s.sendJSONResponse(w, response, http.StatusOK)
}

// handleHealth handles GET /health - returns server health status
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	health := map[string]any{
		"status":    "healthy",
		"timestamp": time.Now(),
		"sensor":    s.getSensorStatus(),
	}

	s.sendJSONResponse(w, health, http.StatusOK)
}

// handleRoot handles GET / - returns HTML page with weather information
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if the request wants JSON (API client)
	if r.Header.Get("Accept") == "application/json" || r.URL.Query().Get("format") == "json" {
		s.handleAPIInfo(w, r)
		return
	}

	// Serve HTML page
	data := TemplateData{
		SystemStartTime: systemStartTime.Format("02/01/2006 15:04:05"),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := s.template.Execute(w, data); err != nil {
		log.Printf("Failed to execute template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleAPIInfo handles API information requests (JSON format)
func (s *Server) handleAPIInfo(w http.ResponseWriter, _ *http.Request) {
	info := map[string]any{
		"service":   "Atmosbyte Weather API",
		"version":   "1.0.0",
		"timestamp": time.Now(),
		"endpoints": map[string]string{
			"/":             "Web interface / API information",
			"/health":       "Health check",
			"/measurements": "Current sensor measurements",
		},
	}

	s.sendJSONResponse(w, info, http.StatusOK)
}

// getSensorStatus returns the current sensor status
func (s *Server) getSensorStatus() string {
	if s.useSimulated {
		return "simulated"
	}

	// Try to read from sensor to check if it's working
	_, err := s.sensor.Read()
	if err != nil {
		return "error"
	}

	return "connected"
}

// sendJSONResponse sends a JSON response with proper headers
func (s *Server) sendJSONResponse(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Failed to encode JSON response: %v", err)
	}
}

// sendErrorResponse sends a JSON error response
func (s *Server) sendErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	response := ErrorResponse{
		Error: message,
		Code:  statusCode,
		Time:  time.Now(),
	}

	s.sendJSONResponse(w, response, statusCode)
}

// loggingMiddleware logs all HTTP requests
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a custom ResponseWriter to capture status code
		lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(lrw, r)

		duration := time.Since(start)
		log.Printf("%s %s %d %v %s",
			r.Method,
			r.URL.Path,
			lrw.statusCode,
			duration,
			r.RemoteAddr,
		)
	})
}

// loggingResponseWriter wraps http.ResponseWriter to capture status codes
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}
