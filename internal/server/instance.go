// Package server implements instance identification and management for horizontal scaling.
package server

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"runtime"
	"sync/atomic"
	"time"
)

// InstanceInfo contains information about the server instance
type InstanceInfo struct {
	ID        string    `json:"id"`
	Hostname  string    `json:"hostname"`
	StartTime time.Time `json:"start_time"`
	Version   string    `json:"version"`
	GoVersion string    `json:"go_version"`
	Platform  string    `json:"platform"`
}

// generateInstanceID generates a unique instance ID
func generateInstanceID() string {
	// Try to get from environment first (useful for containers)
	if id := os.Getenv("INSTANCE_ID"); id != "" {
		return id
	}

	// Generate random ID
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID
		return fmt.Sprintf("ts-%d", time.Now().UnixNano())
	}
	
	return hex.EncodeToString(bytes)
}

// GetInstanceID returns the server instance ID
func (s *Server) GetInstanceID() string {
	return s.instanceID
}

// GetInstanceInfo returns detailed instance information
func (s *Server) GetInstanceInfo() InstanceInfo {
	hostname, _ := os.Hostname()
	
	return InstanceInfo{
		ID:        s.instanceID,
		Hostname:  hostname,
		StartTime: s.startTime,
		Version:   s.GetVersion(),
		GoVersion: runtime.Version(),
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}

// GetVersion returns the server version
func (s *Server) GetVersion() string {
	// Try to get from environment or build info
	if version := os.Getenv("APP_VERSION"); version != "" {
		return version
	}
	return "1.0.0" // Default version
}

// GetInstanceMetrics returns instance-specific metrics
func (s *Server) GetInstanceMetrics() map[string]interface{} {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	uptime := time.Since(s.startTime)
	
	return map[string]interface{}{
		"instance_id":        s.instanceID,
		"uptime_seconds":     uptime.Seconds(),
		"active_connections": atomic.LoadInt32(&s.activeConns),
		"total_connections":  atomic.LoadUint64(&s.totalConns),
		"auth_success":       atomic.LoadUint64(&s.authSuccess),
		"auth_failures":      atomic.LoadUint64(&s.authFailures),
		"auth_rate_limited":  atomic.LoadUint64(&s.authRateLimited),
		"memory_alloc_mb":    bToMb(m.Alloc),
		"memory_sys_mb":      bToMb(m.Sys),
		"goroutines":         runtime.NumGoroutine(),
		"gc_runs":            m.NumGC,
		"version":            s.GetVersion(),
		"go_version":         runtime.Version(),
		"platform":           fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}
