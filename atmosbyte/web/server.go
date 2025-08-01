package web

import (
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"log"
	"maps"
	"net/http"
	"time"

	"github.com/anibaldeboni/zero-paper/atmosbyte/bme280"
)

//go:embed templates/index.html
var indexTemplate string

// Server encapsulates the HTTP server configuration and dependencies
type Server struct {
	config          *Config
	htmlData        *TemplateData
	sensor          bme280.Reader
	template        *template.Template
	server          *http.Server
	routes          map[string]string  // Armazena as rotas e suas descrições
	queue           QueueStatsProvider // Interface para obter estatísticas da fila
	ctx             context.Context    // Contexto para shutdown
	systemStartTime time.Time          // Thread-safe: set once during initialization
}

// NewServer creates a new HTTP server instance with the given sensor provider
// Optionally accepts a queue parameter for queue monitoring functionality
func NewServer(ctx context.Context, sensor bme280.Reader, config *Config, queue QueueStatsProvider) *Server {
	if config == nil {
		panic("web.Config cannot be nil - use config.Load().WebConfig() instead")
	}

	// Compile the HTML template
	tmpl, err := template.New("index").Parse(indexTemplate)
	if err != nil {
		log.Fatalf("Failed to parse HTML template: %v", err)
	}

	s := &Server{
		config:          config,
		sensor:          sensor,
		template:        tmpl,
		queue:           queue,
		ctx:             ctx,
		systemStartTime: time.Now(), // Thread-safe: set once during initialization
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
// Thread-safe: creates a copy to avoid concurrent access issues
func (s *Server) GetRoutes() map[string]string {
	// Create a copy to avoid race conditions on map access
	routesCopy := make(map[string]string, len(s.routes))
	maps.Copy(routesCopy, s.routes)
	return routesCopy
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
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.config.ShutdownTimeout)
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
