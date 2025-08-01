package web

import (
	"net/http"
	"time"

	"github.com/anibaldeboni/zero-paper/atmosbyte/queue"
)

// QueueStatsProvider define a interface para obter estatísticas da fila
type QueueStatsProvider interface {
	Stats() queue.QueueStats
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

// loggingResponseWriter wraps http.ResponseWriter to capture status codes
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}
