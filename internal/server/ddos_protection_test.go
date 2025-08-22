package server

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDDoSProtection_CheckConnectionAllowed(t *testing.T) {
	ddos := NewDDoSProtection()
	
	// Test normal connection
	addr, _ := net.ResolveTCPAddr("tcp", "192.168.1.100:12345")
	assert.True(t, ddos.CheckConnectionAllowed(addr))
	
	// Test rate limiting
	for i := 0; i < 15; i++ {
		ddos.CheckConnectionAllowed(addr)
	}
	
	// Should be rate limited now
	assert.False(t, ddos.CheckConnectionAllowed(addr))
}

func TestDDoSProtection_ConnectionRateLimit(t *testing.T) {
	ddos := NewDDoSProtection()
	ddos.maxConnectionsPerSec = 3
	
	addr, _ := net.ResolveTCPAddr("tcp", "10.0.0.1:54321")
	
	// First 3 connections should be allowed
	for i := 0; i < 3; i++ {
		assert.True(t, ddos.CheckConnectionAllowed(addr), "Connection %d should be allowed", i+1)
	}
	
	// 4th connection should be blocked
	assert.False(t, ddos.CheckConnectionAllowed(addr), "4th connection should be blocked")
	
	// Wait and try again
	time.Sleep(time.Second)
	assert.True(t, ddos.CheckConnectionAllowed(addr), "Connection after wait should be allowed")
}

func TestPortScanDetector_IsPortScanning(t *testing.T) {
	psd := NewPortScanDetector()
	psd.maxPortsPerIP = 5
	
	ip := "192.168.1.200"
	
	// Normal port access
	psd.RecordPortAccess(ip, 8080)
	assert.False(t, psd.IsPortScanning(ip))
	
	// Simulate port scanning
	for port := 8000; port < 8010; port++ {
		psd.RecordPortAccess(ip, port)
	}
	
	assert.True(t, psd.IsPortScanning(ip))
}

func TestPortScanDetector_ConsecutiveScanning(t *testing.T) {
	psd := NewPortScanDetector()
	psd.consecutiveThresh = 3
	
	ip := "10.0.0.200"
	
	// Rapid consecutive port access
	start := time.Now()
	for i := 0; i < 5; i++ {
		psd.RecordPortAccess(ip, 8000+i)
		// Simulate rapid scanning
		time.Sleep(100 * time.Millisecond)
	}
	
	assert.True(t, psd.IsPortScanning(ip))
	
	// Verify timing
	elapsed := time.Since(start)
	assert.Less(t, elapsed, 2*time.Second, "Test should complete quickly")
}

func TestDDoSProtection_Metrics(t *testing.T) {
	ddos := NewDDoSProtection()
	
	// Generate some activity
	addr1, _ := net.ResolveTCPAddr("tcp", "192.168.1.1:12345")
	addr2, _ := net.ResolveTCPAddr("tcp", "192.168.1.2:12345")
	
	// Normal connections
	ddos.CheckConnectionAllowed(addr1)
	ddos.CheckConnectionAllowed(addr2)
	
	// Trigger rate limiting
	for i := 0; i < 15; i++ {
		ddos.CheckConnectionAllowed(addr1)
	}
	
	// Record port scanning
	ddos.RecordPortAccess(addr2, 8080)
	
	metrics := ddos.GetMetrics()
	
	assert.Contains(t, metrics, "blocked_connections")
	assert.Contains(t, metrics, "rate_limited_connections")
	assert.Contains(t, metrics, "active_tracked_ips")
	assert.Contains(t, metrics, "max_connections_per_ip")
	
	// Should have some rate limited connections
	assert.Greater(t, metrics["rate_limited_connections"].(uint64), uint64(0))
}

func TestDDoSProtection_Cleanup(t *testing.T) {
	ddos := NewDDoSProtection()
	
	// Add some tracking data
	addr, _ := net.ResolveTCPAddr("tcp", "192.168.1.100:12345")
	ddos.CheckConnectionAllowed(addr)
	ddos.RecordPortAccess(addr, 8080)
	
	// Verify data exists
	ddos.rateMutex.RLock()
	initialTrackers := len(ddos.connectionRates)
	ddos.rateMutex.RUnlock()
	
	ddos.portScanDetector.mutex.RLock()
	initialScanTrackers := len(ddos.portScanDetector.scanAttempts)
	ddos.portScanDetector.mutex.RUnlock()
	
	assert.Greater(t, initialTrackers, 0)
	assert.Greater(t, initialScanTrackers, 0)
	
	// Manually set old timestamps to trigger cleanup
	ddos.rateMutex.Lock()
	for _, tracker := range ddos.connectionRates {
		tracker.lastConnection = time.Now().Add(-2 * time.Hour)
	}
	ddos.rateMutex.Unlock()
	
	ddos.portScanDetector.mutex.Lock()
	for _, tracker := range ddos.portScanDetector.scanAttempts {
		tracker.lastAttempt = time.Now().Add(-2 * time.Hour)
	}
	ddos.portScanDetector.mutex.Unlock()
	
	// Run cleanup
	ddos.Cleanup()
	
	// Verify cleanup worked
	ddos.rateMutex.RLock()
	finalTrackers := len(ddos.connectionRates)
	ddos.rateMutex.RUnlock()
	
	ddos.portScanDetector.mutex.RLock()
	finalScanTrackers := len(ddos.portScanDetector.scanAttempts)
	ddos.portScanDetector.mutex.RUnlock()
	
	assert.Equal(t, 0, finalTrackers)
	assert.Equal(t, 0, finalScanTrackers)
}

func TestDDoSProtection_InvalidAddress(t *testing.T) {
	ddos := NewDDoSProtection()
	
	// Test with invalid address
	invalidAddr := &net.TCPAddr{
		IP:   nil,
		Port: 0,
	}
	
	assert.False(t, ddos.CheckConnectionAllowed(invalidAddr))
}

func BenchmarkDDoSProtection_CheckConnectionAllowed(b *testing.B) {
	ddos := NewDDoSProtection()
	addr, _ := net.ResolveTCPAddr("tcp", "192.168.1.100:12345")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ddos.CheckConnectionAllowed(addr)
	}
}

func BenchmarkPortScanDetector_RecordPortAccess(b *testing.B) {
	psd := NewPortScanDetector()
	ip := "192.168.1.100"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		psd.RecordPortAccess(ip, 8000+(i%1000))
	}
}
