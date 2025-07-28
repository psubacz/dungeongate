package metrics

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// CommonMetrics contains metrics shared by all services

// ServiceMetrics contains general service health metrics
type ServiceMetrics struct {
	// General service metrics
	BuildInfo *prometheus.GaugeVec
	StartTime prometheus.Gauge

	// HTTP metrics
	HTTPRequestsTotal   *prometheus.CounterVec
	HTTPRequestDuration *prometheus.HistogramVec
	HTTPResponseSize    *prometheus.HistogramVec

	// gRPC metrics
	GRPCRequestsTotal   *prometheus.CounterVec
	GRPCRequestDuration *prometheus.HistogramVec

	// Database metrics
	DBConnectionsActive prometheus.Gauge
	DBQueriesTotal      *prometheus.CounterVec
	DBQueryDuration     *prometheus.HistogramVec
	DBErrors            *prometheus.CounterVec
}

// NewServiceMetrics creates and registers all service metrics
func NewServiceMetrics(namespace string) *ServiceMetrics {
	return &ServiceMetrics{
		BuildInfo: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "build_info",
			Help:      "Build information",
		}, []string{"version", "commit", "build_time"}),
		StartTime: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "start_time_seconds",
			Help:      "Unix timestamp of service start time",
		}),

		// HTTP metrics
		HTTPRequestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total number of HTTP requests",
		}, []string{"method", "path", "status"}),
		HTTPRequestDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "HTTP request duration in seconds",
			Buckets:   prometheus.DefBuckets,
		}, []string{"method", "path"}),
		HTTPResponseSize: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "http",
			Name:      "response_size_bytes",
			Help:      "HTTP response size in bytes",
			Buckets:   prometheus.ExponentialBuckets(100, 2, 10),
		}, []string{"method", "path"}),

		// gRPC metrics
		GRPCRequestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "grpc",
			Name:      "requests_total",
			Help:      "Total number of gRPC requests",
		}, []string{"method", "status"}),
		GRPCRequestDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "grpc",
			Name:      "request_duration_seconds",
			Help:      "gRPC request duration in seconds",
			Buckets:   prometheus.DefBuckets,
		}, []string{"method"}),

		// Database metrics
		DBConnectionsActive: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "database",
			Name:      "connections_active",
			Help:      "Number of active database connections",
		}),
		DBQueriesTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "database",
			Name:      "queries_total",
			Help:      "Total number of database queries",
		}, []string{"query_type", "table"}),
		DBQueryDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "database",
			Name:      "query_duration_seconds",
			Help:      "Database query duration in seconds",
			Buckets:   prometheus.DefBuckets,
		}, []string{"query_type"}),
		DBErrors: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "database",
			Name:      "errors_total",
			Help:      "Total number of database errors",
		}, []string{"error_type"}),
	}
}

// Registry represents a metrics registry for a service
type Registry struct {
	serviceName    string
	serviceVersion string
	buildTime      string
	gitCommit      string
	logger         *slog.Logger

	// Core metrics
	Service        *ServiceMetrics
	SessionService *SessionServiceMetrics
	GameService    *GameServiceMetrics
	AuthService    *AuthServiceMetrics

	// HTTP server for metrics endpoint
	server *http.Server
}

// NewRegistry creates a new metrics registry
func NewRegistry(serviceName, version, buildTime, gitCommit string, logger *slog.Logger) *Registry {
	reg := &Registry{
		serviceName:    serviceName,
		serviceVersion: version,
		buildTime:      buildTime,
		gitCommit:      gitCommit,
		logger:         logger,
	}

	// Initialize common service metrics
	reg.Service = NewServiceMetrics("dungeongate")

	// Initialize service-specific metrics based on service name
	switch serviceName {
	case "session-service":
		reg.SessionService = NewSessionServiceMetrics("dungeongate")
	case "game-service":
		reg.GameService = NewGameServiceMetrics("dungeongate")
	case "auth-service":
		reg.AuthService = NewAuthServiceMetrics("dungeongate")
	}

	// Set build info
	reg.Service.BuildInfo.WithLabelValues(version, gitCommit, buildTime).Set(1)
	reg.Service.StartTime.SetToCurrentTime()

	return reg
}

// StartMetricsServer starts the HTTP server for Prometheus metrics
func (r *Registry) StartMetricsServer(port int) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","service":"` + r.serviceName + `"}`))
	})

	r.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	r.logger.Info("Starting metrics server", "port", port)
	return r.server.ListenAndServe()
}

// StopMetricsServer stops the metrics HTTP server
func (r *Registry) StopMetricsServer(ctx context.Context) error {
	if r.server == nil {
		return nil
	}
	r.logger.Info("Stopping metrics server")
	return r.server.Shutdown(ctx)
}

// HTTPMiddleware returns HTTP middleware that instruments requests
func (r *Registry) HTTPMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			start := time.Now()

			// Create response writer wrapper to capture status code
			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// Process request
			next.ServeHTTP(wrapped, req)

			// Record metrics
			duration := time.Since(start)
			status := strconv.Itoa(wrapped.statusCode)

			r.Service.HTTPRequestsTotal.WithLabelValues(req.Method, req.URL.Path, status).Inc()
			r.Service.HTTPRequestDuration.WithLabelValues(req.Method, req.URL.Path).Observe(duration.Seconds())

			// Log request with metrics correlation
			r.logger.Info("HTTP request",
				"method", req.Method,
				"path", req.URL.Path,
				"status", status,
				"duration_ms", duration.Milliseconds(),
				"remote_addr", req.RemoteAddr,
			)
		})
	}
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

// UnaryServerInterceptor returns a gRPC unary interceptor that instruments requests
func (r *Registry) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()

		// Process request
		resp, err := handler(ctx, req)

		// Record metrics
		duration := time.Since(start)
		statusCode := "OK"
		if err != nil {
			statusCode = status.Code(err).String()
		}

		// Extract method name from full method path
		method := info.FullMethod

		r.Service.GRPCRequestsTotal.WithLabelValues(method, statusCode).Inc()
		r.Service.GRPCRequestDuration.WithLabelValues(method).Observe(duration.Seconds())

		// Log request with metrics correlation
		r.logger.Info("gRPC request",
			"method", method,
			"status", statusCode,
			"duration_ms", duration.Milliseconds(),
		)

		return resp, err
	}
}

// StreamServerInterceptor returns a gRPC stream interceptor that instruments streams
func (r *Registry) StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()

		// Process stream
		err := handler(srv, ss)

		// Record metrics
		duration := time.Since(start)
		statusCode := "OK"
		if err != nil {
			statusCode = status.Code(err).String()
		}

		method := info.FullMethod

		r.Service.GRPCRequestsTotal.WithLabelValues(method, statusCode).Inc()
		r.Service.GRPCRequestDuration.WithLabelValues(method).Observe(duration.Seconds())

		// Log stream with metrics correlation
		r.logger.Info("gRPC stream",
			"method", method,
			"status", statusCode,
			"duration_ms", duration.Milliseconds(),
		)

		return err
	}
}
