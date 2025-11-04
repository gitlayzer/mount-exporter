package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mount-exporter/mount-exporter/config"
	"github.com/mount-exporter/mount-exporter/metrics"
	"github.com/mount-exporter/mount-exporter/resources"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var version = "dev" // This will be set at build time

// Server represents the HTTP server
type Server struct {
	config          *config.Config
	collector       *metrics.Collector
	registry        *prometheus.Registry
	httpServer      *http.Server
	logger          *log.Logger
	resourceManager *resources.ResourceManager
}

// NewServer creates a new HTTP server
func NewServer(cfg *config.Config, logger *log.Logger) (*Server, error) {
	// Create metrics collector
	collector := metrics.NewCollector(cfg)

	// Create Prometheus registry
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)

	// Create resource manager
	resourceManager := resources.NewResourceManager(resources.ResourceManagerConfig{
		Logger:     &resourcesLogger{logger: logger},
		EnableGC:   true,
		GCInterval: 5 * time.Minute,
	})

	// Create HTTP server
	server := &Server{
		config:          cfg,
		collector:       collector,
		registry:        registry,
		logger:          logger,
		resourceManager: resourceManager,
	}

	return server, nil
}

// setupRoutes sets up the HTTP routes
func (s *Server) setupRoutes() {
	mux := http.NewServeMux()

	// Metrics endpoint
	mux.Handle(s.config.Server.Path, promhttp.HandlerFor(s.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	}))

	// Health endpoint
	mux.HandleFunc("/health", s.healthHandler)
	mux.HandleFunc("/healthz", s.healthHandler) // Alternative health endpoint

	// Root endpoint
	mux.HandleFunc("/", s.rootHandler)

	// Apply middleware
	handler := s.loggingMiddleware(mux)
	handler = s.securityMiddleware(handler)

	s.httpServer = &http.Server{
		Addr:         s.config.GetAddress(),
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

// healthHandler handles health check requests
func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if findmnt is available
	findmnt := s.collector.GetFindmntWrapper()
	if !findmnt.IsAvailable() {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"status": "unhealthy", "error": "findmnt command not available"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "healthy"}`))
}

// rootHandler handles requests to the root path
func (s *Server) rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Mount Exporter\n\n")
	fmt.Fprintf(w, "Metrics: %s\n", s.config.Server.Path)
	fmt.Fprintf(w, "Health: /health\n")
	fmt.Fprintf(w, "Version: %s\n", version)
}

// loggingMiddleware adds request logging
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a response writer wrapper to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)
		s.logger.Printf("%s %s %d %v", r.Method, r.URL.Path, wrapped.statusCode, duration)
	})
}

// securityMiddleware adds security headers
func (s *Server) securityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add security headers
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		next.ServeHTTP(w, r)
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.setupRoutes()

	// Register resources for cleanup
	s.registerResources()

	s.logger.Printf("Starting server on %s", s.config.GetAddress())
	s.logger.Printf("Metrics available at %s", s.config.Server.Path)
	s.logger.Printf("Health check available at /health")

	// Create listener for better control
	listener, err := net.Listen("tcp", s.config.GetAddress())
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}

	// Register listener as a resource
	listenerID := fmt.Sprintf("tcp-listener-%s", s.config.GetAddress())
	s.resourceManager.RegisterResource(
		listenerID,
		resources.ResourceTypeNetwork,
		fmt.Sprintf("TCP listener on %s", s.config.GetAddress()),
		func() error {
			return listener.Close()
		},
	)

	// Start server in a goroutine
	go func() {
		if err := s.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			s.logger.Printf("Server error: %v", err)
		}
	}()

	return nil
}

// Stop gracefully shuts down the server
func (s *Server) Stop(ctx context.Context) error {
	if s.httpServer == nil {
		return fmt.Errorf("server not initialized")
	}

	s.logger.Println("Shutting down server...")

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		s.logger.Printf("Server shutdown error: %v", err)
		return err
	}

	s.logger.Println("Server shutdown complete")

	// Cleanup all registered resources
	s.logger.Println("Cleaning up resources...")
	if s.resourceManager != nil {
		errors := s.resourceManager.CleanupAll()
		if len(errors) > 0 {
			s.logger.Printf("Resource cleanup encountered %d errors", len(errors))
			for _, err := range errors {
				s.logger.Printf("Cleanup error: %v", err)
			}
		} else {
			s.logger.Println("All resources cleaned up successfully")
		}

		// Close resource manager
		s.resourceManager.Close()
	}

	return nil
}

// WaitForShutdown waits for shutdown signals and gracefully shuts down the server
func (s *Server) WaitForShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	s.logger.Printf("Received signal: %v", sig)

	// Create context for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.Stop(ctx); err != nil {
		s.logger.Printf("Graceful shutdown failed: %v", err)
		os.Exit(1)
	}

	os.Exit(0)
}

// GetAddress returns the server address
func (s *Server) GetAddress() string {
	return s.config.GetAddress()
}

// GetCollector returns the metrics collector
func (s *Server) GetCollector() *metrics.Collector {
	return s.collector
}

// GetHTTPServer returns the underlying HTTP server (for testing)
func (s *Server) GetHTTPServer() *http.Server {
	return s.httpServer
}

// SetupRoutes sets up the HTTP routes (for testing)
func (s *Server) SetupRoutes() {
	s.setupRoutes()
}

// SetVersion sets the server version
func SetVersion(v string) {
	version = v
}

// registerResources registers application resources for cleanup
func (s *Server) registerResources() {
	if s.resourceManager == nil {
		return
	}

	// Register HTTP server as a resource
	s.resourceManager.RegisterResource(
		"http-server",
		resources.ResourceTypeNetwork,
		fmt.Sprintf("HTTP server on %s", s.config.GetAddress()),
		func() error {
			if s.httpServer != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				return s.httpServer.Shutdown(ctx)
			}
			return nil
		},
	)

	// Register metrics collector as a resource
	s.resourceManager.RegisterResource(
		"metrics-collector",
		resources.ResourceTypeCustom,
		"Prometheus metrics collector",
		func() error {
			// Collector doesn't need explicit cleanup, but we register it for tracking
			return nil
		},
	)

	s.logger.Println("Registered application resources for cleanup")
}

// GetResourceManager returns the resource manager (for testing)
func (s *Server) GetResourceManager() *resources.ResourceManager {
	return s.resourceManager
}

// resourcesLogger adapts standard log.Logger to resources.Logger interface
type resourcesLogger struct {
	logger *log.Logger
}

func (l *resourcesLogger) Printf(format string, args ...interface{}) {
	l.logger.Printf(format, args...)
}