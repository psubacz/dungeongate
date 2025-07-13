package resources

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// MetricType defines the type of metric being collected
type MetricType int

const (
	MetricTypeCounter MetricType = iota
	MetricTypeGauge
	MetricTypeHistogram
	MetricTypeSummary
)

// Metric represents a single metric value
type Metric struct {
	Name      string
	Type      MetricType
	Value     float64
	Labels    map[string]string
	Timestamp time.Time
	Help      string
}

// MetricCollector interface for collecting metrics
type MetricCollector interface {
	Collect() []Metric
	Describe() []MetricDescription
}

// MetricDescription describes a metric
type MetricDescription struct {
	Name string
	Type MetricType
	Help string
}

// CounterMetric represents a counter metric
type CounterMetric struct {
	name   string
	help   string
	value  int64
	labels map[string]string
}

// GaugeMetric represents a gauge metric
type GaugeMetric struct {
	name   string
	help   string
	value  int64
	labels map[string]string
	mu     sync.RWMutex
}

// HistogramMetric represents a histogram metric
type HistogramMetric struct {
	name     string
	help     string
	buckets  []float64
	counts   []int64
	sum      float64
	count    int64
	labels   map[string]string
	mu       sync.RWMutex
}

// MetricsRegistry manages all metrics
type MetricsRegistry struct {
	counters   map[string]*CounterMetric
	gauges     map[string]*GaugeMetric
	histograms map[string]*HistogramMetric
	collectors []MetricCollector
	
	logger       *slog.Logger
	shutdownChan chan struct{}
	
	mu sync.RWMutex
	wg sync.WaitGroup
}

// MetricsConfig holds configuration for metrics collection
type MetricsConfig struct {
	CollectionInterval time.Duration
	ExportInterval     time.Duration
	RetentionPeriod    time.Duration
	DefaultBuckets     []float64
}

// DefaultMetricsConfig returns a default metrics configuration
func DefaultMetricsConfig() *MetricsConfig {
	return &MetricsConfig{
		CollectionInterval: 10 * time.Second,
		ExportInterval:     30 * time.Second,
		RetentionPeriod:    24 * time.Hour,
		DefaultBuckets:     []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	}
}

// NewMetricsRegistry creates a new metrics registry
func NewMetricsRegistry(config *MetricsConfig, logger *slog.Logger) *MetricsRegistry {
	if config == nil {
		config = DefaultMetricsConfig()
	}

	mr := &MetricsRegistry{
		counters:     make(map[string]*CounterMetric),
		gauges:       make(map[string]*GaugeMetric),
		histograms:   make(map[string]*HistogramMetric),
		collectors:   make([]MetricCollector, 0),
		logger:       logger,
		shutdownChan: make(chan struct{}),
	}

	logger.Info("Created metrics registry",
		"collection_interval", config.CollectionInterval,
		"export_interval", config.ExportInterval,
		"retention_period", config.RetentionPeriod)

	return mr
}

// Start starts the metrics registry
func (mr *MetricsRegistry) Start(ctx context.Context) error {
	mr.logger.Info("Starting metrics registry")

	// Start metrics collection routine
	mr.wg.Add(1)
	go mr.collectionRoutine(ctx)

	// Start metrics export routine
	mr.wg.Add(1)
	go mr.exportRoutine(ctx)

	return nil
}

// Stop stops the metrics registry gracefully
func (mr *MetricsRegistry) Stop(ctx context.Context) error {
	mr.logger.Info("Stopping metrics registry")

	// Signal shutdown
	close(mr.shutdownChan)

	// Wait for goroutines to finish
	done := make(chan struct{})
	go func() {
		mr.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		mr.logger.Info("Metrics registry stopped gracefully")
	case <-ctx.Done():
		mr.logger.Warn("Metrics registry stop timeout exceeded")
	}

	return nil
}

// RegisterCounter registers a new counter metric
func (mr *MetricsRegistry) RegisterCounter(name, help string, labels map[string]string) *CounterMetric {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	key := mr.metricKey(name, labels)
	if existing, exists := mr.counters[key]; exists {
		mr.logger.Warn("Counter already registered", "name", name, "labels", labels)
		return existing
	}

	counter := &CounterMetric{
		name:   name,
		help:   help,
		labels: labels,
	}

	mr.counters[key] = counter
	mr.logger.Debug("Registered counter", "name", name, "labels", labels)

	return counter
}

// RegisterGauge registers a new gauge metric
func (mr *MetricsRegistry) RegisterGauge(name, help string, labels map[string]string) *GaugeMetric {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	key := mr.metricKey(name, labels)
	if existing, exists := mr.gauges[key]; exists {
		mr.logger.Warn("Gauge already registered", "name", name, "labels", labels)
		return existing
	}

	gauge := &GaugeMetric{
		name:   name,
		help:   help,
		labels: labels,
	}

	mr.gauges[key] = gauge
	mr.logger.Debug("Registered gauge", "name", name, "labels", labels)

	return gauge
}

// RegisterHistogram registers a new histogram metric
func (mr *MetricsRegistry) RegisterHistogram(name, help string, buckets []float64, labels map[string]string) *HistogramMetric {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	key := mr.metricKey(name, labels)
	if existing, exists := mr.histograms[key]; exists {
		mr.logger.Warn("Histogram already registered", "name", name, "labels", labels)
		return existing
	}

	if buckets == nil {
		buckets = DefaultMetricsConfig().DefaultBuckets
	}

	histogram := &HistogramMetric{
		name:    name,
		help:    help,
		buckets: buckets,
		counts:  make([]int64, len(buckets)+1), // +1 for +Inf bucket
		labels:  labels,
	}

	mr.histograms[key] = histogram
	mr.logger.Debug("Registered histogram", "name", name, "buckets", len(buckets), "labels", labels)

	return histogram
}

// RegisterCollector registers a custom metric collector
func (mr *MetricsRegistry) RegisterCollector(collector MetricCollector) {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	mr.collectors = append(mr.collectors, collector)
	mr.logger.Debug("Registered metric collector")
}

// GetAllMetrics returns all current metrics
func (mr *MetricsRegistry) GetAllMetrics() []Metric {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	var metrics []Metric
	now := time.Now()

	// Collect counter metrics
	for _, counter := range mr.counters {
		metrics = append(metrics, Metric{
			Name:      counter.name,
			Type:      MetricTypeCounter,
			Value:     float64(atomic.LoadInt64(&counter.value)),
			Labels:    counter.labels,
			Timestamp: now,
			Help:      counter.help,
		})
	}

	// Collect gauge metrics
	for _, gauge := range mr.gauges {
		gauge.mu.RLock()
		metrics = append(metrics, Metric{
			Name:      gauge.name,
			Type:      MetricTypeGauge,
			Value:     float64(gauge.value),
			Labels:    gauge.labels,
			Timestamp: now,
			Help:      gauge.help,
		})
		gauge.mu.RUnlock()
	}

	// Collect histogram metrics
	for _, histogram := range mr.histograms {
		histogram.mu.RLock()
		
		// Add bucket metrics
		for i, bucket := range histogram.buckets {
			bucketLabels := make(map[string]string)
			for k, v := range histogram.labels {
				bucketLabels[k] = v
			}
			bucketLabels["le"] = fmt.Sprintf("%.3f", bucket)
			
			metrics = append(metrics, Metric{
				Name:      histogram.name + "_bucket",
				Type:      MetricTypeCounter,
				Value:     float64(histogram.counts[i]),
				Labels:    bucketLabels,
				Timestamp: now,
				Help:      histogram.help,
			})
		}
		
		// Add +Inf bucket
		infLabels := make(map[string]string)
		for k, v := range histogram.labels {
			infLabels[k] = v
		}
		infLabels["le"] = "+Inf"
		
		metrics = append(metrics, Metric{
			Name:      histogram.name + "_bucket",
			Type:      MetricTypeCounter,
			Value:     float64(histogram.count),
			Labels:    infLabels,
			Timestamp: now,
			Help:      histogram.help,
		})
		
		// Add sum and count
		metrics = append(metrics, Metric{
			Name:      histogram.name + "_sum",
			Type:      MetricTypeCounter,
			Value:     histogram.sum,
			Labels:    histogram.labels,
			Timestamp: now,
			Help:      histogram.help,
		})
		
		metrics = append(metrics, Metric{
			Name:      histogram.name + "_count",
			Type:      MetricTypeCounter,
			Value:     float64(histogram.count),
			Labels:    histogram.labels,
			Timestamp: now,
			Help:      histogram.help,
		})
		
		histogram.mu.RUnlock()
	}

	// Collect from custom collectors
	for _, collector := range mr.collectors {
		collectorMetrics := collector.Collect()
		metrics = append(metrics, collectorMetrics...)
	}

	return metrics
}

// metricKey generates a unique key for a metric
func (mr *MetricsRegistry) metricKey(name string, labels map[string]string) string {
	key := name
	for k, v := range labels {
		key += fmt.Sprintf("_%s_%s", k, v)
	}
	return key
}

// Counter methods

// Inc increments the counter by 1
func (c *CounterMetric) Inc() {
	atomic.AddInt64(&c.value, 1)
}

// Add adds the given value to the counter
func (c *CounterMetric) Add(value float64) {
	atomic.AddInt64(&c.value, int64(value))
}

// Get returns the current counter value
func (c *CounterMetric) Get() float64 {
	return float64(atomic.LoadInt64(&c.value))
}

// Gauge methods

// Set sets the gauge to the given value
func (g *GaugeMetric) Set(value float64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.value = int64(value)
}

// Inc increments the gauge by 1
func (g *GaugeMetric) Inc() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.value++
}

// Dec decrements the gauge by 1
func (g *GaugeMetric) Dec() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.value--
}

// Add adds the given value to the gauge
func (g *GaugeMetric) Add(value float64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.value += int64(value)
}

// Sub subtracts the given value from the gauge
func (g *GaugeMetric) Sub(value float64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.value -= int64(value)
}

// Get returns the current gauge value
func (g *GaugeMetric) Get() float64 {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return float64(g.value)
}

// Histogram methods

// Observe adds an observation to the histogram
func (h *HistogramMetric) Observe(value float64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Find the appropriate bucket
	for i, bucket := range h.buckets {
		if value <= bucket {
			h.counts[i]++
		}
	}

	// Update sum and count
	h.sum += value
	h.count++
}

// GetCount returns the total number of observations
func (h *HistogramMetric) GetCount() int64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.count
}

// GetSum returns the sum of all observations
func (h *HistogramMetric) GetSum() float64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.sum
}

// GetBuckets returns the bucket counts
func (h *HistogramMetric) GetBuckets() []int64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	buckets := make([]int64, len(h.counts))
	copy(buckets, h.counts)
	return buckets
}

// ConnectionPoolCollector implements MetricCollector for connection pool metrics
type ConnectionPoolCollector struct {
	registry *MetricsRegistry
	
	activeConnections   *GaugeMetric
	totalConnections    *CounterMetric
	rejectedConnections *CounterMetric
	queuedRequests      *GaugeMetric
	queueTime          *HistogramMetric
}

// NewConnectionPoolCollector creates a new connection pool metrics collector
func NewConnectionPoolCollector(registry *MetricsRegistry) *ConnectionPoolCollector {
	collector := &ConnectionPoolCollector{
		registry: registry,
	}

	// Register metrics
	collector.activeConnections = registry.RegisterGauge(
		"session_pool_active_connections",
		"Number of active connections in the pool",
		nil)

	collector.totalConnections = registry.RegisterCounter(
		"session_pool_total_connections",
		"Total number of connections created",
		nil)

	collector.rejectedConnections = registry.RegisterCounter(
		"session_pool_rejected_connections",
		"Total number of rejected connections",
		nil)

	collector.queuedRequests = registry.RegisterGauge(
		"session_pool_queued_requests",
		"Number of requests waiting in queue",
		nil)

	collector.queueTime = registry.RegisterHistogram(
		"session_pool_queue_time_seconds",
		"Time spent waiting in connection queue",
		nil,
		nil)

	return collector
}

// UpdateMetrics updates the connection pool metrics
func (cpc *ConnectionPoolCollector) UpdateMetrics(activeConns, totalConns, rejectedConns, queuedReqs int64, avgQueueTime time.Duration) {
	cpc.activeConnections.Set(float64(activeConns))
	cpc.totalConnections.Add(float64(totalConns))
	cpc.rejectedConnections.Add(float64(rejectedConns))
	cpc.queuedRequests.Set(float64(queuedReqs))
	
	if avgQueueTime > 0 {
		cpc.queueTime.Observe(avgQueueTime.Seconds())
	}
}

// Collect implements MetricCollector
func (cpc *ConnectionPoolCollector) Collect() []Metric {
	// Metrics are automatically collected by the registry
	return []Metric{}
}

// Describe implements MetricCollector
func (cpc *ConnectionPoolCollector) Describe() []MetricDescription {
	return []MetricDescription{
		{Name: "session_pool_active_connections", Type: MetricTypeGauge, Help: "Number of active connections in the pool"},
		{Name: "session_pool_total_connections", Type: MetricTypeCounter, Help: "Total number of connections created"},
		{Name: "session_pool_rejected_connections", Type: MetricTypeCounter, Help: "Total number of rejected connections"},
		{Name: "session_pool_queued_requests", Type: MetricTypeGauge, Help: "Number of requests waiting in queue"},
		{Name: "session_pool_queue_time_seconds", Type: MetricTypeHistogram, Help: "Time spent waiting in connection queue"},
	}
}

// Monitoring routines

// collectionRoutine collects metrics periodically
func (mr *MetricsRegistry) collectionRoutine(ctx context.Context) {
	defer mr.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	mr.logger.Debug("Metrics collection routine started")

	for {
		select {
		case <-ticker.C:
			mr.collectMetrics()
		case <-ctx.Done():
			mr.logger.Debug("Metrics collection routine stopped due to context cancellation")
			return
		case <-mr.shutdownChan:
			mr.logger.Debug("Metrics collection routine stopped")
			return
		}
	}
}

// collectMetrics triggers metric collection from all collectors
func (mr *MetricsRegistry) collectMetrics() {
	mr.mu.RLock()
	collectors := make([]MetricCollector, len(mr.collectors))
	copy(collectors, mr.collectors)
	mr.mu.RUnlock()

	for _, collector := range collectors {
		metrics := collector.Collect()
		if len(metrics) > 0 {
			mr.logger.Debug("Collected metrics from collector", "metric_count", len(metrics))
		}
	}
}

// exportRoutine exports metrics periodically
func (mr *MetricsRegistry) exportRoutine(ctx context.Context) {
	defer mr.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	mr.logger.Debug("Metrics export routine started")

	for {
		select {
		case <-ticker.C:
			mr.exportMetrics()
		case <-ctx.Done():
			mr.logger.Debug("Metrics export routine stopped due to context cancellation")
			return
		case <-mr.shutdownChan:
			mr.logger.Debug("Metrics export routine stopped")
			return
		}
	}
}

// exportMetrics logs current metrics
func (mr *MetricsRegistry) exportMetrics() {
	metrics := mr.GetAllMetrics()
	
	mr.logger.Info("Metrics summary",
		"total_metrics", len(metrics),
		"counters", len(mr.counters),
		"gauges", len(mr.gauges),
		"histograms", len(mr.histograms),
		"collectors", len(mr.collectors))

	// Log key metrics
	for _, metric := range metrics {
		if metric.Value > 0 {
			mr.logger.Debug("Metric value",
				"name", metric.Name,
				"type", metric.Type.String(),
				"value", metric.Value,
				"labels", metric.Labels)
		}
	}
}

// String method for MetricType
func (mt MetricType) String() string {
	switch mt {
	case MetricTypeCounter:
		return "Counter"
	case MetricTypeGauge:
		return "Gauge"
	case MetricTypeHistogram:
		return "Histogram"
	case MetricTypeSummary:
		return "Summary"
	default:
		return "Unknown"
	}
}