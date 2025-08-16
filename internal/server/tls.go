package server

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"
)

// TLSConfig holds TLS configuration settings
type TLSConfig struct {
	// Basic TLS settings
	Enabled         bool
	CertFile        string
	KeyFile         string
	CAFile          string
	
	// mTLS settings
	ClientAuth      tls.ClientAuthType
	ClientCAFile    string
	
	// Security settings
	MinVersion      uint16
	MaxVersion      uint16
	CipherSuites    []uint16
	CurvePreferences []tls.CurveID
	
	// OCSP and certificate validation
	OCSPEnabled     bool
	InsecureSkipVerify bool
	
	// Certificate rotation
	CertWatchEnabled bool
	CertCheckInterval time.Duration
}

// DefaultTLSConfig returns secure default TLS configuration
func DefaultTLSConfig() *TLSConfig {
	cfg := &TLSConfig{
		Enabled:    false, // Disabled by default, enable via environment
		MinVersion: tls.VersionTLS13,
		MaxVersion: tls.VersionTLS13,
		ClientAuth: tls.NoClientCert,
		
		// Strong cipher suites for TLS 1.3
		CipherSuites: []uint16{
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_CHACHA20_POLY1305_SHA256,
		},
		
		// Secure curve preferences
		CurvePreferences: []tls.CurveID{
			tls.X25519,
			tls.CurveP384,
			tls.CurveP256,
		},
		
		OCSPEnabled:       false, // Can be enabled for production
		InsecureSkipVerify: false,
		CertWatchEnabled:  false,
		CertCheckInterval: 5 * time.Minute,
	}
	
	return cfg
}

// LoadTLSConfigFromEnv loads TLS configuration from environment variables
func LoadTLSConfigFromEnv(cfg *TLSConfig) {
	if enabled := os.Getenv("TLS_ENABLED"); enabled != "" {
		cfg.Enabled = strings.ToLower(enabled) == "true"
	}
	
	if certFile := os.Getenv("TLS_CERT_FILE"); certFile != "" {
		cfg.CertFile = certFile
	}
	
	if keyFile := os.Getenv("TLS_KEY_FILE"); keyFile != "" {
		cfg.KeyFile = keyFile
	}
	
	if caFile := os.Getenv("TLS_CA_FILE"); caFile != "" {
		cfg.CAFile = caFile
	}
	
	if clientCAFile := os.Getenv("TLS_CLIENT_CA_FILE"); clientCAFile != "" {
		cfg.ClientCAFile = clientCAFile
	}
	
	// Client authentication mode
	if clientAuth := os.Getenv("TLS_CLIENT_AUTH"); clientAuth != "" {
		switch strings.ToLower(clientAuth) {
		case "none":
			cfg.ClientAuth = tls.NoClientCert
		case "request":
			cfg.ClientAuth = tls.RequestClientCert
		case "require":
			cfg.ClientAuth = tls.RequireAnyClientCert
		case "verify":
			cfg.ClientAuth = tls.VerifyClientCertIfGiven
		case "require_verify":
			cfg.ClientAuth = tls.RequireAndVerifyClientCert
		}
	}
	
	if ocsp := os.Getenv("TLS_OCSP_ENABLED"); ocsp != "" {
		cfg.OCSPEnabled = strings.ToLower(ocsp) == "true"
	}
	
	if insecure := os.Getenv("TLS_INSECURE_SKIP_VERIFY"); insecure != "" {
		cfg.InsecureSkipVerify = strings.ToLower(insecure) == "true"
	}
	
	if certWatch := os.Getenv("TLS_CERT_WATCH_ENABLED"); certWatch != "" {
		cfg.CertWatchEnabled = strings.ToLower(certWatch) == "true"
	}
	
	if interval := os.Getenv("TLS_CERT_CHECK_INTERVAL"); interval != "" {
		if d, err := time.ParseDuration(interval); err == nil {
			cfg.CertCheckInterval = d
		}
	}
}

// BuildTLSConfig creates a *tls.Config from TLSConfig
func (cfg *TLSConfig) BuildTLSConfig() (*tls.Config, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	
	// Validate required files
	if cfg.CertFile == "" || cfg.KeyFile == "" {
		return nil, fmt.Errorf("TLS_CERT_FILE and TLS_KEY_FILE must be specified when TLS is enabled")
	}
	
	// Load server certificate
	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load server certificate: %w", err)
	}
	
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   cfg.MinVersion,
		MaxVersion:   cfg.MaxVersion,
		CipherSuites: cfg.CipherSuites,
		CurvePreferences: cfg.CurvePreferences,
		ClientAuth:   cfg.ClientAuth,
		InsecureSkipVerify: cfg.InsecureSkipVerify,
	}
	
	// Configure client certificate validation for mTLS
	if cfg.ClientAuth != tls.NoClientCert {
		if err := cfg.setupClientCertValidation(tlsConfig); err != nil {
			return nil, fmt.Errorf("failed to setup client certificate validation: %w", err)
		}
	}
	
	// Set up custom verification if OCSP is enabled
	if cfg.OCSPEnabled {
		tlsConfig.VerifyConnection = cfg.verifyConnectionWithOCSP
	}
	
	return tlsConfig, nil
}

// setupClientCertValidation configures client certificate validation for mTLS
func (cfg *TLSConfig) setupClientCertValidation(tlsConfig *tls.Config) error {
	if cfg.ClientCAFile == "" {
		return fmt.Errorf("TLS_CLIENT_CA_FILE must be specified for client certificate authentication")
	}
	
	// Load client CA certificates
	caCert, err := ioutil.ReadFile(cfg.ClientCAFile)
	if err != nil {
		return fmt.Errorf("failed to read client CA file: %w", err)
	}
	
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return fmt.Errorf("failed to parse client CA certificate")
	}
	
	tlsConfig.ClientCAs = caCertPool
	
	// Set up custom client certificate verification
	tlsConfig.VerifyPeerCertificate = cfg.verifyClientCertificate
	
	return nil
}

// verifyClientCertificate performs custom client certificate verification
func (cfg *TLSConfig) verifyClientCertificate(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
	if len(rawCerts) == 0 {
		return fmt.Errorf("no client certificate provided")
	}
	
	// Parse client certificate
	cert, err := x509.ParseCertificate(rawCerts[0])
	if err != nil {
		return fmt.Errorf("failed to parse client certificate: %w", err)
	}
	
	// Check certificate validity period
	now := time.Now()
	if now.Before(cert.NotBefore) {
		return fmt.Errorf("client certificate not yet valid")
	}
	if now.After(cert.NotAfter) {
		return fmt.Errorf("client certificate has expired")
	}
	
	// Additional custom validation can be added here
	// For example: checking specific certificate fields, CRL, etc.
	
	return nil
}

// verifyConnectionWithOCSP performs OCSP verification during TLS handshake
func (cfg *TLSConfig) verifyConnectionWithOCSP(cs tls.ConnectionState) error {
	// Basic connection state validation
	if len(cs.PeerCertificates) == 0 {
		return fmt.Errorf("no peer certificates")
	}
	
	// OCSP verification implementation would go here
	// This is a placeholder for actual OCSP implementation
	// In production, you would implement proper OCSP checking
	
	return nil
}

// ValidateTLSConfig validates the TLS configuration
func (cfg *TLSConfig) ValidateTLSConfig() error {
	if !cfg.Enabled {
		return nil
	}
	
	// Check required files exist
	if cfg.CertFile != "" {
		if _, err := os.Stat(cfg.CertFile); os.IsNotExist(err) {
			return fmt.Errorf("TLS certificate file does not exist: %s", cfg.CertFile)
		}
	}
	
	if cfg.KeyFile != "" {
		if _, err := os.Stat(cfg.KeyFile); os.IsNotExist(err) {
			return fmt.Errorf("TLS key file does not exist: %s", cfg.KeyFile)
		}
	}
	
	if cfg.ClientCAFile != "" {
		if _, err := os.Stat(cfg.ClientCAFile); os.IsNotExist(err) {
			return fmt.Errorf("TLS client CA file does not exist: %s", cfg.ClientCAFile)
		}
	}
	
	// Validate TLS version settings
	if cfg.MinVersion > cfg.MaxVersion {
		return fmt.Errorf("TLS min version cannot be greater than max version")
	}
	
	// Ensure TLS 1.3 is used for security
	if cfg.MinVersion < tls.VersionTLS13 {
		return fmt.Errorf("minimum TLS version must be 1.3 for security compliance")
	}
	
	return nil
}

// GetTLSInfo returns information about the TLS configuration
func (cfg *TLSConfig) GetTLSInfo() map[string]interface{} {
	info := map[string]interface{}{
		"enabled":     cfg.Enabled,
		"min_version": cfg.getTLSVersionString(cfg.MinVersion),
		"max_version": cfg.getTLSVersionString(cfg.MaxVersion),
		"client_auth": cfg.getClientAuthString(cfg.ClientAuth),
		"ocsp_enabled": cfg.OCSPEnabled,
		"cert_watch_enabled": cfg.CertWatchEnabled,
	}
	
	if cfg.Enabled {
		info["cert_file"] = cfg.CertFile
		info["key_file"] = cfg.KeyFile
		if cfg.ClientCAFile != "" {
			info["client_ca_file"] = cfg.ClientCAFile
		}
	}
	
	return info
}

// getTLSVersionString returns a human-readable TLS version string
func (cfg *TLSConfig) getTLSVersionString(version uint16) string {
	switch version {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return fmt.Sprintf("Unknown (%d)", version)
	}
}

// getClientAuthString returns a human-readable client auth string
func (cfg *TLSConfig) getClientAuthString(auth tls.ClientAuthType) string {
	switch auth {
	case tls.NoClientCert:
		return "None"
	case tls.RequestClientCert:
		return "Request"
	case tls.RequireAnyClientCert:
		return "Require Any"
	case tls.VerifyClientCertIfGiven:
		return "Verify If Given"
	case tls.RequireAndVerifyClientCert:
		return "Require and Verify"
	default:
		return fmt.Sprintf("Unknown (%d)", auth)
	}
}
