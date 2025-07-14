package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// AuthServiceMetrics contains all Auth Service related Prometheus metrics
type AuthServiceMetrics struct {
	// Authentication Metrics
	LoginAttemptsTotal *prometheus.CounterVec
	LoginDuration      *prometheus.HistogramVec
	LoginFailuresTotal *prometheus.CounterVec
	LockoutsTotal      *prometheus.CounterVec

	// Token Metrics
	TokenGenerationsTotal *prometheus.CounterVec
	TokenValidationsTotal *prometheus.CounterVec
	TokenExpiredTotal     *prometheus.CounterVec
	TokenRefreshTotal     *prometheus.CounterVec

	// User Management Metrics
	UsersRegisteredTotal *prometheus.CounterVec
	UsersActiveTotal     prometheus.Gauge
	UserDeletionsTotal   *prometheus.CounterVec

	// Session Metrics
	ActiveSessionsTotal prometheus.Gauge
	SessionDuration     *prometheus.HistogramVec
	SessionLogoutsTotal *prometheus.CounterVec

	// Rate Limiting Metrics
	RateLimitHitsTotal     *prometheus.CounterVec
	RateLimitBlockedTotal  *prometheus.CounterVec
	BruteForceAttacksTotal *prometheus.CounterVec
}

// NewAuthServiceMetrics creates and registers all Auth Service metrics
func NewAuthServiceMetrics(namespace string) *AuthServiceMetrics {
	return &AuthServiceMetrics{
		// Authentication Metrics
		LoginAttemptsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "auth",
			Name:      "login_attempts_total",
			Help:      "Total number of login attempts",
		}, []string{"status", "user_type"}),
		LoginDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "auth",
			Name:      "login_duration_seconds",
			Help:      "Login operation duration in seconds",
			Buckets:   prometheus.DefBuckets,
		}, []string{"status"}),
		LoginFailuresTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "auth",
			Name:      "login_failures_total",
			Help:      "Total number of login failures",
		}, []string{"reason"}),
		LockoutsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "auth",
			Name:      "lockouts_total",
			Help:      "Total number of account lockouts",
		}, []string{"reason"}),

		// Token Metrics
		TokenGenerationsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "token",
			Name:      "generations_total",
			Help:      "Total number of tokens generated",
		}, []string{"token_type"}),
		TokenValidationsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "token",
			Name:      "validations_total",
			Help:      "Total number of token validations",
		}, []string{"status", "token_type"}),
		TokenExpiredTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "token",
			Name:      "expired_total",
			Help:      "Total number of expired tokens",
		}, []string{"token_type"}),
		TokenRefreshTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "token",
			Name:      "refresh_total",
			Help:      "Total number of token refreshes",
		}, []string{"status"}),

		// User Management Metrics
		UsersRegisteredTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "user",
			Name:      "registered_total",
			Help:      "Total number of registered users",
		}, []string{"method"}),
		UsersActiveTotal: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "user",
			Name:      "active_total",
			Help:      "Number of active users",
		}),
		UserDeletionsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "user",
			Name:      "deletions_total",
			Help:      "Total number of user deletions",
		}, []string{"reason"}),

		// Session Metrics
		ActiveSessionsTotal: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "session",
			Name:      "active_total",
			Help:      "Number of active authentication sessions",
		}),
		SessionDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "session",
			Name:      "duration_seconds",
			Help:      "Authentication session duration in seconds",
			Buckets:   []float64{300, 900, 1800, 3600, 7200, 14400, 28800, 86400}, // 5m to 24h
		}, []string{"logout_reason"}),
		SessionLogoutsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "session",
			Name:      "logouts_total",
			Help:      "Total number of session logouts",
		}, []string{"reason"}),

		// Rate Limiting Metrics
		RateLimitHitsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "rate_limit",
			Name:      "hits_total",
			Help:      "Total number of rate limit hits",
		}, []string{"endpoint", "client_ip"}),
		RateLimitBlockedTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "rate_limit",
			Name:      "blocked_total",
			Help:      "Total number of blocked requests due to rate limiting",
		}, []string{"endpoint", "client_ip"}),
		BruteForceAttacksTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "security",
			Name:      "brute_force_attacks_total",
			Help:      "Total number of detected brute force attacks",
		}, []string{"source_ip", "target_user"}),
	}
}
