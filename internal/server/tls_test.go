package server

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultTLSConfig(t *testing.T) {
	cfg := DefaultTLSConfig()
	require.NotNil(t, cfg)
	
	assert.False(t, cfg.Enabled)
	assert.Equal(t, uint16(tls.VersionTLS13), cfg.MinVersion)
	assert.Equal(t, uint16(tls.VersionTLS13), cfg.MaxVersion)
	assert.Equal(t, tls.NoClientCert, cfg.ClientAuth)
	assert.False(t, cfg.OCSPEnabled)
	assert.False(t, cfg.InsecureSkipVerify)
	assert.False(t, cfg.CertWatchEnabled)
	assert.Equal(t, 5*time.Minute, cfg.CertCheckInterval)
	
	// Check cipher suites
	expectedCiphers := []uint16{
		tls.TLS_AES_256_GCM_SHA384,
		tls.TLS_AES_128_GCM_SHA256,
		tls.TLS_CHACHA20_POLY1305_SHA256,
	}
	assert.Equal(t, expectedCiphers, cfg.CipherSuites)
	
	// Check curve preferences
	expectedCurves := []tls.CurveID{
		tls.X25519,
		tls.CurveP384,
		tls.CurveP256,
	}
	assert.Equal(t, expectedCurves, cfg.CurvePreferences)
}

func TestLoadTLSConfigFromEnv(t *testing.T) {
	// Save original env vars
	originalVars := map[string]string{
		"TLS_ENABLED":              os.Getenv("TLS_ENABLED"),
		"TLS_CERT_FILE":            os.Getenv("TLS_CERT_FILE"),
		"TLS_KEY_FILE":             os.Getenv("TLS_KEY_FILE"),
		"TLS_CA_FILE":              os.Getenv("TLS_CA_FILE"),
		"TLS_CLIENT_CA_FILE":       os.Getenv("TLS_CLIENT_CA_FILE"),
		"TLS_CLIENT_AUTH":          os.Getenv("TLS_CLIENT_AUTH"),
		"TLS_OCSP_ENABLED":         os.Getenv("TLS_OCSP_ENABLED"),
		"TLS_INSECURE_SKIP_VERIFY": os.Getenv("TLS_INSECURE_SKIP_VERIFY"),
		"TLS_CERT_WATCH_ENABLED":   os.Getenv("TLS_CERT_WATCH_ENABLED"),
		"TLS_CERT_CHECK_INTERVAL":  os.Getenv("TLS_CERT_CHECK_INTERVAL"),
	}
	
	// Clean up after test
	defer func() {
		for key, value := range originalVars {
			if value == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, value)
			}
		}
	}()
	
	t.Run("enable TLS", func(t *testing.T) {
		os.Setenv("TLS_ENABLED", "true")
		os.Setenv("TLS_CERT_FILE", "/path/to/cert.pem")
		os.Setenv("TLS_KEY_FILE", "/path/to/key.pem")
		
		cfg := DefaultTLSConfig()
		LoadTLSConfigFromEnv(cfg)
		
		assert.True(t, cfg.Enabled)
		assert.Equal(t, "/path/to/cert.pem", cfg.CertFile)
		assert.Equal(t, "/path/to/key.pem", cfg.KeyFile)
	})
	
	t.Run("client authentication modes", func(t *testing.T) {
		testCases := []struct {
			envValue string
			expected tls.ClientAuthType
		}{
			{"none", tls.NoClientCert},
			{"request", tls.RequestClientCert},
			{"require", tls.RequireAnyClientCert},
			{"verify", tls.VerifyClientCertIfGiven},
			{"require_verify", tls.RequireAndVerifyClientCert},
		}
		
		for _, tc := range testCases {
			t.Run(tc.envValue, func(t *testing.T) {
				os.Setenv("TLS_CLIENT_AUTH", tc.envValue)
				
				cfg := DefaultTLSConfig()
				LoadTLSConfigFromEnv(cfg)
				
				assert.Equal(t, tc.expected, cfg.ClientAuth)
			})
		}
	})
	
	t.Run("boolean flags", func(t *testing.T) {
		os.Setenv("TLS_OCSP_ENABLED", "true")
		os.Setenv("TLS_INSECURE_SKIP_VERIFY", "true")
		os.Setenv("TLS_CERT_WATCH_ENABLED", "true")
		
		cfg := DefaultTLSConfig()
		LoadTLSConfigFromEnv(cfg)
		
		assert.True(t, cfg.OCSPEnabled)
		assert.True(t, cfg.InsecureSkipVerify)
		assert.True(t, cfg.CertWatchEnabled)
	})
	
	t.Run("cert check interval", func(t *testing.T) {
		os.Setenv("TLS_CERT_CHECK_INTERVAL", "10m")
		
		cfg := DefaultTLSConfig()
		LoadTLSConfigFromEnv(cfg)
		
		assert.Equal(t, 10*time.Minute, cfg.CertCheckInterval)
	})
}

func TestTLSConfig_ValidateTLSConfig(t *testing.T) {
	t.Run("disabled TLS", func(t *testing.T) {
		cfg := &TLSConfig{Enabled: false}
		err := cfg.ValidateTLSConfig()
		assert.NoError(t, err)
	})
	
	t.Run("missing certificate files", func(t *testing.T) {
		cfg := &TLSConfig{
			Enabled:  true,
			CertFile: "",
			KeyFile:  "",
		}
		
		_, err := cfg.BuildTLSConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "TLS_CERT_FILE and TLS_KEY_FILE must be specified")
	})
	
	t.Run("invalid TLS version range", func(t *testing.T) {
		cfg := &TLSConfig{
			Enabled:    true,
			MinVersion: tls.VersionTLS13,
			MaxVersion: tls.VersionTLS12,
		}
		
		err := cfg.ValidateTLSConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "min version cannot be greater than max version")
	})
	
	t.Run("insecure TLS version", func(t *testing.T) {
		cfg := &TLSConfig{
			Enabled:    true,
			MinVersion: tls.VersionTLS12,
			MaxVersion: tls.VersionTLS13,
		}
		
		err := cfg.ValidateTLSConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "minimum TLS version must be 1.3")
	})
	
	t.Run("valid configuration", func(t *testing.T) {
		cfg := &TLSConfig{
			Enabled:    true,
			MinVersion: tls.VersionTLS13,
			MaxVersion: tls.VersionTLS13,
		}
		
		err := cfg.ValidateTLSConfig()
		assert.NoError(t, err)
	})
}

func TestTLSConfig_GetTLSInfo(t *testing.T) {
	cfg := &TLSConfig{
		Enabled:     true,
		MinVersion:  tls.VersionTLS13,
		MaxVersion:  tls.VersionTLS13,
		ClientAuth:  tls.RequireAndVerifyClientCert,
		OCSPEnabled: true,
		CertWatchEnabled: true,
		CertFile:    "/path/to/cert.pem",
		KeyFile:     "/path/to/key.pem",
		ClientCAFile: "/path/to/ca.pem",
	}
	
	info := cfg.GetTLSInfo()
	
	assert.True(t, info["enabled"].(bool))
	assert.Equal(t, "TLS 1.3", info["min_version"])
	assert.Equal(t, "TLS 1.3", info["max_version"])
	assert.Equal(t, "Require and Verify", info["client_auth"])
	assert.True(t, info["ocsp_enabled"].(bool))
	assert.True(t, info["cert_watch_enabled"].(bool))
	assert.Equal(t, "/path/to/cert.pem", info["cert_file"])
	assert.Equal(t, "/path/to/key.pem", info["key_file"])
	assert.Equal(t, "/path/to/ca.pem", info["client_ca_file"])
}

func TestTLSConfig_getTLSVersionString(t *testing.T) {
	cfg := &TLSConfig{}
	
	testCases := []struct {
		version  uint16
		expected string
	}{
		{tls.VersionTLS10, "TLS 1.0"},
		{tls.VersionTLS11, "TLS 1.1"},
		{tls.VersionTLS12, "TLS 1.2"},
		{tls.VersionTLS13, "TLS 1.3"},
		{0x9999, "Unknown (39321)"},
	}
	
	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			result := cfg.getTLSVersionString(tc.version)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestTLSConfig_getClientAuthString(t *testing.T) {
	cfg := &TLSConfig{}
	
	testCases := []struct {
		auth     tls.ClientAuthType
		expected string
	}{
		{tls.NoClientCert, "None"},
		{tls.RequestClientCert, "Request"},
		{tls.RequireAnyClientCert, "Require Any"},
		{tls.VerifyClientCertIfGiven, "Verify If Given"},
		{tls.RequireAndVerifyClientCert, "Require and Verify"},
		{tls.ClientAuthType(99), "Unknown (99)"},
	}
	
	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			result := cfg.getClientAuthString(tc.auth)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestTLSConfig_verifyClientCertificate(t *testing.T) {
	cfg := &TLSConfig{}
	
	t.Run("no certificates", func(t *testing.T) {
		err := cfg.verifyClientCertificate(nil, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no client certificate provided")
	})
	
	t.Run("empty certificate list", func(t *testing.T) {
		err := cfg.verifyClientCertificate([][]byte{}, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no client certificate provided")
	})
}

func TestTLSConfig_verifyConnectionWithOCSP(t *testing.T) {
	cfg := &TLSConfig{}
	
	t.Run("no peer certificates", func(t *testing.T) {
		cs := tls.ConnectionState{
			PeerCertificates: nil,
		}
		
		err := cfg.verifyConnectionWithOCSP(cs)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no peer certificates")
	})
	
	t.Run("with peer certificates", func(t *testing.T) {
		// This is a placeholder test since we don't have actual certificates
		// In a real implementation, you would test with valid certificates
		cs := tls.ConnectionState{
			PeerCertificates: []*x509.Certificate{{}}, // Mock certificate
		}
		
		// This should pass the basic validation
		err := cfg.verifyConnectionWithOCSP(cs)
		assert.NoError(t, err)
	})
}
