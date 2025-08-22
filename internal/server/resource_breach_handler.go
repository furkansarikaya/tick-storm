// Package server implements graceful handling of resource limit breaches.
package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"runtime"
	"sync/atomic"
	"time"
)

// ResourceBreachHandler manages graceful degradation when resource limits are exceeded
type ResourceBreachHandler struct {
	logger           *slog.Logger
	resourceMonitor  *ResourceMonitor
	
	// Breach state tracking
	memoryBreach     atomic.Bool
	fdBreach         atomic.Bool
	goroutineBreach  atomic.Bool
	connectionBreach atomic.Bool
	
	// Graceful degradation settings
	enableGracefulDegradation atomic.Bool
	rejectNewConnections     atomic.Bool
	
	// Metrics
	connectionsRejected uint64
	degradationEvents   uint64
}

// NewResourceBreachHandler creates a new resource breach handler
func NewResourceBreachHandler(logger *slog.Logger, monitor *ResourceMonitor) *ResourceBreachHandler {
	handler := &ResourceBreachHandler{
		logger:          logger,
		resourceMonitor: monitor,
	}
	
	// Enable graceful degradation by default
	handler.enableGracefulDegradation.Store(true)
	
	return handler
}

// CheckResourceLimits evaluates current resource usage and triggers breach handling
func (rbh *ResourceBreachHandler) CheckResourceLimits() {
	if rbh.resourceMonitor == nil {
		return
	}
	
	usage := rbh.resourceMonitor.GetCurrentUsage()
	
	// Check memory usage
	if usage.MemoryUsagePercent > 90.0 {
		if !rbh.memoryBreach.Load() {
			rbh.handleMemoryBreach(usage.MemoryUsagePercent)
		}
	} else if rbh.memoryBreach.Load() && usage.MemoryUsagePercent < 80.0 {
		rbh.clearMemoryBreach()
	}
	
	// Check file descriptor usage
	if usage.FDUsagePercent > 90.0 {
		if !rbh.fdBreach.Load() {
			rbh.handleFDBreach(usage.FDUsagePercent)
		}
	} else if rbh.fdBreach.Load() && usage.FDUsagePercent < 80.0 {
		rbh.clearFDBreach()
	}
	
	// Check goroutine count
	if usage.GoroutineCount > 50000 {
		if !rbh.goroutineBreach.Load() {
			rbh.handleGoroutineBreach(usage.GoroutineCount)
		}
	} else if rbh.goroutineBreach.Load() && usage.GoroutineCount < 40000 {
		rbh.clearGoroutineBreach()
	}
	
	// Check connection count
	if usage.ActiveConnections > 95000 {
		if !rbh.connectionBreach.Load() {
			rbh.handleConnectionBreach(usage.ActiveConnections)
		}
	} else if rbh.connectionBreach.Load() && usage.ActiveConnections < 90000 {
		rbh.clearConnectionBreach()
	}
}

// ShouldRejectConnection determines if new connections should be rejected
func (rbh *ResourceBreachHandler) ShouldRejectConnection() bool {
	return rbh.rejectNewConnections.Load()
}

// GetRejectionReason returns the reason for connection rejection
func (rbh *ResourceBreachHandler) GetRejectionReason() string {
	if rbh.memoryBreach.Load() {
		return "server memory limit exceeded"
	}
	if rbh.fdBreach.Load() {
		return "server file descriptor limit exceeded"
	}
	if rbh.connectionBreach.Load() {
		return "server connection limit exceeded"
	}
	if rbh.goroutineBreach.Load() {
		return "server goroutine limit exceeded"
	}
	return "server resource limit exceeded"
}

// RejectConnection handles rejecting a new connection gracefully
func (rbh *ResourceBreachHandler) RejectConnection(conn net.Conn) {
	atomic.AddUint64(&rbh.connectionsRejected, 1)
	
	// Send a brief error message before closing
	reason := rbh.GetRejectionReason()
	errorMsg := fmt.Sprintf("503 Service Unavailable: %s\n", reason)
	
	// Set a short write timeout
	conn.SetWriteDeadline(time.Now().Add(1 * time.Second))
	conn.Write([]byte(errorMsg))
	conn.Close()
	
	rbh.logger.Warn("rejected connection due to resource breach",
		"reason", reason,
		"remote_addr", conn.RemoteAddr().String())
}

// handleMemoryBreach handles memory usage breach
func (rbh *ResourceBreachHandler) handleMemoryBreach(usage float64) {
	rbh.memoryBreach.Store(true)
	rbh.rejectNewConnections.Store(true)
	atomic.AddUint64(&rbh.degradationEvents, 1)
	
	rbh.logger.Error("memory usage breach detected - enabling graceful degradation",
		"memory_usage_percent", usage,
		"action", "rejecting_new_connections")
	
	// Trigger garbage collection to free memory
	runtime.GC()
}

// clearMemoryBreach clears memory breach state
func (rbh *ResourceBreachHandler) clearMemoryBreach() {
	rbh.memoryBreach.Store(false)
	rbh.checkAllBreaches()
	
	rbh.logger.Info("memory usage returned to normal - breach cleared")
}

// handleFDBreach handles file descriptor usage breach
func (rbh *ResourceBreachHandler) handleFDBreach(usage float64) {
	rbh.fdBreach.Store(true)
	rbh.rejectNewConnections.Store(true)
	atomic.AddUint64(&rbh.degradationEvents, 1)
	
	rbh.logger.Error("file descriptor usage breach detected - enabling graceful degradation",
		"fd_usage_percent", usage,
		"action", "rejecting_new_connections")
}

// clearFDBreach clears file descriptor breach state
func (rbh *ResourceBreachHandler) clearFDBreach() {
	rbh.fdBreach.Store(false)
	rbh.checkAllBreaches()
	
	rbh.logger.Info("file descriptor usage returned to normal - breach cleared")
}

// handleGoroutineBreach handles goroutine count breach
func (rbh *ResourceBreachHandler) handleGoroutineBreach(count int) {
	rbh.goroutineBreach.Store(true)
	rbh.rejectNewConnections.Store(true)
	atomic.AddUint64(&rbh.degradationEvents, 1)
	
	rbh.logger.Error("goroutine count breach detected - enabling graceful degradation",
		"goroutine_count", count,
		"action", "rejecting_new_connections")
}

// clearGoroutineBreach clears goroutine breach state
func (rbh *ResourceBreachHandler) clearGoroutineBreach() {
	rbh.goroutineBreach.Store(false)
	rbh.checkAllBreaches()
	
	rbh.logger.Info("goroutine count returned to normal - breach cleared")
}

// handleConnectionBreach handles connection count breach
func (rbh *ResourceBreachHandler) handleConnectionBreach(count int) {
	rbh.connectionBreach.Store(true)
	rbh.rejectNewConnections.Store(true)
	atomic.AddUint64(&rbh.degradationEvents, 1)
	
	rbh.logger.Error("connection count breach detected - enabling graceful degradation",
		"connection_count", count,
		"action", "rejecting_new_connections")
}

// clearConnectionBreach clears connection breach state
func (rbh *ResourceBreachHandler) clearConnectionBreach() {
	rbh.connectionBreach.Store(false)
	rbh.checkAllBreaches()
	
	rbh.logger.Info("connection count returned to normal - breach cleared")
}

// checkAllBreaches checks if any breaches are still active
func (rbh *ResourceBreachHandler) checkAllBreaches() {
	hasAnyBreach := rbh.memoryBreach.Load() || 
		rbh.fdBreach.Load() || 
		rbh.goroutineBreach.Load() || 
		rbh.connectionBreach.Load()
	
	if !hasAnyBreach {
		rbh.rejectNewConnections.Store(false)
		rbh.logger.Info("all resource breaches cleared - accepting new connections")
	}
}

// GetBreachStats returns current breach statistics
func (rbh *ResourceBreachHandler) GetBreachStats() map[string]interface{} {
	return map[string]interface{}{
		"memory_breach":        rbh.memoryBreach.Load(),
		"fd_breach":           rbh.fdBreach.Load(),
		"goroutine_breach":    rbh.goroutineBreach.Load(),
		"connection_breach":   rbh.connectionBreach.Load(),
		"rejecting_connections": rbh.rejectNewConnections.Load(),
		"connections_rejected":  atomic.LoadUint64(&rbh.connectionsRejected),
		"degradation_events":   atomic.LoadUint64(&rbh.degradationEvents),
	}
}

// StartMonitoring starts the resource breach monitoring loop
func (rbh *ResourceBreachHandler) StartMonitoring(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	rbh.logger.Info("started resource breach monitoring")
	
	for {
		select {
		case <-ctx.Done():
			rbh.logger.Info("stopped resource breach monitoring")
			return
		case <-ticker.C:
			rbh.CheckResourceLimits()
		}
	}
}
