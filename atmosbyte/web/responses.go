package web

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

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

// getSensorStatus returns the current sensor status
func (s *Server) getSensorStatus() string {
	_, err := s.sensor.Read()
	if err != nil {
		return "error"
	}
	return "connected"
}
