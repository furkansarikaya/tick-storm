package server

import (
	"sync"
	"sync/atomic"
	"time"
)

// PerformanceMetrics tracks server performance metrics
type PerformanceMetrics struct {
	// Connection metrics
	ActiveConnections     int64
	TotalConnections      int64
	IPRejectedConnections int64
	SlowClientsDetected   int64
	WriteQueueFullErrors  int64
	
	// Message metrics
	MessagesSentTotal     int64
	MessagesRecvTotal     int64
	BytesSentTotal        int64
	BytesRecvTotal        int64
	
	// Performance metrics
	WriteLatencyNs        int64 // Average write latency in nanoseconds
	WriteLatencyCount     int64
	WriteTimeouts         int64
	WriteDeadlineExceeded int64
	
	// Object pool metrics
	FramePoolHits         int64
	FramePoolMisses       int64
	BufferPoolHits        int64
	BufferPoolMisses      int64
	
	// Heartbeat metrics
	HeartbeatTimeouts     int64
	HeartbeatsSent        int64
	HeartbeatsRecv        int64
	
	mu sync.RWMutex
}

// GlobalMetrics is the global metrics instance
var GlobalMetrics = &PerformanceMetrics{}

// IncrementActiveConnections increments active connection count
func (m *PerformanceMetrics) IncrementActiveConnections() {
	atomic.AddInt64(&m.ActiveConnections, 1)
	atomic.AddInt64(&m.TotalConnections, 1)
}

// DecrementActiveConnections decrements active connection count
func (m *PerformanceMetrics) DecrementActiveConnections() {
	atomic.AddInt64(&m.ActiveConnections, -1)
}

// IncrementSlowClients increments slow client detection count
func (m *PerformanceMetrics) IncrementSlowClients() {
	atomic.AddInt64(&m.SlowClientsDetected, 1)
}

// IncrementWriteQueueFull increments write queue full error count
func (m *PerformanceMetrics) IncrementWriteQueueFull() {
	atomic.AddInt64(&m.WriteQueueFullErrors, 1)
}

// IncrementIPRejectedConnections increments rejected connections due to IP filtering
func (m *PerformanceMetrics) IncrementIPRejectedConnections() {
	atomic.AddInt64(&m.IPRejectedConnections, 1)
}

// AddMessagesSent adds to messages sent count
func (m *PerformanceMetrics) AddMessagesSent(count int64) {
	atomic.AddInt64(&m.MessagesSentTotal, count)
}

// AddMessagesRecv adds to messages received count
func (m *PerformanceMetrics) AddMessagesRecv(count int64) {
	atomic.AddInt64(&m.MessagesRecvTotal, count)
}

// AddBytesSent adds to bytes sent count
func (m *PerformanceMetrics) AddBytesSent(count int64) {
	atomic.AddInt64(&m.BytesSentTotal, count)
}

// AddBytesRecv adds to bytes received count
func (m *PerformanceMetrics) AddBytesRecv(count int64) {
	atomic.AddInt64(&m.BytesRecvTotal, count)
}

// RecordWriteLatency records write latency measurement
func (m *PerformanceMetrics) RecordWriteLatency(latencyNs int64) {
	// Simple moving average calculation
	count := atomic.AddInt64(&m.WriteLatencyCount, 1)
	currentAvg := atomic.LoadInt64(&m.WriteLatencyNs)
	
	// Calculate new average: newAvg = (oldAvg * (count-1) + newValue) / count
	newAvg := (currentAvg*(count-1) + latencyNs) / count
	atomic.StoreInt64(&m.WriteLatencyNs, newAvg)
}

// IncrementWriteTimeouts increments write timeout count
func (m *PerformanceMetrics) IncrementWriteTimeouts() {
	atomic.AddInt64(&m.WriteTimeouts, 1)
}

// IncrementWriteDeadlineExceeded increments write deadline exceeded count
func (m *PerformanceMetrics) IncrementWriteDeadlineExceeded() {
	atomic.AddInt64(&m.WriteDeadlineExceeded, 1)
}

// IncrementFramePoolHits increments frame pool hit count
func (m *PerformanceMetrics) IncrementFramePoolHits() {
	atomic.AddInt64(&m.FramePoolHits, 1)
}

// IncrementFramePoolMisses increments frame pool miss count
func (m *PerformanceMetrics) IncrementFramePoolMisses() {
	atomic.AddInt64(&m.FramePoolMisses, 1)
}

// IncrementBufferPoolHits increments buffer pool hit count
func (m *PerformanceMetrics) IncrementBufferPoolHits() {
	atomic.AddInt64(&m.BufferPoolHits, 1)
}

// IncrementBufferPoolMisses increments buffer pool miss count
func (m *PerformanceMetrics) IncrementBufferPoolMisses() {
	atomic.AddInt64(&m.BufferPoolMisses, 1)
}

// IncrementHeartbeatTimeouts increments heartbeat timeout count
func (m *PerformanceMetrics) IncrementHeartbeatTimeouts() {
	atomic.AddInt64(&m.HeartbeatTimeouts, 1)
}

// IncrementHeartbeatsSent increments heartbeats sent count
func (m *PerformanceMetrics) IncrementHeartbeatsSent() {
	atomic.AddInt64(&m.HeartbeatsSent, 1)
}

// IncrementHeartbeatsRecv increments heartbeats received count
func (m *PerformanceMetrics) IncrementHeartbeatsRecv() {
	atomic.AddInt64(&m.HeartbeatsRecv, 1)
}

// GetSnapshot returns a snapshot of current metrics
func (m *PerformanceMetrics) GetSnapshot() map[string]interface{} {
	return map[string]interface{}{
		"active_connections":        atomic.LoadInt64(&m.ActiveConnections),
		"total_connections":         atomic.LoadInt64(&m.TotalConnections),
		"ip_rejected_connections":   atomic.LoadInt64(&m.IPRejectedConnections),
		"slow_clients_detected":     atomic.LoadInt64(&m.SlowClientsDetected),
		"write_queue_full_errors":   atomic.LoadInt64(&m.WriteQueueFullErrors),
		"messages_sent_total":       atomic.LoadInt64(&m.MessagesSentTotal),
		"messages_recv_total":       atomic.LoadInt64(&m.MessagesRecvTotal),
		"bytes_sent_total":          atomic.LoadInt64(&m.BytesSentTotal),
		"bytes_recv_total":          atomic.LoadInt64(&m.BytesRecvTotal),
		"write_latency_ns":          atomic.LoadInt64(&m.WriteLatencyNs),
		"write_latency_count":       atomic.LoadInt64(&m.WriteLatencyCount),
		"write_timeouts":            atomic.LoadInt64(&m.WriteTimeouts),
		"write_deadline_exceeded":   atomic.LoadInt64(&m.WriteDeadlineExceeded),
		"frame_pool_hits":           atomic.LoadInt64(&m.FramePoolHits),
		"frame_pool_misses":         atomic.LoadInt64(&m.FramePoolMisses),
		"buffer_pool_hits":          atomic.LoadInt64(&m.BufferPoolHits),
		"buffer_pool_misses":        atomic.LoadInt64(&m.BufferPoolMisses),
		"heartbeat_timeouts":        atomic.LoadInt64(&m.HeartbeatTimeouts),
		"heartbeats_sent":           atomic.LoadInt64(&m.HeartbeatsSent),
		"heartbeats_recv":           atomic.LoadInt64(&m.HeartbeatsRecv),
	}
}

// Reset resets all metrics to zero
func (m *PerformanceMetrics) Reset() {
	atomic.StoreInt64(&m.ActiveConnections, 0)
	atomic.StoreInt64(&m.TotalConnections, 0)
	atomic.StoreInt64(&m.IPRejectedConnections, 0)
	atomic.StoreInt64(&m.SlowClientsDetected, 0)
	atomic.StoreInt64(&m.WriteQueueFullErrors, 0)
	atomic.StoreInt64(&m.MessagesSentTotal, 0)
	atomic.StoreInt64(&m.MessagesRecvTotal, 0)
	atomic.StoreInt64(&m.BytesSentTotal, 0)
	atomic.StoreInt64(&m.BytesRecvTotal, 0)
	atomic.StoreInt64(&m.WriteLatencyNs, 0)
	atomic.StoreInt64(&m.WriteLatencyCount, 0)
	atomic.StoreInt64(&m.WriteTimeouts, 0)
	atomic.StoreInt64(&m.WriteDeadlineExceeded, 0)
	atomic.StoreInt64(&m.FramePoolHits, 0)
	atomic.StoreInt64(&m.FramePoolMisses, 0)
	atomic.StoreInt64(&m.BufferPoolHits, 0)
	atomic.StoreInt64(&m.BufferPoolMisses, 0)
	atomic.StoreInt64(&m.HeartbeatTimeouts, 0)
	atomic.StoreInt64(&m.HeartbeatsSent, 0)
	atomic.StoreInt64(&m.HeartbeatsRecv, 0)
}

// PerformanceMonitor provides performance monitoring functionality
type PerformanceMonitor struct {
	metrics       *PerformanceMetrics
	sampleTicker  *time.Ticker
	stopCh        chan struct{}
	
	// Performance thresholds
	MaxWriteLatencyMs     int64
	MaxSlowClientRatio    float64
	MaxWriteQueueFullRate float64
}

// NewPerformanceMonitor creates a new performance monitor
func NewPerformanceMonitor(metrics *PerformanceMetrics) *PerformanceMonitor {
	return &PerformanceMonitor{
		metrics:               metrics,
		stopCh:               make(chan struct{}),
		MaxWriteLatencyMs:    10,    // 10ms max write latency
		MaxSlowClientRatio:   0.05,  // 5% max slow client ratio
		MaxWriteQueueFullRate: 0.01, // 1% max write queue full rate
	}
}

// Start starts the performance monitor
func (pm *PerformanceMonitor) Start(sampleInterval time.Duration) {
	pm.sampleTicker = time.NewTicker(sampleInterval)
	
	go func() {
		for {
			select {
			case <-pm.sampleTicker.C:
				pm.checkPerformance()
			case <-pm.stopCh:
				return
			}
		}
	}()
}

// Stop stops the performance monitor
func (pm *PerformanceMonitor) Stop() {
	if pm.sampleTicker != nil {
		pm.sampleTicker.Stop()
	}
	close(pm.stopCh)
}

// checkPerformance checks performance metrics against thresholds
func (pm *PerformanceMonitor) checkPerformance() {
	snapshot := pm.metrics.GetSnapshot()
	
	// Check write latency
	writeLatencyNs := snapshot["write_latency_ns"].(int64)
	writeLatencyMs := writeLatencyNs / 1_000_000
	if writeLatencyMs > pm.MaxWriteLatencyMs {
		// Log warning about high write latency
	}
	
	// Check slow client ratio
	totalConns := snapshot["total_connections"].(int64)
	slowClients := snapshot["slow_clients_detected"].(int64)
	if totalConns > 0 {
		slowClientRatio := float64(slowClients) / float64(totalConns)
		if slowClientRatio > pm.MaxSlowClientRatio {
			// Log warning about high slow client ratio
		}
	}
	
	// Check write queue full rate
	writeQueueFullErrors := snapshot["write_queue_full_errors"].(int64)
	messagesSent := snapshot["messages_sent_total"].(int64)
	if messagesSent > 0 {
		writeQueueFullRate := float64(writeQueueFullErrors) / float64(messagesSent)
		if writeQueueFullRate > pm.MaxWriteQueueFullRate {
			// Log warning about high write queue full rate
		}
	}
}
