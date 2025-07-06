package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// SSHMetrics contains all SSH server related Prometheus metrics
type SSHMetrics struct {
	// Connection metrics
	ConnectionsTotal   prometheus.Counter
	ConnectionsActive  prometheus.Gauge
	ConnectionsFailed  prometheus.Counter
	ConnectionDuration prometheus.Histogram

	// Session metrics
	SessionsTotal     prometheus.Counter
	SessionsActive    prometheus.Gauge
	SessionDuration   prometheus.Histogram
	SessionBytesRead  prometheus.Counter
	SessionBytesWrite prometheus.Counter

	// Authentication metrics
	AuthAttemptsTotal  *prometheus.CounterVec
	AuthFailuresTotal  *prometheus.CounterVec
	AuthDuration       prometheus.Histogram

	// Game metrics
	GamesStartedTotal  *prometheus.CounterVec
	GamesActive        *prometheus.GaugeVec
	GameDuration       *prometheus.HistogramVec
	GameSessionErrors  *prometheus.CounterVec

	// Terminal metrics
	TerminalSizeChanges prometheus.Counter
	TerminalTypes       *prometheus.CounterVec
}

// NewSSHMetrics creates and registers all SSH metrics
func NewSSHMetrics(namespace, subsystem string) *SSHMetrics {
	return &SSHMetrics{
		// Connection metrics
		ConnectionsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "connections_total",
			Help:      "Total number of SSH connections",
		}),
		ConnectionsActive: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "connections_active",
			Help:      "Number of active SSH connections",
		}),
		ConnectionsFailed: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "connections_failed_total",
			Help:      "Total number of failed SSH connections",
		}),
		ConnectionDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "connection_duration_seconds",
			Help:      "SSH connection duration in seconds",
			Buckets:   prometheus.DefBuckets,
		}),

		// Session metrics
		SessionsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "sessions_total",
			Help:      "Total number of SSH sessions",
		}),
		SessionsActive: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "sessions_active",
			Help:      "Number of active SSH sessions",
		}),
		SessionDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "session_duration_seconds",
			Help:      "SSH session duration in seconds",
			Buckets:   prometheus.DefBuckets,
		}),
		SessionBytesRead: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "session_bytes_read_total",
			Help:      "Total bytes read from SSH sessions",
		}),
		SessionBytesWrite: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "session_bytes_written_total",
			Help:      "Total bytes written to SSH sessions",
		}),

		// Authentication metrics
		AuthAttemptsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "auth_attempts_total",
			Help:      "Total number of authentication attempts",
		}, []string{"method", "username"}),
		AuthFailuresTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "auth_failures_total",
			Help:      "Total number of authentication failures",
		}, []string{"method", "reason"}),
		AuthDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "auth_duration_seconds",
			Help:      "Authentication duration in seconds",
			Buckets:   prometheus.DefBuckets,
		}),

		// Game metrics
		GamesStartedTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "games_started_total",
			Help:      "Total number of games started",
		}, []string{"game_id", "game_name"}),
		GamesActive: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "games_active",
			Help:      "Number of active game sessions",
		}, []string{"game_id", "game_name"}),
		GameDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "game_duration_seconds",
			Help:      "Game session duration in seconds",
			Buckets:   prometheus.DefBuckets,
		}, []string{"game_id", "game_name"}),
		GameSessionErrors: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "game_session_errors_total",
			Help:      "Total number of game session errors",
		}, []string{"game_id", "error_type"}),

		// Terminal metrics
		TerminalSizeChanges: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "terminal_size_changes_total",
			Help:      "Total number of terminal size changes",
		}),
		TerminalTypes: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "terminal_types_total",
			Help:      "Terminal types used for connections",
		}, []string{"type"}),
	}
}

// ServiceMetrics contains general service health metrics
type ServiceMetrics struct {
	// General service metrics
	BuildInfo *prometheus.GaugeVec
	StartTime prometheus.Gauge

	// HTTP metrics
	HTTPRequestsTotal    *prometheus.CounterVec
	HTTPRequestDuration  *prometheus.HistogramVec
	HTTPResponseSize     *prometheus.HistogramVec

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