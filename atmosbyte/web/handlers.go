package web

import (
	"log"
	"net/http"
	"time"
)

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
		Source:      s.sensor.Name(),
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
		SystemStartTime: s.systemStartTime.Format("02/01/2006 15:04:05"),
		Routes:          s.GetRoutes(),
		QueueAvailable:  s.queue != nil,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := s.template.Execute(w, data); err != nil {
		log.Printf("Failed to execute template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
