// Package server implements simplified Prometheus metrics collection for Tick-Storm TCP server.
package server

import (
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PrometheusMetrics contains all Prometheus metrics for the server
type PrometheusMetrics struct {
	// Connection metrics
	activeConnections     *prometheus.CounterVec
	totalConnections      *prometheus.CounterVec
	connectionDuration    *prometheus.CounterVec
	connectionErrors      *prometheus.CounterVec
	
	// Message metrics
	messagesSentTotal     *prometheus.CounterVec
	messagesRecvTotal     *prometheus.CounterVec
	bytesSentTotal        *prometheus.CounterVec
	bytesRecvTotal        *prometheus.CounterVec
	
	// Performance metrics
	publishLatency        prometheus.Histogram
	writeLatency          prometheus.Histogram
	messageProcessingDuration prometheus.Histogram
	writeTimeouts         prometheus.Counter
	writeDeadlineExceeded prometheus.Counter
	
	// Authentication metrics
	authSuccess           *prometheus.CounterVec
	authFailures          *prometheus.CounterVec
	authRateLimited       prometheus.Counter
	
	// Heartbeat metrics
	heartbeatTimeouts     prometheus.Counter
	heartbeatSent         *prometheus.CounterVec
	heartbeatsRecv        prometheus.Counter
	
	// Error metrics
	errorsByType          *prometheus.CounterVec
	protocolErrors        *prometheus.CounterVec
	
	// Resource metrics
	memoryUsage           prometheus.Gauge
	goroutineCount        prometheus.Gauge
	gcDuration            prometheus.Histogram
	
	// Business metrics
	subscriptionCount     *prometheus.GaugeVec
	messagesSent          *prometheus.GaugeVec
	revenue               *prometheus.GaugeVec
	clientsByRegion       *prometheus.GaugeVec
	
	// Pool metrics
	framePoolHits         prometheus.Counter
	framePoolMisses       prometheus.Counter
	bufferPoolHits        prometheus.Counter
	bufferPoolMisses      prometheus.Counter
	
	registry *prometheus.Registry
}

// NewPrometheusMetrics creates a new Prometheus metrics instance
func NewPrometheusMetrics() *PrometheusMetrics {
	pm := &PrometheusMetrics{
		registry: prometheus.NewRegistry(),
		
		// Connection metrics
		activeConnections: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "tick_storm_active_connections_total",
				Help: "Number of active connections",
			},
			[]string{"instance_id"},
		),
		totalConnections: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "tick_storm_total_connections_total",
				Help: "Total number of connections processed",
			},
			[]string{"instance_id"},
		),
		connectionDuration: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "tick_storm_connection_duration_seconds_total",
				Help: "Total connection duration in seconds",
			},
			[]string{"instance_id"},
		),
		connectionErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "tick_storm_connection_errors_total",
				Help: "Number of connection errors",
			},
			[]string{"instance_id", "error_type"},
		),
		
		// Message metrics
		messagesSentTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "tick_storm_messages_sent_total",
			Help: "Total messages sent by type",
		}, []string{"message_type", "subscription_mode"}),
		messagesRecvTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "tick_storm_messages_recv_total",
			Help: "Total messages received by type",
		}, []string{"message_type"}),
		bytesSentTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "tick_storm_bytes_sent_total",
			Help: "Total bytes sent",
		}, []string{"connection_type"}),
		bytesRecvTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "tick_storm_bytes_recv_total",
			Help: "Total bytes received",
		}, []string{"connection_type"}),
		
		// Performance metrics
		publishLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "tick_storm_publish_latency_seconds",
			Help:    "Latency of publish operations in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
		}),
		writeLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "tick_storm_write_latency_seconds",
			Help:    "Write latency in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
		}),
		messageProcessingDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "tick_storm_message_processing_duration_seconds",
			Help:    "Message processing duration in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
		}),
		writeTimeouts: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "tick_storm_write_timeouts_total",
			Help: "Total write timeouts",
		}),
		writeDeadlineExceeded: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "tick_storm_write_deadline_exceeded_total",
			Help: "Total write deadline exceeded errors",
		}),
		
		// Authentication metrics
		authSuccess: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "tick_storm_auth_success_total",
				Help: "Number of successful authentications",
			},
			[]string{"instance_id"},
		),
		authFailures: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "tick_storm_auth_failures_total",
				Help: "Number of authentication failures",
			},
			[]string{"instance_id", "reason"},
		),
		authRateLimited: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "tick_storm_auth_rate_limited_total",
			Help: "Total rate limited authentication attempts",
		}),
		
		// Heartbeat metrics
		heartbeatTimeouts: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "tick_storm_heartbeat_timeouts_total",
			Help: "Total heartbeat timeouts",
		}),
		heartbeatSent: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "tick_storm_heartbeat_sent_total",
				Help: "Number of heartbeats sent",
			},
			[]string{"instance_id"},
		),
		heartbeatsRecv: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "tick_storm_heartbeats_recv_total",
			Help: "Total heartbeats received",
		}),
		
		// Error metrics
		errorsByType: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "tick_storm_errors_total",
			Help: "Total errors by type",
		}, []string{"error_type", "error_code"}),
		protocolErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "tick_storm_protocol_errors_total",
				Help: "Number of protocol errors",
			},
			[]string{"instance_id", "error_type"},
		),
		
		// Resource metrics
		memoryUsage: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "tick_storm_memory_usage_bytes",
			Help: "Current memory usage in bytes",
		}),
		goroutineCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "tick_storm_goroutines",
			Help: "Current number of goroutines",
		}),
		gcDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "tick_storm_gc_duration_seconds",
			Help:    "Garbage collection duration in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
		}),
		
		// Business metrics
		subscriptionCount: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "tick_storm_subscriptions_current",
				Help: "Current number of subscriptions",
			},
			[]string{"instance_id", "symbol"},
		),
		messagesSent: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "tick_storm_messages_sent_total",
				Help: "Total messages sent to clients",
			},
			[]string{"instance_id", "symbol"},
		),
		revenue: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "tick_storm_revenue_total",
				Help: "Total revenue generated",
			},
			[]string{"instance_id"},
		),
		clientsByRegion: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "tick_storm_clients_by_region",
			Help: "Number of clients by region",
		}, []string{"region"}),
		
		// Pool metrics
		framePoolHits: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "tick_storm_frame_pool_hits_total",
			Help: "Total frame pool hits",
		}),
		framePoolMisses: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "tick_storm_frame_pool_misses_total",
			Help: "Total frame pool misses",
		}),
		bufferPoolHits: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "tick_storm_buffer_pool_hits_total",
			Help: "Total buffer pool hits",
		}),
		bufferPoolMisses: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "tick_storm_buffer_pool_misses_total",
			Help: "Total buffer pool misses",
		}),
	}
	
	pm.registerMetrics()
	return pm
}

// registerMetrics registers all metrics with the registry
func (pm *PrometheusMetrics) registerMetrics() {
	// Connection metrics
	pm.registry.MustRegister(pm.activeConnections)
	pm.registry.MustRegister(pm.totalConnections)
	pm.registry.MustRegister(pm.connectionDuration)
	pm.registry.MustRegister(pm.connectionErrors)
	
	// Message metrics
	pm.registry.MustRegister(pm.messagesSentTotal)
	pm.registry.MustRegister(pm.messagesRecvTotal)
	pm.registry.MustRegister(pm.bytesSentTotal)
	pm.registry.MustRegister(pm.bytesRecvTotal)
	
	// Performance metrics
	pm.registry.MustRegister(pm.publishLatency)
	pm.registry.MustRegister(pm.writeLatency)
	pm.registry.MustRegister(pm.messageProcessingDuration)
	pm.registry.MustRegister(pm.writeTimeouts)
	pm.registry.MustRegister(pm.writeDeadlineExceeded)
	
	// Authentication metrics
	pm.registry.MustRegister(pm.authSuccess)
	pm.registry.MustRegister(pm.authFailures)
	pm.registry.MustRegister(pm.authRateLimited)
	
	// Heartbeat metrics
	pm.registry.MustRegister(pm.heartbeatTimeouts)
	pm.registry.MustRegister(pm.heartbeatSent)
	pm.registry.MustRegister(pm.heartbeatsRecv)
	
	// Error metrics
	pm.registry.MustRegister(pm.errorsByType)
	pm.registry.MustRegister(pm.protocolErrors)
	
	// Resource metrics
	pm.registry.MustRegister(pm.memoryUsage)
	pm.registry.MustRegister(pm.goroutineCount)
	pm.registry.MustRegister(pm.gcDuration)
	
	// Business metrics
	pm.registry.MustRegister(pm.subscriptionCount)
	pm.registry.MustRegister(pm.messagesSent)
	pm.registry.MustRegister(pm.revenue)
	pm.registry.MustRegister(pm.clientsByRegion)
	
	// Pool metrics
	pm.registry.MustRegister(pm.framePoolHits)
	pm.registry.MustRegister(pm.framePoolMisses)
	pm.registry.MustRegister(pm.bufferPoolHits)
	pm.registry.MustRegister(pm.bufferPoolMisses)
}

// Connection metric methods
func (pm *PrometheusMetrics) IncrementActiveConnections(instanceID string) {
	pm.activeConnections.WithLabelValues(instanceID).Inc()
	pm.totalConnections.WithLabelValues(instanceID).Inc()
}

func (pm *PrometheusMetrics) DecrementActiveConnections(instanceID string) {
	// Note: Prometheus counters can't be decremented, so we just track total
	// Active connections should be tracked as a gauge in production
}

func (pm *PrometheusMetrics) RecordConnectionDuration(instanceID string, duration time.Duration) {
	pm.connectionDuration.WithLabelValues(instanceID).Add(duration.Seconds())
}

func (pm *PrometheusMetrics) IncrementConnectionError(instanceID, errorType string) {
	pm.connectionErrors.WithLabelValues(instanceID, errorType).Inc()
}

// Message metric methods
func (pm *PrometheusMetrics) IncrementMessagesSent(messageType, subscriptionMode string) {
	pm.messagesSentTotal.WithLabelValues(messageType, subscriptionMode).Inc()
}

func (pm *PrometheusMetrics) IncrementMessagesRecv(messageType string) {
	pm.messagesRecvTotal.WithLabelValues(messageType).Inc()
}

func (pm *PrometheusMetrics) IncrementBytesSent(connectionType string, bytes int64) {
	pm.bytesSentTotal.WithLabelValues(connectionType).Add(float64(bytes))
}

func (pm *PrometheusMetrics) IncrementBytesRecv(connectionType string, bytes int64) {
	pm.bytesRecvTotal.WithLabelValues(connectionType).Add(float64(bytes))
}

// Performance metric methods
func (pm *PrometheusMetrics) RecordPublishLatency(latency time.Duration) {
	pm.publishLatency.Observe(float64(latency.Nanoseconds()) / 1e6) // Convert to milliseconds
}

func (pm *PrometheusMetrics) RecordWriteLatency(latency time.Duration) {
	pm.writeLatency.Observe(float64(latency.Nanoseconds()) / 1e6) // Convert to milliseconds
}

func (pm *PrometheusMetrics) IncrementWriteTimeouts() {
	pm.writeTimeouts.Inc()
}

func (pm *PrometheusMetrics) UpdatePerformanceMetrics(duration time.Duration) {
	pm.messageProcessingDuration.Observe(float64(duration.Nanoseconds()) / 1e6) // Convert to milliseconds
	pm.goroutineCount.Set(float64(runtime.NumGoroutine()))
	
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	pm.memoryUsage.Set(float64(memStats.Alloc))
	pm.gcDuration.Observe(float64(memStats.PauseTotalNs) / 1e6) // Convert to milliseconds
}

func (pm *PrometheusMetrics) IncrementWriteDeadlineExceeded() {
	pm.writeDeadlineExceeded.Inc()
}

// Authentication metric methods
func (pm *PrometheusMetrics) IncrementAuthSuccess(instanceID string) {
	pm.authSuccess.WithLabelValues(instanceID).Inc()
}

func (pm *PrometheusMetrics) IncrementAuthFailure(instanceID, reason string) {
	pm.authFailures.WithLabelValues(instanceID, reason).Inc()
}

func (pm *PrometheusMetrics) IncrementAuthRateLimited() {
	pm.authRateLimited.Inc()
}

// Heartbeat metric methods
func (pm *PrometheusMetrics) IncrementHeartbeatTimeouts() {
	pm.heartbeatTimeouts.Inc()
}

func (pm *PrometheusMetrics) IncrementHeartbeatSent(instanceID string) {
	pm.heartbeatSent.WithLabelValues(instanceID).Inc()
}

func (pm *PrometheusMetrics) IncrementHeartbeatsRecv() {
	pm.heartbeatsRecv.Inc()
}

// Error metric methods
func (pm *PrometheusMetrics) IncrementError(errorType, errorCode string) {
	pm.errorsByType.WithLabelValues(errorType, errorCode).Inc()
}

func (pm *PrometheusMetrics) IncrementProtocolError(instanceID, errorType string) {
	pm.protocolErrors.WithLabelValues(instanceID, errorType).Inc()
}

// Resource metric methods
func (pm *PrometheusMetrics) UpdateResourceMetrics() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	pm.memoryUsage.Set(float64(m.Alloc))
	pm.goroutineCount.Set(float64(runtime.NumGoroutine()))
}

func (pm *PrometheusMetrics) RecordGCDuration(duration time.Duration) {
	pm.gcDuration.Observe(duration.Seconds())
}

// Business metric methods
func (pm *PrometheusMetrics) SetSubscriptionCount(instanceID, symbol string, count int) {
	pm.subscriptionCount.WithLabelValues(instanceID, symbol).Set(float64(count))
}

func (pm *PrometheusMetrics) SetMessagesSent(instanceID, symbol string, count int) {
	pm.messagesSent.WithLabelValues(instanceID, symbol).Set(float64(count))
}

func (pm *PrometheusMetrics) SetRevenue(instanceID string, revenue float64) {
	pm.revenue.WithLabelValues(instanceID).Set(revenue)
}

func (pm *PrometheusMetrics) SetClientsByRegion(region string, count int) {
	pm.clientsByRegion.WithLabelValues(region).Set(float64(count))
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

// StartMetricsServer starts the Prometheus metrics HTTP server
func (pm *PrometheusMetrics) StartMetricsServer(port int) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(pm.registry, promhttp.HandlerOpts{}))
	
	addr := fmt.Sprintf(":%d", port)
	return http.ListenAndServe(addr, mux)
}

// StartMetricsCollector starts periodic metrics collection
func (pm *PrometheusMetrics) StartMetricsCollector(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			pm.UpdateResourceMetrics()
		}
	}()
}
