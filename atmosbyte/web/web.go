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
	"github.com/anibaldeboni/zero-paper/atmosbyte/queue"
)

//go:embed templates/index.html
var indexTemplate string

var (
	systemStartTime = time.Now()
)

// Server encapsulates the HTTP server configuration and dependencies
type Server struct {
	config   *Config
	htmlData *TemplateData
	sensor   bme280.Reader
	template *template.Template
	server   *http.Server
	routes   map[string]string  // Armazena as rotas e suas descrições
	queue    QueueStatsProvider // Interface para obter estatísticas da fila
	ctx      context.Context    // Contexto para shutdown
}

// QueueStatsProvider define a interface para obter estatísticas da fila
type QueueStatsProvider interface {
	Stats() queue.QueueStats
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
	Routes          map[string]string
	QueueAvailable  bool
}

// NewServer creates a new HTTP server instance with the given sensor provider
// Optionally accepts a queue parameter for queue monitoring functionality
func NewServer(ctx context.Context, sensor bme280.Reader, config *Config, queue QueueStatsProvider) *Server {
	if config == nil {
		config = DefaultConfig()
	}

	// Compile the HTML template
	tmpl, err := template.New("index").Parse(indexTemplate)
	if err != nil {
		log.Fatalf("Failed to parse HTML template: %v", err)
	}

	s := &Server{
		config:   config,
		sensor:   sensor,
		template: tmpl,
		queue:    queue,
		ctx:      ctx,
		routes: map[string]string{
			"/":             "Interface web principal",
			"/health":       "Status de saúde do sistema",
			"/measurements": "Dados atuais do sensor (JSON)",
			"/queue":        "Status da fila de processamento (JSON)",
		},
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
	mux.HandleFunc("/queue", s.handleQueue)
	mux.HandleFunc("/", s.handleRoot)
}

// GetRoutes returns the configured routes and their descriptions
func (s *Server) GetRoutes() map[string]string {
	return s.routes
}

// Start starts the HTTP server and handles graceful shutdown via context
func (s *Server) Start() error {
	log.Printf("Starting HTTP server on %s", s.server.Addr)

	// Canal para capturar erros do servidor
	serverErr := make(chan error, 1)

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- fmt.Errorf("failed to start server: %w", err)
		} else {
			serverErr <- nil
		}
	}()

	select {
	case <-s.ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := s.server.Shutdown(shutdownCtx); err != nil {
			log.Printf("Error during server shutdown: %v", err)
			return fmt.Errorf("failed to shutdown server: %w", err)
		}

		log.Println("HTTP server stopped")
		return s.ctx.Err()

	case err := <-serverErr:
		return err
	}
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

	response := MeasurementResponse{
		Timestamp:   time.Now(),
		Temperature: measurement.Temperature,
		Humidity:    measurement.Humidity,
		Pressure:    float64(measurement.Pressure),
		Source:      "BME280",
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

// handleQueue handles GET /queue - returns queue status
func (s *Server) handleQueue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.queue == nil {
		s.sendErrorResponse(w, "Queue not available", http.StatusServiceUnavailable)
		return
	}

	stats := s.queue.Stats()
	response := map[string]any{
		"queue_size":            stats.QueueSize,
		"retry_queue_size":      stats.RetryQueueSize,
		"circuit_breaker_state": int(stats.CircuitBreakerState),
		"workers":               stats.Workers,
		"timestamp":             time.Now(),
	}

	s.sendJSONResponse(w, response, http.StatusOK)
}

// handleRoot handles GET / - returns HTML page with weather information
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Serve HTML page
	data := TemplateData{
		SystemStartTime: systemStartTime.Format("02/01/2006 15:04:05"),
		Routes:          s.GetRoutes(),
		QueueAvailable:  s.queue != nil,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := s.template.Execute(w, data); err != nil {
		log.Printf("Failed to execute template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// getSensorStatus returns the current sensor status
func (s *Server) getSensorStatus() string {
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
