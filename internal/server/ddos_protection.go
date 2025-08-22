// Package server implements DDoS protection mechanisms for the TCP server.
package server

import (
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// DDoSProtection provides protection against various DDoS attack vectors
type DDoSProtection struct {
	// Connection rate limiting per IP
	connectionRates map[string]*ConnectionRateTracker
	rateMutex       sync.RWMutex
	
	// Global connection limits
	maxConnectionsPerIP    int32
	connectionRateWindow   time.Duration
	maxConnectionsPerSec   int32
	
	// Port scanning detection
	portScanDetector *PortScanDetector
	
	// Metrics
	blockedConnections     uint64
	rateLimitedConnections uint64
	portScanAttempts       uint64
}

// ConnectionRateTracker tracks connection attempts per IP
type ConnectionRateTracker struct {
	connections    []time.Time
	lastConnection time.Time
	totalAttempts  uint64
	mutex          sync.Mutex
}

// PortScanDetector detects port scanning attempts
type PortScanDetector struct {
	scanAttempts map[string]*ScanTracker
	mutex        sync.RWMutex
	
	// Detection thresholds
	maxPortsPerIP     int
	scanTimeWindow    time.Duration
	consecutiveThresh int
}

// ScanTracker tracks scanning behavior per IP
type ScanTracker struct {
	ports         map[int]time.Time
	lastAttempt   time.Time
	totalAttempts int
	consecutive   int
}

// NewDDoSProtection creates a new DDoS protection instance
func NewDDoSProtection() *DDoSProtection {
	return &DDoSProtection{
		connectionRates:        make(map[string]*ConnectionRateTracker),
		maxConnectionsPerIP:    100,  // Max 100 connections per IP
		connectionRateWindow:   time.Minute,
		maxConnectionsPerSec:   10,   // Max 10 connections per second per IP
		portScanDetector:       NewPortScanDetector(),
	}
}

// NewPortScanDetector creates a new port scan detector
func NewPortScanDetector() *PortScanDetector {
	return &PortScanDetector{
		scanAttempts:      make(map[string]*ScanTracker),
		maxPortsPerIP:     20,
		scanTimeWindow:    5 * time.Minute,
		consecutiveThresh: 5,
	}
}

// CheckConnectionAllowed verifies if a connection from the given IP should be allowed
func (d *DDoSProtection) CheckConnectionAllowed(remoteAddr net.Addr) bool {
	host, _, err := net.SplitHostPort(remoteAddr.String())
	if err != nil {
		return false
	}
	
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	
	// Check if IP is currently being port scanned
	if d.portScanDetector.IsPortScanning(host) {
		atomic.AddUint64(&d.blockedConnections, 1)
		return false
	}
	
	// Check connection rate limits
	if !d.checkConnectionRate(host) {
		atomic.AddUint64(&d.rateLimitedConnections, 1)
		return false
	}
	
	return true
}

// checkConnectionRate verifies connection rate limits for an IP
func (d *DDoSProtection) checkConnectionRate(ip string) bool {
	d.rateMutex.Lock()
	defer d.rateMutex.Unlock()
	
	now := time.Now()
	tracker, exists := d.connectionRates[ip]
	if !exists {
		tracker = &ConnectionRateTracker{
			connections: make([]time.Time, 0),
		}
		d.connectionRates[ip] = tracker
	}
	
	tracker.mutex.Lock()
	defer tracker.mutex.Unlock()
	
	// Clean old connections outside the rate window
	cutoff := now.Add(-d.connectionRateWindow)
	var validConnections []time.Time
	for _, connTime := range tracker.connections {
		if connTime.After(cutoff) {
			validConnections = append(validConnections, connTime)
		}
	}
	tracker.connections = validConnections
	
	// Check if we're exceeding the rate limit
	if len(tracker.connections) >= int(d.maxConnectionsPerSec) {
		return false
	}
	
	// Check for burst connections (too many in short time)
	if len(tracker.connections) > 0 {
		timeSinceLastConn := now.Sub(tracker.lastConnection)
		if timeSinceLastConn < time.Second/time.Duration(d.maxConnectionsPerSec) {
			return false
		}
	}
	
	// Record this connection
	tracker.connections = append(tracker.connections, now)
	tracker.lastConnection = now
	tracker.totalAttempts++
	
	return true
}

// RecordPortAccess records a port access attempt for scan detection
func (d *DDoSProtection) RecordPortAccess(remoteAddr net.Addr, port int) {
	host, _, err := net.SplitHostPort(remoteAddr.String())
	if err != nil {
		return
	}
	
	d.portScanDetector.RecordPortAccess(host, port)
}

// IsPortScanning checks if an IP is currently port scanning
func (psd *PortScanDetector) IsPortScanning(ip string) bool {
	psd.mutex.RLock()
	defer psd.mutex.RUnlock()
	
	tracker, exists := psd.scanAttempts[ip]
	if !exists {
		return false
	}
	
	now := time.Now()
	
	// Check if we have too many ports accessed recently
	recentPorts := 0
	for _, accessTime := range tracker.ports {
		if now.Sub(accessTime) <= psd.scanTimeWindow {
			recentPorts++
		}
	}
	
	return recentPorts >= psd.maxPortsPerIP || tracker.consecutive >= psd.consecutiveThresh
}

// RecordPortAccess records a port access attempt
func (psd *PortScanDetector) RecordPortAccess(ip string, port int) {
	psd.mutex.Lock()
	defer psd.mutex.Unlock()
	
	now := time.Now()
	tracker, exists := psd.scanAttempts[ip]
	if !exists {
		tracker = &ScanTracker{
			ports: make(map[int]time.Time),
		}
		psd.scanAttempts[ip] = tracker
	}
	
	// Record port access
	tracker.ports[port] = now
	tracker.totalAttempts++
	
	// Check for consecutive port scanning
	if now.Sub(tracker.lastAttempt) <= time.Second {
		tracker.consecutive++
	} else {
		tracker.consecutive = 1
	}
	
	tracker.lastAttempt = now
}

// GetMetrics returns DDoS protection metrics
func (d *DDoSProtection) GetMetrics() map[string]interface{} {
	d.rateMutex.RLock()
	activeIPs := len(d.connectionRates)
	d.rateMutex.RUnlock()
	
	d.portScanDetector.mutex.RLock()
	suspiciousIPs := len(d.portScanDetector.scanAttempts)
	d.portScanDetector.mutex.RUnlock()
	
	return map[string]interface{}{
		"blocked_connections":      atomic.LoadUint64(&d.blockedConnections),
		"rate_limited_connections": atomic.LoadUint64(&d.rateLimitedConnections),
		"port_scan_attempts":       atomic.LoadUint64(&d.portScanAttempts),
		"active_tracked_ips":       activeIPs,
		"suspicious_ips":           suspiciousIPs,
		"max_connections_per_ip":   d.maxConnectionsPerIP,
		"max_connections_per_sec":  d.maxConnectionsPerSec,
	}
}

// Cleanup removes old tracking data to prevent memory leaks
func (d *DDoSProtection) Cleanup() {
	now := time.Now()
	cleanupCutoff := now.Add(-time.Hour) // Clean data older than 1 hour
	
	// Clean connection rate trackers
	d.rateMutex.Lock()
	for ip, tracker := range d.connectionRates {
		tracker.mutex.Lock()
		if tracker.lastConnection.Before(cleanupCutoff) {
			delete(d.connectionRates, ip)
		}
		tracker.mutex.Unlock()
	}
	d.rateMutex.Unlock()
	
	// Clean port scan trackers
	d.portScanDetector.mutex.Lock()
	for ip, tracker := range d.portScanDetector.scanAttempts {
		if tracker.lastAttempt.Before(cleanupCutoff) {
			delete(d.portScanDetector.scanAttempts, ip)
		}
	}
	d.portScanDetector.mutex.Unlock()
}

// StartCleanupRoutine starts a background cleanup routine
func (d *DDoSProtection) StartCleanupRoutine() {
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		
		for range ticker.C {
			d.Cleanup()
		}
	}()
}
