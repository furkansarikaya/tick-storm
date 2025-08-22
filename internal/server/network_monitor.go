// Package server implements network monitoring and alerting for the TCP server.
package server

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// NetworkMonitor provides comprehensive network monitoring and alerting
type NetworkMonitor struct {
	// Monitoring state
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	
	// Alert thresholds
	maxConnectionsPerSecond   int64
	maxFailedConnectionsRate  float64
	maxPortScanAttemptsPerMin int64
	
	// Metrics tracking
	connectionAttempts        uint64
	failedConnections         uint64
	portScanAttempts          uint64
	lastAlertTime             time.Time
	alertCooldown             time.Duration
	
	// Alert callbacks
	alertHandlers []AlertHandler
	logger        *slog.Logger
	
	mutex sync.RWMutex
}

// AlertLevel represents the severity of an alert
type AlertLevel int

const (
	AlertLevelInfo AlertLevel = iota
	AlertLevelWarning
	AlertLevelCritical
)

// NetworkAlert represents a network security alert
type NetworkAlert struct {
	Level     AlertLevel
	Type      string
	Message   string
	Timestamp time.Time
	Metadata  map[string]interface{}
}

// AlertHandler defines the interface for handling network alerts
type AlertHandler interface {
	HandleAlert(alert NetworkAlert)
}

// LogAlertHandler logs alerts using structured logging
type LogAlertHandler struct {
	logger *slog.Logger
}

// NewNetworkMonitor creates a new network monitor
func NewNetworkMonitor() *NetworkMonitor {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &NetworkMonitor{
		ctx:                       ctx,
		cancel:                    cancel,
		maxConnectionsPerSecond:   1000,  // Alert if > 1000 connections/sec
		maxFailedConnectionsRate:  0.5,   // Alert if > 50% connections fail
		maxPortScanAttemptsPerMin: 100,   // Alert if > 100 port scans/min
		alertCooldown:             5 * time.Minute,
		logger:                    slog.Default().With("component", "network_monitor"),
		alertHandlers:             []AlertHandler{},
	}
}

// NewLogAlertHandler creates a new log-based alert handler
func NewLogAlertHandler(logger *slog.Logger) *LogAlertHandler {
	return &LogAlertHandler{logger: logger}
}

// HandleAlert implements AlertHandler for logging
func (h *LogAlertHandler) HandleAlert(alert NetworkAlert) {
	level := slog.LevelInfo
	switch alert.Level {
	case AlertLevelWarning:
		level = slog.LevelWarn
	case AlertLevelCritical:
		level = slog.LevelError
	}
	
	h.logger.Log(context.Background(), level, alert.Message,
		"alert_type", alert.Type,
		"timestamp", alert.Timestamp,
		"metadata", alert.Metadata,
	)
}

// AddAlertHandler adds an alert handler
func (nm *NetworkMonitor) AddAlertHandler(handler AlertHandler) {
	nm.mutex.Lock()
	defer nm.mutex.Unlock()
	
	nm.alertHandlers = append(nm.alertHandlers, handler)
}

// Start begins network monitoring
func (nm *NetworkMonitor) Start() {
	nm.wg.Add(1)
	go nm.monitoringLoop()
	
	nm.logger.Info("network monitoring started",
		"max_connections_per_second", nm.maxConnectionsPerSecond,
		"max_failed_connections_rate", nm.maxFailedConnectionsRate,
		"max_port_scan_attempts_per_min", nm.maxPortScanAttemptsPerMin,
	)
}

// Stop stops network monitoring
func (nm *NetworkMonitor) Stop() {
	nm.cancel()
	nm.wg.Wait()
	nm.logger.Info("network monitoring stopped")
}

// RecordConnectionAttempt records a connection attempt
func (nm *NetworkMonitor) RecordConnectionAttempt(success bool) {
	atomic.AddUint64(&nm.connectionAttempts, 1)
	if !success {
		atomic.AddUint64(&nm.failedConnections, 1)
	}
}

// RecordPortScanAttempt records a port scanning attempt
func (nm *NetworkMonitor) RecordPortScanAttempt() {
	atomic.AddUint64(&nm.portScanAttempts, 1)
}

// monitoringLoop runs the main monitoring loop
func (nm *NetworkMonitor) monitoringLoop() {
	defer nm.wg.Done()
	
	ticker := time.NewTicker(10 * time.Second) // Check every 10 seconds
	defer ticker.Stop()
	
	var lastConnAttempts, lastFailedConns, lastPortScans uint64
	
	for {
		select {
		case <-nm.ctx.Done():
			return
		case <-ticker.C:
			nm.checkMetrics(&lastConnAttempts, &lastFailedConns, &lastPortScans)
		}
	}
}

// checkMetrics analyzes current metrics and triggers alerts if needed
func (nm *NetworkMonitor) checkMetrics(lastConnAttempts, lastFailedConns, lastPortScans *uint64) {
	currentConnAttempts := atomic.LoadUint64(&nm.connectionAttempts)
	currentFailedConns := atomic.LoadUint64(&nm.failedConnections)
	currentPortScans := atomic.LoadUint64(&nm.portScanAttempts)
	
	// Calculate rates (per 10 seconds, so multiply by 6 for per minute)
	connAttemptsRate := (currentConnAttempts - *lastConnAttempts) * 6
	failedConnsRate := (currentFailedConns - *lastFailedConns) * 6
	portScansRate := (currentPortScans - *lastPortScans) * 6
	
	now := time.Now()
	
	// Check connection rate
	if int64(connAttemptsRate) > nm.maxConnectionsPerSecond*60 { // Convert to per minute
		nm.triggerAlert(NetworkAlert{
			Level:     AlertLevelWarning,
			Type:      "high_connection_rate",
			Message:   fmt.Sprintf("High connection rate detected: %d connections/min", connAttemptsRate),
			Timestamp: now,
			Metadata: map[string]interface{}{
				"connections_per_minute": connAttemptsRate,
				"threshold":              nm.maxConnectionsPerSecond * 60,
			},
		})
	}
	
	// Check failed connection rate
	if currentConnAttempts > *lastConnAttempts {
		failureRate := float64(failedConnsRate) / float64(connAttemptsRate)
		if failureRate > nm.maxFailedConnectionsRate {
			nm.triggerAlert(NetworkAlert{
				Level:     AlertLevelCritical,
				Type:      "high_failure_rate",
				Message:   fmt.Sprintf("High connection failure rate: %.2f%%", failureRate*100),
				Timestamp: now,
				Metadata: map[string]interface{}{
					"failure_rate":     failureRate,
					"failed_per_min":   failedConnsRate,
					"total_per_min":    connAttemptsRate,
					"threshold":        nm.maxFailedConnectionsRate,
				},
			})
		}
	}
	
	// Check port scan attempts
	if int64(portScansRate) > nm.maxPortScanAttemptsPerMin {
		nm.triggerAlert(NetworkAlert{
			Level:     AlertLevelCritical,
			Type:      "port_scanning_detected",
			Message:   fmt.Sprintf("High port scanning activity: %d attempts/min", portScansRate),
			Timestamp: now,
			Metadata: map[string]interface{}{
				"port_scans_per_minute": portScansRate,
				"threshold":             nm.maxPortScanAttemptsPerMin,
			},
		})
	}
	
	// Update last values
	*lastConnAttempts = currentConnAttempts
	*lastFailedConns = currentFailedConns
	*lastPortScans = currentPortScans
}

// triggerAlert sends an alert to all registered handlers
func (nm *NetworkMonitor) triggerAlert(alert NetworkAlert) {
	nm.mutex.RLock()
	defer nm.mutex.RUnlock()
	
	// Check cooldown period
	if time.Since(nm.lastAlertTime) < nm.alertCooldown {
		return
	}
	
	nm.lastAlertTime = alert.Timestamp
	
	// Send to all handlers
	for _, handler := range nm.alertHandlers {
		go handler.HandleAlert(alert)
	}
}

// GetMetrics returns current monitoring metrics
func (nm *NetworkMonitor) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"connection_attempts":              atomic.LoadUint64(&nm.connectionAttempts),
		"failed_connections":               atomic.LoadUint64(&nm.failedConnections),
		"port_scan_attempts":               atomic.LoadUint64(&nm.portScanAttempts),
		"max_connections_per_second":       nm.maxConnectionsPerSecond,
		"max_failed_connections_rate":      nm.maxFailedConnectionsRate,
		"max_port_scan_attempts_per_min":   nm.maxPortScanAttemptsPerMin,
		"alert_cooldown_seconds":           nm.alertCooldown.Seconds(),
		"last_alert_time":                  nm.lastAlertTime,
	}
}

// SetThresholds updates monitoring thresholds
func (nm *NetworkMonitor) SetThresholds(maxConnPerSec int64, maxFailureRate float64, maxPortScansPerMin int64) {
	nm.mutex.Lock()
	defer nm.mutex.Unlock()
	
	nm.maxConnectionsPerSecond = maxConnPerSec
	nm.maxFailedConnectionsRate = maxFailureRate
	nm.maxPortScanAttemptsPerMin = maxPortScansPerMin
	
	nm.logger.Info("monitoring thresholds updated",
		"max_connections_per_second", maxConnPerSec,
		"max_failed_connections_rate", maxFailureRate,
		"max_port_scan_attempts_per_min", maxPortScansPerMin,
	)
}
