package server

import (
	"crypto/tls"
	"sync"
	"sync/atomic"
	"time"
)

// TLSMetrics tracks TLS-related performance and security metrics
type TLSMetrics struct {
	// Connection metrics
	TLSConnections      int64
	TLSHandshakes       int64
	TLSHandshakeErrors  int64
	TLSHandshakeDuration int64 // nanoseconds
	
	// Certificate metrics
	CertificateValidations int64
	CertificateErrors      int64
	ClientCertValidations  int64
	ClientCertErrors       int64
	
	// Protocol metrics
	TLS13Connections int64
	TLS12Connections int64
	OtherTLSVersions int64
	
	// Cipher suite metrics
	mu           sync.RWMutex
	CipherSuites map[uint16]int64
	
	// Performance metrics
	AverageHandshakeTime time.Duration
	MaxHandshakeTime     time.Duration
	MinHandshakeTime     time.Duration
}

// NewTLSMetrics creates a new TLS metrics instance
func NewTLSMetrics() *TLSMetrics {
	return &TLSMetrics{
		CipherSuites:     make(map[uint16]int64),
		MinHandshakeTime: time.Duration(^uint64(0) >> 1), // Max duration
	}
}

// RecordTLSConnection records a new TLS connection
func (m *TLSMetrics) RecordTLSConnection() {
	atomic.AddInt64(&m.TLSConnections, 1)
}

// RecordTLSHandshake records a TLS handshake with timing
func (m *TLSMetrics) RecordTLSHandshake(duration time.Duration, err error) {
	atomic.AddInt64(&m.TLSHandshakes, 1)
	
	if err != nil {
		atomic.AddInt64(&m.TLSHandshakeErrors, 1)
		return
	}
	
	// Update timing metrics
	atomic.AddInt64(&m.TLSHandshakeDuration, int64(duration))
	
	// Update min/max handshake times (with basic race condition handling)
	if duration > m.MaxHandshakeTime {
		m.MaxHandshakeTime = duration
	}
	if duration < m.MinHandshakeTime {
		m.MinHandshakeTime = duration
	}
	
	// Calculate average (simple approximation)
	totalHandshakes := atomic.LoadInt64(&m.TLSHandshakes)
	if totalHandshakes > 0 {
		avgNanos := atomic.LoadInt64(&m.TLSHandshakeDuration) / totalHandshakes
		m.AverageHandshakeTime = time.Duration(avgNanos)
	}
}

// RecordTLSVersion records the TLS version used
func (m *TLSMetrics) RecordTLSVersion(version uint16) {
	switch version {
	case tls.VersionTLS13:
		atomic.AddInt64(&m.TLS13Connections, 1)
	case tls.VersionTLS12:
		atomic.AddInt64(&m.TLS12Connections, 1)
	default:
		atomic.AddInt64(&m.OtherTLSVersions, 1)
	}
}

// RecordCipherSuite records the cipher suite used
func (m *TLSMetrics) RecordCipherSuite(cipherSuite uint16) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CipherSuites[cipherSuite]++
}

// RecordCertificateValidation records certificate validation results
func (m *TLSMetrics) RecordCertificateValidation(err error) {
	atomic.AddInt64(&m.CertificateValidations, 1)
	if err != nil {
		atomic.AddInt64(&m.CertificateErrors, 1)
	}
}

// RecordClientCertValidation records client certificate validation results
func (m *TLSMetrics) RecordClientCertValidation(err error) {
	atomic.AddInt64(&m.ClientCertValidations, 1)
	if err != nil {
		atomic.AddInt64(&m.ClientCertErrors, 1)
	}
}

// GetTLSMetrics returns current TLS metrics
func (m *TLSMetrics) GetTLSMetrics() map[string]interface{} {
	m.mu.RLock()
	cipherSuites := make(map[string]int64)
	for suite, count := range m.CipherSuites {
		cipherSuites[m.getCipherSuiteName(suite)] = count
	}
	m.mu.RUnlock()
	
	return map[string]interface{}{
		"tls_connections":          atomic.LoadInt64(&m.TLSConnections),
		"tls_handshakes":           atomic.LoadInt64(&m.TLSHandshakes),
		"tls_handshake_errors":     atomic.LoadInt64(&m.TLSHandshakeErrors),
		"tls13_connections":        atomic.LoadInt64(&m.TLS13Connections),
		"tls12_connections":        atomic.LoadInt64(&m.TLS12Connections),
		"other_tls_versions":       atomic.LoadInt64(&m.OtherTLSVersions),
		"certificate_validations":  atomic.LoadInt64(&m.CertificateValidations),
		"certificate_errors":       atomic.LoadInt64(&m.CertificateErrors),
		"client_cert_validations":  atomic.LoadInt64(&m.ClientCertValidations),
		"client_cert_errors":       atomic.LoadInt64(&m.ClientCertErrors),
		"average_handshake_time_ms": float64(m.AverageHandshakeTime.Nanoseconds()) / 1e6,
		"max_handshake_time_ms":     float64(m.MaxHandshakeTime.Nanoseconds()) / 1e6,
		"min_handshake_time_ms":     float64(m.MinHandshakeTime.Nanoseconds()) / 1e6,
		"cipher_suites":            cipherSuites,
	}
}

// getCipherSuiteName returns a human-readable cipher suite name
func (m *TLSMetrics) getCipherSuiteName(suite uint16) string {
	switch suite {
	case tls.TLS_AES_128_GCM_SHA256:
		return "TLS_AES_128_GCM_SHA256"
	case tls.TLS_AES_256_GCM_SHA384:
		return "TLS_AES_256_GCM_SHA384"
	case tls.TLS_CHACHA20_POLY1305_SHA256:
		return "TLS_CHACHA20_POLY1305_SHA256"
	case tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256:
		return "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256"
	case tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384:
		return "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384"
	case tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256:
		return "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"
	case tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384:
		return "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"
	default:
		return "Unknown"
	}
}

// GetTLSHealthStatus returns TLS health indicators
func (m *TLSMetrics) GetTLSHealthStatus() map[string]interface{} {
	totalHandshakes := atomic.LoadInt64(&m.TLSHandshakes)
	handshakeErrors := atomic.LoadInt64(&m.TLSHandshakeErrors)
	certErrors := atomic.LoadInt64(&m.CertificateErrors)
	clientCertErrors := atomic.LoadInt64(&m.ClientCertErrors)
	
	var handshakeErrorRate, certErrorRate, clientCertErrorRate float64
	
	if totalHandshakes > 0 {
		handshakeErrorRate = float64(handshakeErrors) / float64(totalHandshakes) * 100
	}
	
	totalCertValidations := atomic.LoadInt64(&m.CertificateValidations)
	if totalCertValidations > 0 {
		certErrorRate = float64(certErrors) / float64(totalCertValidations) * 100
	}
	
	totalClientCertValidations := atomic.LoadInt64(&m.ClientCertValidations)
	if totalClientCertValidations > 0 {
		clientCertErrorRate = float64(clientCertErrors) / float64(totalClientCertValidations) * 100
	}
	
	// Determine health status
	healthy := handshakeErrorRate < 5.0 && certErrorRate < 1.0 && clientCertErrorRate < 1.0
	
	return map[string]interface{}{
		"healthy":                   healthy,
		"handshake_error_rate":      handshakeErrorRate,
		"certificate_error_rate":    certErrorRate,
		"client_cert_error_rate":    clientCertErrorRate,
		"average_handshake_time_ms": float64(m.AverageHandshakeTime.Nanoseconds()) / 1e6,
		"tls13_usage_percentage":    m.getTLS13UsagePercentage(),
	}
}

// getTLS13UsagePercentage calculates the percentage of TLS 1.3 connections
func (m *TLSMetrics) getTLS13UsagePercentage() float64 {
	tls13 := atomic.LoadInt64(&m.TLS13Connections)
	tls12 := atomic.LoadInt64(&m.TLS12Connections)
	other := atomic.LoadInt64(&m.OtherTLSVersions)
	
	total := tls13 + tls12 + other
	if total == 0 {
		return 0.0
	}
	
	return float64(tls13) / float64(total) * 100
}

// Reset resets all TLS metrics
func (m *TLSMetrics) Reset() {
	atomic.StoreInt64(&m.TLSConnections, 0)
	atomic.StoreInt64(&m.TLSHandshakes, 0)
	atomic.StoreInt64(&m.TLSHandshakeErrors, 0)
	atomic.StoreInt64(&m.TLSHandshakeDuration, 0)
	atomic.StoreInt64(&m.CertificateValidations, 0)
	atomic.StoreInt64(&m.CertificateErrors, 0)
	atomic.StoreInt64(&m.ClientCertValidations, 0)
	atomic.StoreInt64(&m.ClientCertErrors, 0)
	atomic.StoreInt64(&m.TLS13Connections, 0)
	atomic.StoreInt64(&m.TLS12Connections, 0)
	atomic.StoreInt64(&m.OtherTLSVersions, 0)
	
	m.mu.Lock()
	m.CipherSuites = make(map[uint16]int64)
	m.mu.Unlock()
	
	m.AverageHandshakeTime = 0
	m.MaxHandshakeTime = 0
	m.MinHandshakeTime = time.Duration(^uint64(0) >> 1)
}
