package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// GameServiceMetrics contains all Game Service related Prometheus metrics
type GameServiceMetrics struct {
	// Game Instance Metrics
	GameInstancesTotal   *prometheus.CounterVec
	GameInstancesActive  *prometheus.GaugeVec
	GameInstanceDuration *prometheus.HistogramVec
	GameInstanceFailures *prometheus.CounterVec

	// Session Metrics
	SessionsStartedTotal *prometheus.CounterVec
	SessionsEndedTotal   *prometheus.CounterVec
	SessionsActive       *prometheus.GaugeVec
	SessionDuration      *prometheus.HistogramVec
	SessionCrashes       *prometheus.CounterVec

	// Save Operations Metrics
	SaveOperationsTotal   *prometheus.CounterVec
	SaveOperationDuration *prometheus.HistogramVec
	SaveFileSizeBytes     *prometheus.HistogramVec
	SaveOperationFailures *prometheus.CounterVec

	// Resource Usage Metrics
	GameProcessMemoryBytes *prometheus.GaugeVec
	GameProcessCPUSeconds  *prometheus.CounterVec
	GameProcessCount       *prometheus.GaugeVec

	// Game Management Metrics
	GamesTotal            *prometheus.GaugeVec
	GamesEnabled          *prometheus.GaugeVec
	GameConfigReloads     *prometheus.CounterVec
	GameSetupOperations   *prometheus.CounterVec
	GameCleanupOperations *prometheus.CounterVec
}

// NewGameServiceMetrics creates and registers all Game Service metrics
func NewGameServiceMetrics(namespace string) *GameServiceMetrics {
	return &GameServiceMetrics{
		// Game Instance Metrics
		GameInstancesTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "game",
			Name:      "instances_total",
			Help:      "Total number of game instances started",
		}, []string{"game_id", "game_name", "status"}),
		GameInstancesActive: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "game",
			Name:      "instances_active",
			Help:      "Number of active game instances",
		}, []string{"game_id", "game_name"}),
		GameInstanceDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "game",
			Name:      "instance_duration_seconds",
			Help:      "Game instance duration in seconds",
			Buckets:   []float64{60, 300, 900, 1800, 3600, 7200, 14400, 28800}, // 1m to 8h
		}, []string{"game_id", "game_name"}),
		GameInstanceFailures: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "game",
			Name:      "instance_failures_total",
			Help:      "Total number of game instance failures",
		}, []string{"game_id", "failure_reason"}),

		// Session Metrics
		SessionsStartedTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "session",
			Name:      "started_total",
			Help:      "Total number of game sessions started",
		}, []string{"game_id", "user_id"}),
		SessionsEndedTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "session",
			Name:      "ended_total",
			Help:      "Total number of game sessions ended",
		}, []string{"game_id", "end_reason"}),
		SessionsActive: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "session",
			Name:      "active",
			Help:      "Number of active game sessions",
		}, []string{"game_id", "game_name"}),
		SessionDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "session",
			Name:      "duration_seconds",
			Help:      "Game session duration in seconds",
			Buckets:   []float64{60, 300, 900, 1800, 3600, 7200, 14400, 28800}, // 1m to 8h
		}, []string{"game_id"}),
		SessionCrashes: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "session",
			Name:      "crashes_total",
			Help:      "Total number of game session crashes",
		}, []string{"game_id", "crash_reason"}),

		// Save Operations Metrics
		SaveOperationsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "save",
			Name:      "operations_total",
			Help:      "Total number of save file operations",
		}, []string{"game_id", "operation", "status"}),
		SaveOperationDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "save",
			Name:      "operation_duration_seconds",
			Help:      "Save operation duration in seconds",
			Buckets:   prometheus.DefBuckets,
		}, []string{"operation"}),
		SaveFileSizeBytes: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "save",
			Name:      "file_size_bytes",
			Help:      "Save file size in bytes",
			Buckets:   prometheus.ExponentialBuckets(1024, 2, 20), // 1KB to 1GB
		}, []string{"game_id"}),
		SaveOperationFailures: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "save",
			Name:      "operation_failures_total",
			Help:      "Total number of save operation failures",
		}, []string{"game_id", "operation", "error_type"}),

		// Resource Usage Metrics
		GameProcessMemoryBytes: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "resource",
			Name:      "process_memory_bytes",
			Help:      "Game process memory usage in bytes",
		}, []string{"game_id", "session_id", "pid"}),
		GameProcessCPUSeconds: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "resource",
			Name:      "process_cpu_seconds_total",
			Help:      "Game process CPU usage in seconds",
		}, []string{"game_id", "session_id", "pid"}),
		GameProcessCount: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "resource",
			Name:      "process_count",
			Help:      "Number of game processes running",
		}, []string{"game_id"}),

		// Game Management Metrics
		GamesTotal: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "management",
			Name:      "games_total",
			Help:      "Total number of configured games",
		}, []string{"status"}),
		GamesEnabled: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "management",
			Name:      "games_enabled",
			Help:      "Number of enabled games",
		}, []string{"game_id", "game_name"}),
		GameConfigReloads: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "management",
			Name:      "config_reloads_total",
			Help:      "Total number of game configuration reloads",
		}, []string{"game_id", "status"}),
		GameSetupOperations: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "management",
			Name:      "setup_operations_total",
			Help:      "Total number of game setup operations",
		}, []string{"game_id", "operation", "status"}),
		GameCleanupOperations: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "management",
			Name:      "cleanup_operations_total",
			Help:      "Total number of game cleanup operations",
		}, []string{"game_id", "operation", "status"}),
	}
}
