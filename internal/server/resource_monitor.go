// Package server implements resource monitoring and constraint enforcement for the TCP server.
package server

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// ResourceMonitor provides comprehensive resource monitoring and constraint enforcement
type ResourceMonitor struct {
	// Monitoring state
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	
	// Resource limits
	maxMemoryMB       int64
	maxFileDescriptors int64
	maxGoroutines     int64
	maxConnections    int64
	
	// Current usage tracking
	currentMemoryMB    int64
	currentFDs         int64
	currentGoroutines  int64
	currentConnections int64
	
	// Breach counters
	memoryBreaches     uint64
	fdBreaches         uint64
	goroutineBreaches  uint64
	connectionBreaches uint64
	
	// Alert thresholds (percentage of limit)
	warningThreshold float64
	criticalThreshold float64
	
	// Alert callbacks
	alertHandlers []ResourceAlertHandler
	logger        *slog.Logger
	
	mutex sync.RWMutex
}

// ResourceAlert represents a resource constraint alert
type ResourceAlert struct {
	Type      string
	Level     AlertLevel
	Message   string
	Current   int64
	Limit     int64
	Usage     float64
	Timestamp time.Time
}

// ResourceAlertHandler defines the interface for handling resource alerts
type ResourceAlertHandler interface {
	HandleResourceAlert(alert ResourceAlert)
}

// LogResourceAlertHandler logs resource alerts using structured logging
type LogResourceAlertHandler struct {
	logger *slog.Logger
}

// ResourceLimits holds configuration for resource constraints
type ResourceLimits struct {
	MaxMemoryMB       int64
	MaxFileDescriptors int64
	MaxGoroutines     int64
	MaxConnections    int64
	WarningThreshold  float64
	CriticalThreshold float64
}

// NewResourceMonitor creates a new resource monitor
func NewResourceMonitor(limits ResourceLimits) *ResourceMonitor {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &ResourceMonitor{
		ctx:                ctx,
		cancel:             cancel,
		maxMemoryMB:        limits.MaxMemoryMB,
		maxFileDescriptors: limits.MaxFileDescriptors,
		maxGoroutines:      limits.MaxGoroutines,
		maxConnections:     limits.MaxConnections,
		warningThreshold:   limits.WarningThreshold,
		criticalThreshold:  limits.CriticalThreshold,
		logger:             slog.Default().With("component", "resource_monitor"),
		alertHandlers:      []ResourceAlertHandler{},
	}
}

// NewLogResourceAlertHandler creates a new log-based resource alert handler
func NewLogResourceAlertHandler(logger *slog.Logger) *LogResourceAlertHandler {
	return &LogResourceAlertHandler{logger: logger}
}

// HandleResourceAlert implements ResourceAlertHandler for logging
func (h *LogResourceAlertHandler) HandleResourceAlert(alert ResourceAlert) {
	level := slog.LevelInfo
	switch alert.Level {
	case AlertLevelWarning:
		level = slog.LevelWarn
	case AlertLevelCritical:
		level = slog.LevelError
	}
	
	h.logger.Log(context.Background(), level, alert.Message,
		"resource_type", alert.Type,
		"current_usage", alert.Current,
		"limit", alert.Limit,
		"usage_percentage", fmt.Sprintf("%.1f%%", alert.Usage*100),
		"timestamp", alert.Timestamp,
	)
}

// AddAlertHandler adds a resource alert handler
func (rm *ResourceMonitor) AddAlertHandler(handler ResourceAlertHandler) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()
	
	rm.alertHandlers = append(rm.alertHandlers, handler)
}

// Start begins resource monitoring
func (rm *ResourceMonitor) Start() {
	rm.wg.Add(1)
	go rm.monitoringLoop()
	
	rm.logger.Info("resource monitoring started",
		"max_memory_mb", rm.maxMemoryMB,
		"max_file_descriptors", rm.maxFileDescriptors,
		"max_goroutines", rm.maxGoroutines,
		"max_connections", rm.maxConnections,
		"warning_threshold", rm.warningThreshold,
		"critical_threshold", rm.criticalThreshold,
	)
}

// Stop stops resource monitoring
func (rm *ResourceMonitor) Stop() {
	rm.cancel()
	rm.wg.Wait()
	rm.logger.Info("resource monitoring stopped")
}

// CheckMemoryLimit checks if memory usage is within limits
func (rm *ResourceMonitor) CheckMemoryLimit() bool {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	currentMB := int64(m.Alloc / 1024 / 1024)
	atomic.StoreInt64(&rm.currentMemoryMB, currentMB)
	
	if rm.maxMemoryMB > 0 && currentMB > rm.maxMemoryMB {
		atomic.AddUint64(&rm.memoryBreaches, 1)
		rm.triggerAlert("memory", currentMB, rm.maxMemoryMB)
		return false
	}
	
	// Check warning thresholds
	if rm.maxMemoryMB > 0 {
		usage := float64(currentMB) / float64(rm.maxMemoryMB)
		if usage >= rm.criticalThreshold {
			rm.triggerAlert("memory", currentMB, rm.maxMemoryMB)
		} else if usage >= rm.warningThreshold {
			rm.triggerWarning("memory", currentMB, rm.maxMemoryMB)
		}
	}
	
	return true
}

// CheckFileDescriptorLimit checks if file descriptor usage is within limits
func (rm *ResourceMonitor) CheckFileDescriptorLimit() bool {
	var rLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		rm.logger.Error("failed to get file descriptor limit", "error", err)
		return true
	}
	
	// Estimate current FD usage (this is approximate)
	currentFDs := rm.estimateFileDescriptorUsage()
	atomic.StoreInt64(&rm.currentFDs, currentFDs)
	
	maxFDs := rm.maxFileDescriptors
	if maxFDs == 0 {
		maxFDs = int64(rLimit.Cur)
	}
	
	if currentFDs > maxFDs {
		atomic.AddUint64(&rm.fdBreaches, 1)
		rm.triggerAlert("file_descriptors", currentFDs, maxFDs)
		return false
	}
	
	// Check warning thresholds
	usage := float64(currentFDs) / float64(maxFDs)
	if usage >= rm.criticalThreshold {
		rm.triggerAlert("file_descriptors", currentFDs, maxFDs)
	} else if usage >= rm.warningThreshold {
		rm.triggerWarning("file_descriptors", currentFDs, maxFDs)
	}
	
	return true
}

// CheckGoroutineLimit checks if goroutine count is within limits
func (rm *ResourceMonitor) CheckGoroutineLimit() bool {
	currentGoroutines := int64(runtime.NumGoroutine())
	atomic.StoreInt64(&rm.currentGoroutines, currentGoroutines)
	
	if rm.maxGoroutines > 0 && currentGoroutines > rm.maxGoroutines {
		atomic.AddUint64(&rm.goroutineBreaches, 1)
		rm.triggerAlert("goroutines", currentGoroutines, rm.maxGoroutines)
		return false
	}
	
	// Check warning thresholds
	if rm.maxGoroutines > 0 {
		usage := float64(currentGoroutines) / float64(rm.maxGoroutines)
		if usage >= rm.criticalThreshold {
			rm.triggerAlert("goroutines", currentGoroutines, rm.maxGoroutines)
		} else if usage >= rm.warningThreshold {
			rm.triggerWarning("goroutines", currentGoroutines, rm.maxGoroutines)
		}
	}
	
	return true
}

// CheckConnectionLimit checks if connection count is within limits
func (rm *ResourceMonitor) CheckConnectionLimit(currentConns int64) bool {
	atomic.StoreInt64(&rm.currentConnections, currentConns)
	
	if rm.maxConnections > 0 && currentConns > rm.maxConnections {
		atomic.AddUint64(&rm.connectionBreaches, 1)
		rm.triggerAlert("connections", currentConns, rm.maxConnections)
		return false
	}
	
	// Check warning thresholds
	if rm.maxConnections > 0 {
		usage := float64(currentConns) / float64(rm.maxConnections)
		if usage >= rm.criticalThreshold {
			rm.triggerAlert("connections", currentConns, rm.maxConnections)
		} else if usage >= rm.warningThreshold {
			rm.triggerWarning("connections", currentConns, rm.maxConnections)
		}
	}
	
	return true
}

// monitoringLoop runs the main resource monitoring loop
func (rm *ResourceMonitor) monitoringLoop() {
	defer rm.wg.Done()
	
	ticker := time.NewTicker(5 * time.Second) // Check every 5 seconds
	defer ticker.Stop()
	
	for {
		select {
		case <-rm.ctx.Done():
			return
		case <-ticker.C:
			rm.CheckMemoryLimit()
			rm.CheckFileDescriptorLimit()
			rm.CheckGoroutineLimit()
		}
	}
}

// estimateFileDescriptorUsage provides an estimate of current FD usage
func (rm *ResourceMonitor) estimateFileDescriptorUsage() int64 {
	// This is a rough estimate based on typical server usage
	// In a real implementation, you might want to scan /proc/self/fd or use other methods
	
	// Base FDs: stdin, stdout, stderr, listening socket
	baseFDs := int64(4)
	
	// Add current connections (each connection typically uses 1 FD)
	connectionFDs := atomic.LoadInt64(&rm.currentConnections)
	
	// Add some overhead for other file operations
	overhead := int64(10)
	
	return baseFDs + connectionFDs + overhead
}

// ResourceUsage represents current resource usage statistics
type ResourceUsage struct {
	MemoryUsagePercent float64
	FDUsagePercent     float64
	GoroutineCount     int
	ActiveConnections  int
}

// GetCurrentUsage returns current resource usage statistics
func (rm *ResourceMonitor) GetCurrentUsage() ResourceUsage {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()
	
	usage := ResourceUsage{
		GoroutineCount:    runtime.NumGoroutine(),
		ActiveConnections: int(atomic.LoadInt64(&rm.currentConnections)),
	}
	
	// Calculate memory usage percentage
	if rm.maxMemoryMB > 0 {
		currentMem := atomic.LoadInt64(&rm.currentMemoryMB)
		usage.MemoryUsagePercent = float64(currentMem) / float64(rm.maxMemoryMB) * 100.0
	}
	
	// Calculate FD usage percentage
	if rm.maxFileDescriptors > 0 {
		currentFDs := rm.estimateFileDescriptorUsage()
		usage.FDUsagePercent = float64(currentFDs) / float64(rm.maxFileDescriptors) * 100.0
	}
	
	return usage
}

// triggerAlert sends a critical alert
func (rm *ResourceMonitor) triggerAlert(resourceType string, current, limit int64) {
	usage := float64(current) / float64(limit)
	alert := ResourceAlert{
		Type:      resourceType,
		Level:     AlertLevelCritical,
		Message:   fmt.Sprintf("Resource limit exceeded: %s usage %.1f%% (%d/%d)", resourceType, usage*100, current, limit),
		Current:   current,
		Limit:     limit,
		Usage:     usage,
		Timestamp: time.Now(),
	}
	
	rm.sendAlert(alert)
}

// triggerWarning sends a warning alert
func (rm *ResourceMonitor) triggerWarning(resourceType string, current, limit int64) {
	usage := float64(current) / float64(limit)
	alert := ResourceAlert{
		Type:      resourceType,
		Level:     AlertLevelWarning,
		Message:   fmt.Sprintf("Resource usage warning: %s usage %.1f%% (%d/%d)", resourceType, usage*100, current, limit),
		Current:   current,
		Limit:     limit,
		Usage:     usage,
		Timestamp: time.Now(),
	}
	
	rm.sendAlert(alert)
}

// sendAlert sends an alert to all registered handlers
func (rm *ResourceMonitor) sendAlert(alert ResourceAlert) {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()
	
	// Send to all handlers
	for _, handler := range rm.alertHandlers {
		go handler.HandleResourceAlert(alert)
	}
}

// GetMetrics returns current resource monitoring metrics
func (rm *ResourceMonitor) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"memory_mb_current":        atomic.LoadInt64(&rm.currentMemoryMB),
		"memory_mb_limit":          rm.maxMemoryMB,
		"file_descriptors_current": atomic.LoadInt64(&rm.currentFDs),
		"file_descriptors_limit":   rm.maxFileDescriptors,
		"goroutines_current":       atomic.LoadInt64(&rm.currentGoroutines),
		"goroutines_limit":         rm.maxGoroutines,
		"connections_current":      atomic.LoadInt64(&rm.currentConnections),
		"connections_limit":        rm.maxConnections,
		"memory_breaches":          atomic.LoadUint64(&rm.memoryBreaches),
		"fd_breaches":              atomic.LoadUint64(&rm.fdBreaches),
		"goroutine_breaches":       atomic.LoadUint64(&rm.goroutineBreaches),
		"connection_breaches":      atomic.LoadUint64(&rm.connectionBreaches),
		"warning_threshold":        rm.warningThreshold,
		"critical_threshold":       rm.criticalThreshold,
	}
}

// GetResourceUsage returns current resource usage percentages
func (rm *ResourceMonitor) GetResourceUsage() map[string]float64 {
	usage := make(map[string]float64)
	
	if rm.maxMemoryMB > 0 {
		usage["memory"] = float64(atomic.LoadInt64(&rm.currentMemoryMB)) / float64(rm.maxMemoryMB)
	}
	
	if rm.maxFileDescriptors > 0 {
		usage["file_descriptors"] = float64(atomic.LoadInt64(&rm.currentFDs)) / float64(rm.maxFileDescriptors)
	}
	
	if rm.maxGoroutines > 0 {
		usage["goroutines"] = float64(atomic.LoadInt64(&rm.currentGoroutines)) / float64(rm.maxGoroutines)
	}
	
	if rm.maxConnections > 0 {
		usage["connections"] = float64(atomic.LoadInt64(&rm.currentConnections)) / float64(rm.maxConnections)
	}
	
	return usage
}

// IsResourceAvailable checks if resources are available for new operations
func (rm *ResourceMonitor) IsResourceAvailable() bool {
	// Check if any resource is at critical threshold
	usage := rm.GetResourceUsage()
	
	for _, u := range usage {
		if u >= rm.criticalThreshold {
			return false
		}
	}
	
	return true
}

// SetLimits updates resource limits dynamically
func (rm *ResourceMonitor) SetLimits(limits ResourceLimits) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()
	
	rm.maxMemoryMB = limits.MaxMemoryMB
	rm.maxFileDescriptors = limits.MaxFileDescriptors
	rm.maxGoroutines = limits.MaxGoroutines
	rm.maxConnections = limits.MaxConnections
	rm.warningThreshold = limits.WarningThreshold
	rm.criticalThreshold = limits.CriticalThreshold
	
	rm.logger.Info("resource limits updated",
		"max_memory_mb", limits.MaxMemoryMB,
		"max_file_descriptors", limits.MaxFileDescriptors,
		"max_goroutines", limits.MaxGoroutines,
		"max_connections", limits.MaxConnections,
	)
}
