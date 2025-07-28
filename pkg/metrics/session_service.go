package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// SessionServiceMetrics contains all Session Service related Prometheus metrics
type SessionServiceMetrics struct {
	// SSH Connection metrics
	SSHConnectionsTotal   *prometheus.CounterVec
	SSHConnectionsActive  prometheus.Gauge
	SSHConnectionsFailed  *prometheus.CounterVec
	SSHConnectionDuration *prometheus.HistogramVec

	// SSH Session metrics
	SSHSessionsTotal     *prometheus.CounterVec
	SSHSessionsActive    prometheus.Gauge
	SSHSessionDuration   *prometheus.HistogramVec
	SSHSessionBytesRead  *prometheus.CounterVec
	SSHSessionBytesWrite *prometheus.CounterVec

	// SSH Authentication metrics
	SSHAuthAttemptsTotal *prometheus.CounterVec
	SSHAuthFailuresTotal *prometheus.CounterVec
	SSHAuthDuration      *prometheus.HistogramVec

	// Terminal metrics
	TerminalSizeChanges *prometheus.CounterVec
	TerminalTypes       *prometheus.CounterVec

	// Spectating metrics
	SpectatorConnectionsTotal  *prometheus.CounterVec
	SpectatorConnectionsActive prometheus.Gauge
	SpectatingSessionsActive   prometheus.Gauge
}

// NewSessionServiceMetrics creates and registers all Session Service metrics
func NewSessionServiceMetrics(namespace string) *SessionServiceMetrics {
	return &SessionServiceMetrics{
		// SSH Connection metrics
		SSHConnectionsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "ssh",
			Name:      "connections_total",
			Help:      "Total number of SSH connections",
		}, []string{"status"}),
		SSHConnectionsActive: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "ssh",
			Name:      "connections_active",
			Help:      "Number of active SSH connections",
		}),
		SSHConnectionsFailed: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "ssh",
			Name:      "connections_failed_total",
			Help:      "Total number of failed SSH connections",
		}, []string{"reason"}),
		SSHConnectionDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "ssh",
			Name:      "connection_duration_seconds",
			Help:      "SSH connection duration in seconds",
			Buckets:   prometheus.DefBuckets,
		}, []string{"status"}),

		// SSH Session metrics
		SSHSessionsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "ssh",
			Name:      "sessions_total",
			Help:      "Total number of SSH sessions",
		}, []string{"status"}),
		SSHSessionsActive: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "ssh",
			Name:      "sessions_active",
			Help:      "Number of active SSH sessions",
		}),
		SSHSessionDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "ssh",
			Name:      "session_duration_seconds",
			Help:      "SSH session duration in seconds",
			Buckets:   prometheus.DefBuckets,
		}, []string{"user_type"}),
		SSHSessionBytesRead: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "ssh",
			Name:      "session_bytes_read_total",
			Help:      "Total bytes read from SSH sessions",
		}, []string{"session_type"}),
		SSHSessionBytesWrite: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "ssh",
			Name:      "session_bytes_written_total",
			Help:      "Total bytes written to SSH sessions",
		}, []string{"session_type"}),

		// SSH Authentication metrics
		SSHAuthAttemptsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "ssh",
			Name:      "auth_attempts_total",
			Help:      "Total number of SSH authentication attempts",
		}, []string{"method", "username"}),
		SSHAuthFailuresTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "ssh",
			Name:      "auth_failures_total",
			Help:      "Total number of SSH authentication failures",
		}, []string{"method", "reason"}),
		SSHAuthDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "ssh",
			Name:      "auth_duration_seconds",
			Help:      "SSH authentication duration in seconds",
			Buckets:   prometheus.DefBuckets,
		}, []string{"method"}),

		// Terminal metrics
		TerminalSizeChanges: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "terminal",
			Name:      "size_changes_total",
			Help:      "Total number of terminal size changes",
		}, []string{"session_id"}),
		TerminalTypes: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "terminal",
			Name:      "types_total",
			Help:      "Terminal types used for connections",
		}, []string{"type"}),

		// Spectating metrics
		SpectatorConnectionsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "spectator",
			Name:      "connections_total",
			Help:      "Total number of spectator connections",
		}, []string{"status"}),
		SpectatorConnectionsActive: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "spectator",
			Name:      "connections_active",
			Help:      "Number of active spectator connections",
		}),
		SpectatingSessionsActive: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "spectator",
			Name:      "sessions_active",
			Help:      "Number of sessions being spectated",
		}),
	}
}
