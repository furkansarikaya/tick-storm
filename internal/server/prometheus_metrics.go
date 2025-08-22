package server

import (
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	contentTypeHeader = "Content-Type"
	prometheusContentType = "text/plain; version=0.0.4; charset=utf-8"
)

// PrometheusMetrics holds all Prometheus metrics for the server.
type PrometheusMetrics struct {
	// Connection metrics
	activeConnections    *prometheus.GaugeVec
	totalConnections     *prometheus.CounterVec
	connectionDuration   *prometheus.HistogramVec
	connectionErrors     *prometheus.CounterVec
	
	// Message metrics
	messagesSentTotal    *prometheus.CounterVec
	messagesRecvTotal    *prometheus.CounterVec
	bytesSentTotal       *prometheus.CounterVec
	bytesRecvTotal       *prometheus.CounterVec
	
	// Performance metrics
	publishLatency       prometheus.Histogram
	writeLatency         prometheus.Histogram
	messageProcessingDuration prometheus.Histogram
	writeTimeouts        prometheus.Counter
	writeDeadlineExceeded prometheus.Counter
	
	// Authentication metrics
	authSuccess          *prometheus.CounterVec
	authFailures         *prometheus.CounterVec
	authRateLimited      prometheus.Counter
	
	// Heartbeat metrics
	heartbeatTimeouts    prometheus.Counter
	heartbeatSent        *prometheus.CounterVec
	heartbeatsRecv       prometheus.Counter
	
	// Error metrics
	errorsByType         *prometheus.CounterVec
	protocolErrors       *prometheus.CounterVec
	
	// Resource metrics
	memoryUsage          prometheus.Gauge
	goroutineCount       prometheus.Gauge
	gcDuration           prometheus.Histogram
	
	// Business metrics
	subscriptionCount    *prometheus.GaugeVec
	messagesSent         *prometheus.CounterVec
	
	// Pool metrics
	framePoolHits        prometheus.Counter
	framePoolMisses      prometheus.Counter
	bufferPoolHits       prometheus.Counter
	bufferPoolMisses     prometheus.Counter
	
	registry *prometheus.Registry
}

// NewPrometheusMetrics creates a new PrometheusMetrics instance.
func NewPrometheusMetrics() *PrometheusMetrics {
	pm := &PrometheusMetrics{
		registry: prometheus.NewRegistry(),
	}
	
	pm.initializeMetrics()
	pm.registerMetrics()
	
	return pm
}

// NewPrometheusMetricsWithRegistry creates a new PrometheusMetrics instance with a custom registry.
func NewPrometheusMetricsWithRegistry(registry *prometheus.Registry) *PrometheusMetrics {
	pm := &PrometheusMetrics{
		registry: registry,
	}
	
	pm.initializeMetrics()
	pm.registerMetrics()
	
	return pm
}

func (pm *PrometheusMetrics) initializeMetrics() {
	// Connection metrics
	pm.activeConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tick_storm_active_connections",
			Help: "Number of active connections",
		},
		[]string{"instance_id"},
	)
	
	pm.totalConnections = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tick_storm_total_connections_total",
			Help: "Total number of connections processed",
		},
		[]string{"instance_id"},
	)
	
	pm.connectionDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tick_storm_connection_duration_seconds",
			Help:    "Connection duration in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
		},
		[]string{"instance_id"},
	)
	
	pm.connectionErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tick_storm_connection_errors_total",
			Help: "Number of connection errors",
		},
		[]string{"instance_id", "error_type"},
	)
	
	// Message metrics
	pm.messagesSentTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tick_storm_messages_sent_total",
			Help: "Total messages sent by type",
		},
		[]string{"message_type", "subscription_mode"},
	)
	
	pm.messagesRecvTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tick_storm_messages_recv_total",
			Help: "Total messages received by type",
		},
		[]string{"message_type"},
	)
	
	pm.bytesSentTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tick_storm_bytes_sent_total",
			Help: "Total bytes sent",
		},
		[]string{"connection_type"},
	)
	
	pm.bytesRecvTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tick_storm_bytes_recv_total",
			Help: "Total bytes received",
		},
		[]string{"connection_type"},
	)
	
	// Performance metrics
	pm.publishLatency = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "tick_storm_publish_latency_seconds",
			Help:    "Latency of publish operations in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
		},
	)
	
	pm.writeLatency = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "tick_storm_write_latency_seconds",
			Help:    "Write latency in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
		},
	)
	
	pm.messageProcessingDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "tick_storm_message_processing_duration_seconds",
			Help:    "Message processing duration in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
		},
	)
	
	pm.writeTimeouts = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "tick_storm_write_timeouts_total",
			Help: "Total write timeouts",
		},
	)
	
	pm.writeDeadlineExceeded = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "tick_storm_write_deadline_exceeded_total",
			Help: "Total write deadline exceeded errors",
		},
	)
	
	// Authentication metrics
	pm.authSuccess = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tick_storm_auth_success_total",
			Help: "Number of successful authentications",
		},
		[]string{"instance_id"},
	)
	
	pm.authFailures = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tick_storm_auth_failures_total",
			Help: "Number of authentication failures",
		},
		[]string{"instance_id", "reason"},
	)
	
	pm.authRateLimited = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "tick_storm_auth_rate_limited_total",
			Help: "Total rate limited authentication attempts",
		},
	)
	
	// Heartbeat metrics
	pm.heartbeatTimeouts = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "tick_storm_heartbeat_timeouts_total",
			Help: "Total heartbeat timeouts",
		},
	)
	
	pm.heartbeatSent = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tick_storm_heartbeat_sent_total",
			Help: "Number of heartbeats sent",
		},
		[]string{"instance_id"},
	)
	
	pm.heartbeatsRecv = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "tick_storm_heartbeats_recv_total",
			Help: "Total heartbeats received",
		},
	)
	
	// Error metrics
	pm.errorsByType = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tick_storm_errors_total",
			Help: "Total errors by type",
		},
		[]string{"error_type", "error_code"},
	)
	
	pm.protocolErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tick_storm_protocol_errors_total",
			Help: "Number of protocol errors",
		},
		[]string{"instance_id", "error_type"},
	)
	
	// Resource metrics
	pm.memoryUsage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "tick_storm_memory_usage_bytes",
			Help: "Current memory usage in bytes",
		},
	)
	
	pm.goroutineCount = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "tick_storm_goroutines",
			Help: "Current number of goroutines",
		},
	)
	
	pm.gcDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "tick_storm_gc_duration_seconds",
			Help:    "Garbage collection duration in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
		},
	)
	
	// Business metrics
	pm.subscriptionCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tick_storm_subscriptions_current",
			Help: "Current number of subscriptions",
		},
		[]string{"instance_id", "symbol"},
	)
	
	pm.messagesSent = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tick_storm_business_messages_sent_total",
			Help: "Total messages sent to clients",
		},
		[]string{"instance_id", "symbol"},
	)
	
	// Pool metrics
	pm.framePoolHits = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "tick_storm_frame_pool_hits_total",
			Help: "Total frame pool hits",
		},
	)
	
	pm.framePoolMisses = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "tick_storm_frame_pool_misses_total",
			Help: "Total frame pool misses",
		},
	)
	
	pm.bufferPoolHits = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "tick_storm_buffer_pool_hits_total",
			Help: "Total buffer pool hits",
		},
	)
	
	pm.bufferPoolMisses = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "tick_storm_buffer_pool_misses_total",
			Help: "Total buffer pool misses",
		},
	)
}

func (pm *PrometheusMetrics) registerMetrics() {
	pm.registry.MustRegister(
		pm.activeConnections,
		pm.totalConnections,
		pm.connectionDuration,
		pm.connectionErrors,
		pm.messagesSentTotal,
		pm.messagesRecvTotal,
		pm.bytesSentTotal,
		pm.bytesRecvTotal,
		pm.publishLatency,
		pm.writeLatency,
		pm.messageProcessingDuration,
		pm.writeTimeouts,
		pm.writeDeadlineExceeded,
		pm.authSuccess,
		pm.authFailures,
		pm.authRateLimited,
		pm.heartbeatTimeouts,
		pm.heartbeatSent,
		pm.heartbeatsRecv,
		pm.errorsByType,
		pm.protocolErrors,
		pm.memoryUsage,
		pm.goroutineCount,
		pm.gcDuration,
		pm.subscriptionCount,
		pm.messagesSent,
		pm.framePoolHits,
		pm.framePoolMisses,
		pm.bufferPoolHits,
		pm.bufferPoolMisses,
	)
}

// Connection metric methods
func (pm *PrometheusMetrics) IncrementActiveConnections(instanceID string) {
	pm.activeConnections.WithLabelValues(instanceID).Inc()
}

func (pm *PrometheusMetrics) DecrementActiveConnections(instanceID string) {
	pm.activeConnections.WithLabelValues(instanceID).Dec()
}

func (pm *PrometheusMetrics) IncrementTotalConnections(instanceID string) {
	pm.totalConnections.WithLabelValues(instanceID).Inc()
}

func (pm *PrometheusMetrics) RecordConnectionDuration(instanceID string, duration time.Duration) {
	pm.connectionDuration.WithLabelValues(instanceID).Observe(duration.Seconds())
}

func (pm *PrometheusMetrics) IncrementConnectionErrors(instanceID, errorType string) {
	pm.connectionErrors.WithLabelValues(instanceID, errorType).Inc()
}

// Authentication metric methods
func (pm *PrometheusMetrics) IncrementAuthSuccess(instanceID string) {
	pm.authSuccess.WithLabelValues(instanceID).Inc()
}

func (pm *PrometheusMetrics) IncrementAuthFailure(instanceID, reason string) {
	pm.authFailures.WithLabelValues(instanceID, reason).Inc()
}

func (pm *PrometheusMetrics) IncrementAuthRateLimited(instanceID string) {
	pm.authRateLimited.Inc()
}

// Message metric methods
func (pm *PrometheusMetrics) IncrementMessagesSent(messageType, subscriptionMode string) {
	pm.messagesSentTotal.WithLabelValues(messageType, subscriptionMode).Inc()
}

func (pm *PrometheusMetrics) IncrementMessagesReceived(messageType string) {
	pm.messagesRecvTotal.WithLabelValues(messageType).Inc()
}

func (pm *PrometheusMetrics) AddBytesSent(connectionType string, bytes int64) {
	pm.bytesSentTotal.WithLabelValues(connectionType).Add(float64(bytes))
}

func (pm *PrometheusMetrics) AddBytesReceived(connectionType string, bytes int64) {
	pm.bytesRecvTotal.WithLabelValues(connectionType).Add(float64(bytes))
}

// Performance metric methods
func (pm *PrometheusMetrics) RecordPublishLatency(duration time.Duration) {
	pm.publishLatency.Observe(duration.Seconds())
}

func (pm *PrometheusMetrics) RecordWriteLatency(duration time.Duration) {
	pm.writeLatency.Observe(duration.Seconds())
}

func (pm *PrometheusMetrics) RecordMessageProcessingDuration(duration time.Duration) {
	pm.messageProcessingDuration.Observe(duration.Seconds())
}

func (pm *PrometheusMetrics) IncrementWriteTimeouts() {
	pm.writeTimeouts.Inc()
}

func (pm *PrometheusMetrics) IncrementWriteDeadlineExceeded() {
	pm.writeDeadlineExceeded.Inc()
}

// Heartbeat metric methods
func (pm *PrometheusMetrics) IncrementHeartbeatTimeouts() {
	pm.heartbeatTimeouts.Inc()
}

func (pm *PrometheusMetrics) IncrementHeartbeatSent(instanceID string) {
	pm.heartbeatSent.WithLabelValues(instanceID).Inc()
}

func (pm *PrometheusMetrics) IncrementHeartbeatsReceived() {
	pm.heartbeatsRecv.Inc()
}

// Error metric methods
func (pm *PrometheusMetrics) IncrementErrorsByType(errorType, errorCode string) {
	pm.errorsByType.WithLabelValues(errorType, errorCode).Inc()
}

func (pm *PrometheusMetrics) IncrementProtocolErrors(instanceID, errorType string) {
	pm.protocolErrors.WithLabelValues(instanceID, errorType).Inc()
}

// Resource metric methods
func (pm *PrometheusMetrics) UpdateMemoryUsage(bytes uint64) {
	pm.memoryUsage.Set(float64(bytes))
}

func (pm *PrometheusMetrics) UpdateGoroutineCount(count int) {
	pm.goroutineCount.Set(float64(count))
}

func (pm *PrometheusMetrics) RecordGCDuration(duration time.Duration) {
	pm.gcDuration.Observe(duration.Seconds())
}

// Business metric methods
func (pm *PrometheusMetrics) SetSubscriptionCount(instanceID, symbol string, count int) {
	pm.subscriptionCount.WithLabelValues(instanceID, symbol).Set(float64(count))
}

func (pm *PrometheusMetrics) IncrementBusinessMessagesSent(instanceID, symbol string) {
	pm.messagesSent.WithLabelValues(instanceID, symbol).Inc()
}

// Pool metric methods
func (pm *PrometheusMetrics) IncrementFramePoolHits() {
	pm.framePoolHits.Inc()
}

func (pm *PrometheusMetrics) IncrementFramePoolMisses() {
	pm.framePoolMisses.Inc()
}

func (pm *PrometheusMetrics) IncrementBufferPoolHits() {
	pm.bufferPoolHits.Inc()
}

func (pm *PrometheusMetrics) IncrementBufferPoolMisses() {
	pm.bufferPoolMisses.Inc()
}

// StartMetricsServer starts the Prometheus metrics HTTP server.
func (pm *PrometheusMetrics) StartMetricsServer(port int) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(pm.registry, promhttp.HandlerOpts{}))
	
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}
	
	return server.ListenAndServe()
}

// StartMetricsCollector starts collecting system metrics periodically.
func (pm *PrometheusMetrics) StartMetricsCollector() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		
		for range ticker.C {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			
			pm.UpdateMemoryUsage(m.Alloc)
			pm.UpdateGoroutineCount(runtime.NumGoroutine())
		}
	}()
}
