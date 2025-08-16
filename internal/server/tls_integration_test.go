package server

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// generateTestCertificate creates a self-signed certificate for testing
func generateTestCertificate(t *testing.T) (certFile, keyFile string) {
	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"Test"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"Test City"},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
		DNSNames:     []string{"localhost"},
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)

	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "tls_test")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tempDir) })

	// Write certificate file
	certFile = filepath.Join(tempDir, "cert.pem")
	certOut, err := os.Create(certFile)
	require.NoError(t, err)
	defer certOut.Close()

	err = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	require.NoError(t, err)

	// Write private key file
	keyFile = filepath.Join(tempDir, "key.pem")
	keyOut, err := os.Create(keyFile)
	require.NoError(t, err)
	defer keyOut.Close()

	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)

	err = pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyDER})
	require.NoError(t, err)

	return certFile, keyFile
}

func TestTLSIntegration(t *testing.T) {
	certFile, keyFile := generateTestCertificate(t)

	t.Run("TLS server startup", func(t *testing.T) {
		config := DefaultConfig()
		config.ListenAddr = "127.0.0.1:0" // Use random port
		config.TLS = &TLSConfig{
			Enabled:    true,
			CertFile:   certFile,
			KeyFile:    keyFile,
			MinVersion: tls.VersionTLS13,
			MaxVersion: tls.VersionTLS13,
		}

		server := NewServer(config)
		require.NotNil(t, server)

		// Start server in goroutine
		errCh := make(chan error, 1)
		go func() {
			errCh <- server.Start()
		}()

		// Give server time to start
		time.Sleep(100 * time.Millisecond)

		// Verify server is listening
		assert.NotNil(t, server.listener)

		// Stop server
		server.Stop(context.Background())

		// Wait for server to stop
		select {
		case err := <-errCh:
			assert.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("Server did not stop within timeout")
		}
	})

	t.Run("TLS client connection", func(t *testing.T) {
		config := DefaultConfig()
		config.ListenAddr = "127.0.0.1:0"
		config.TLS = &TLSConfig{
			Enabled:    true,
			CertFile:   certFile,
			KeyFile:    keyFile,
			MinVersion: tls.VersionTLS13,
			MaxVersion: tls.VersionTLS13,
		}

		server := NewServer(config)
		require.NotNil(t, server)

		// Start server
		go server.Start()
		time.Sleep(100 * time.Millisecond)
		defer server.Stop(context.Background())

		// Get server address
		addr := server.ListenAddr()

		// Create TLS client
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true, // Skip verification for test
		}

		conn, err := tls.Dial("tcp", addr, tlsConfig)
		require.NoError(t, err)
		defer conn.Close()

		// Verify TLS connection
		state := conn.ConnectionState()
		assert.True(t, state.HandshakeComplete)
		assert.Equal(t, uint16(tls.VersionTLS13), state.Version)
	})

	t.Run("TLS metrics tracking", func(t *testing.T) {
		config := DefaultConfig()
		config.ListenAddr = "127.0.0.1:0"
		config.TLS = &TLSConfig{
			Enabled:    true,
			CertFile:   certFile,
			KeyFile:    keyFile,
			MinVersion: tls.VersionTLS13,
			MaxVersion: tls.VersionTLS13,
		}

		server := NewServer(config)
		require.NotNil(t, server)

		// Start server
		go server.Start()
		time.Sleep(100 * time.Millisecond)
		defer server.Stop(context.Background())

		// Initial metrics should be zero
		stats := server.GetStats()
		if tlsData, ok := stats["tls"]; ok {
			tlsStats := tlsData.(map[string]interface{})
			assert.Equal(t, int64(0), tlsStats["connections_total"])
			assert.Equal(t, int64(0), tlsStats["handshakes_total"])
		}

		// Connect client
		addr := server.ListenAddr()
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
		}

		conn, err := tls.Dial("tcp", addr, tlsConfig)
		require.NoError(t, err)
		conn.Close()

		// Give time for metrics to update
		time.Sleep(50 * time.Millisecond)

		// Check updated metrics
		stats = server.GetStats()
		if tlsData, ok := stats["tls"]; ok {
			tlsStats := tlsData.(map[string]interface{})
			assert.Equal(t, int64(1), tlsStats["connections_total"])
			assert.Equal(t, int64(1), tlsStats["handshakes_total"])
			assert.Equal(t, int64(0), tlsStats["handshake_errors_total"])
		}
	})
}

func TestTLSMetricsIntegration(t *testing.T) {
	metrics := NewTLSMetrics()

	t.Run("record TLS operations", func(t *testing.T) {
		// Record some operations
		metrics.RecordTLSConnection()
		metrics.RecordTLSHandshake(10*time.Millisecond, nil)
		metrics.RecordTLSVersion(tls.VersionTLS13)
		metrics.RecordCipherSuite(tls.TLS_AES_256_GCM_SHA384)

		// Get metrics
		tlsMetrics := metrics.GetTLSMetrics()
		
		assert.Equal(t, int64(1), tlsMetrics["connections_total"])
		assert.Equal(t, int64(1), tlsMetrics["handshakes_total"])
		assert.Equal(t, int64(0), tlsMetrics["handshake_errors_total"])
		assert.True(t, tlsMetrics["handshake_duration_avg_ms"].(float64) > 0)
		
		// Check version and cipher suite tracking
		versions := tlsMetrics["tls_versions"].(map[string]int64)
		assert.Equal(t, int64(1), versions["TLS 1.3"])
		
		ciphers := tlsMetrics["cipher_suites"].(map[string]int64)
		assert.Equal(t, int64(1), ciphers["TLS_AES_256_GCM_SHA384"])
	})

	t.Run("health status", func(t *testing.T) {
		health := metrics.GetTLSHealthStatus()
		
		assert.True(t, health["healthy"].(bool))
		assert.Equal(t, float64(0), health["error_rate"])
		assert.True(t, health["avg_handshake_duration_ms"].(float64) > 0)
	})
}
