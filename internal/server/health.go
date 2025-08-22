// Package server implements health check functionality for load balancers.
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"sync/atomic"
	"time"
)

// HealthStatus represents the health status of the server
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// HealthCheck represents the health check response
type HealthCheck struct {
	Status            HealthStatus           `json:"status"`
	Timestamp         time.Time              `json:"timestamp"`
	Version           string                 `json:"version"`
	InstanceID        string                 `json:"instance_id"`
	Uptime            time.Duration          `json:"uptime"`
	ActiveConnections int32                  `json:"active_connections"`
	TotalConnections  uint64                 `json:"total_connections"`
	MemoryUsage       MemoryStats            `json:"memory_usage"`
	ResourceStatus    map[string]interface{} `json:"resource_status"`
	Checks            map[string]CheckResult `json:"checks"`
}

// CheckResult represents the result of an individual health check
type CheckResult struct {
	Status  HealthStatus `json:"status"`
	Message string       `json:"message"`
	Details interface{}  `json:"details,omitempty"`
}

// MemoryStats represents memory usage statistics
type MemoryStats struct {
	AllocMB      uint64 `json:"alloc_mb"`
	TotalAllocMB uint64 `json:"total_alloc_mb"`
	SysMB        uint64 `json:"sys_mb"`
	NumGC        uint32 `json:"num_gc"`
	Goroutines   int    `json:"goroutines"`
}

// HealthChecker provides health check functionality
type HealthChecker struct {
	server    *Server
	startTime time.Time
	version   string
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(server *Server) *HealthChecker {
	return &HealthChecker{
		server:    server,
		startTime: time.Now(),
		version:   server.GetVersion(),
	}
}

// GetHealth returns the current health status
func (hc *HealthChecker) GetHealth() *HealthCheck {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	health := &HealthCheck{
		Status:            hc.determineOverallStatus(),
		Timestamp:         time.Now(),
		Version:           hc.version,
		InstanceID:        hc.server.GetInstanceID(),
		Uptime:            time.Since(hc.startTime),
		ActiveConnections: atomic.LoadInt32(&hc.server.activeConns),
		TotalConnections:  atomic.LoadUint64(&hc.server.totalConns),
		MemoryUsage: MemoryStats{
			AllocMB:      bToMb(m.Alloc),
			TotalAllocMB: bToMb(m.TotalAlloc),
			SysMB:        bToMb(m.Sys),
			NumGC:        m.NumGC,
			Goroutines:   runtime.NumGoroutine(),
		},
		Checks: make(map[string]CheckResult),
	}

	// Add resource status if available
	if hc.server.breachHandler != nil {
		health.ResourceStatus = hc.server.breachHandler.GetBreachStats()
	}

	// Perform individual health checks
	hc.checkServerStatus(health)
	hc.checkResourceLimits(health)
	hc.checkConnectivity(health)
	hc.checkAuthentication(health)

	return health
}

// determineOverallStatus determines the overall health status
func (hc *HealthChecker) determineOverallStatus() HealthStatus {
	// Check if server is closed
	if hc.server.closed.Load() {
		return HealthStatusUnhealthy
	}

	// Check resource breach status
	if hc.server.breachHandler != nil && hc.server.breachHandler.ShouldRejectConnection() {
		return HealthStatusDegraded
	}

	// Check connection limits
	activeConns := atomic.LoadInt32(&hc.server.activeConns)
	maxConns := int32(hc.server.config.MaxConnections)
	
	if maxConns > 0 {
		usage := float64(activeConns) / float64(maxConns)
		if usage > 0.9 {
			return HealthStatusDegraded
		}
	}

	return HealthStatusHealthy
}

// checkServerStatus checks basic server status
func (hc *HealthChecker) checkServerStatus(health *HealthCheck) {
	if hc.server.closed.Load() {
		health.Checks["server"] = CheckResult{
			Status:  HealthStatusUnhealthy,
			Message: "Server is closed",
		}
		return
	}

	if hc.server.listener == nil {
		health.Checks["server"] = CheckResult{
			Status:  HealthStatusUnhealthy,
			Message: "Server listener not initialized",
		}
		return
	}

	health.Checks["server"] = CheckResult{
		Status:  HealthStatusHealthy,
		Message: "Server is running",
		Details: map[string]interface{}{
			"listen_addr": hc.server.config.ListenAddr,
			"uptime":      time.Since(hc.startTime),
		},
	}
}

// checkResourceLimits checks resource usage limits
func (hc *HealthChecker) checkResourceLimits(health *HealthCheck) {
	if hc.server.breachHandler == nil {
		health.Checks["resources"] = CheckResult{
			Status:  HealthStatusHealthy,
			Message: "Resource monitoring not enabled",
		}
		return
	}

	breachStats := hc.server.breachHandler.GetBreachStats()
	
	// Check if any resource breaches are active
	memoryBreach := breachStats["memory_breach"].(bool)
	fdBreach := breachStats["fd_breach"].(bool)
	goroutineBreach := breachStats["goroutine_breach"].(bool)
	connectionBreach := breachStats["connection_breach"].(bool)

	if memoryBreach || fdBreach || goroutineBreach || connectionBreach {
		health.Checks["resources"] = CheckResult{
			Status:  HealthStatusDegraded,
			Message: "Resource limits exceeded",
			Details: breachStats,
		}
		return
	}

	health.Checks["resources"] = CheckResult{
		Status:  HealthStatusHealthy,
		Message: "Resource usage within limits",
		Details: breachStats,
	}
}

// checkConnectivity checks network connectivity
func (hc *HealthChecker) checkConnectivity(health *HealthCheck) {
	activeConns := atomic.LoadInt32(&hc.server.activeConns)
	maxConns := int32(hc.server.config.MaxConnections)

	status := HealthStatusHealthy
	message := "Connection capacity available"

	if maxConns > 0 {
		usage := float64(activeConns) / float64(maxConns)
		if usage > 0.9 {
			status = HealthStatusDegraded
			message = "Connection capacity nearly exhausted"
		} else if usage > 0.8 {
			status = HealthStatusDegraded
			message = "High connection usage"
		}
	}

	health.Checks["connectivity"] = CheckResult{
		Status:  status,
		Message: message,
		Details: map[string]interface{}{
			"active_connections": activeConns,
			"max_connections":    maxConns,
			"usage_percent":      float64(activeConns) / float64(maxConns) * 100,
		},
	}
}

// checkAuthentication checks authentication system health
func (hc *HealthChecker) checkAuthentication(health *HealthCheck) {
	if hc.server.authenticator == nil {
		health.Checks["authentication"] = CheckResult{
			Status:  HealthStatusUnhealthy,
			Message: "Authentication system not initialized",
		}
		return
	}

	authSuccess := atomic.LoadUint64(&hc.server.authSuccess)
	authFailures := atomic.LoadUint64(&hc.server.authFailures)
	authRateLimited := atomic.LoadUint64(&hc.server.authRateLimited)

	total := authSuccess + authFailures + authRateLimited
	status := HealthStatusHealthy
	message := "Authentication system operational"

	if total > 0 {
		failureRate := float64(authFailures) / float64(total)
		if failureRate > 0.5 {
			status = HealthStatusDegraded
			message = "High authentication failure rate"
		}
	}

	health.Checks["authentication"] = CheckResult{
		Status:  status,
		Message: message,
		Details: map[string]interface{}{
			"auth_success":      authSuccess,
			"auth_failures":     authFailures,
			"auth_rate_limited": authRateLimited,
		},
	}
}

// ServeHTTP implements http.Handler for health check endpoint
func (hc *HealthChecker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	health := hc.GetHealth()

	// Set appropriate HTTP status code based on health
	switch health.Status {
	case HealthStatusHealthy:
		w.WriteHeader(http.StatusOK)
	case HealthStatusDegraded:
		w.WriteHeader(http.StatusOK) // Still accepting traffic
	case HealthStatusUnhealthy:
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// IsHealthy returns true if the server is healthy
func (hc *HealthChecker) IsHealthy() bool {
	return hc.determineOverallStatus() != HealthStatusUnhealthy
}

// bToMb converts bytes to megabytes
func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

// StartHealthCheckServer starts an HTTP server for health checks
func (s *Server) StartHealthCheckServer(port int) error {
	if s.healthChecker == nil {
		s.healthChecker = NewHealthChecker(s)
	}

	mux := http.NewServeMux()
	mux.Handle("/health", s.healthChecker)
	mux.Handle("/healthz", s.healthChecker) // Kubernetes style
	mux.Handle("/ready", s.healthChecker)   // Readiness probe

	// Simple ping endpoint
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("pong"))
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("health check server failed", "error", err)
		}
	}()

	s.logger.Info("health check server started", "port", port)
	return nil
}
