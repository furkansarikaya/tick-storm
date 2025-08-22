// Package server implements the TCP server for Tick-Storm.
package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/furkansarikaya/tick-storm/internal/auth"
	"github.com/furkansarikaya/tick-storm/internal/protocol"
	pb "github.com/furkansarikaya/tick-storm/internal/protocol/pb"
)

var (
	// ErrServerClosed is returned when operations are attempted on a closed server.
	ErrServerClosed = errors.New("server closed")
	
	// ErrMaxConnections is returned when the server has reached its connection limit.
	ErrMaxConnections = errors.New("maximum connections reached")
)

// Config holds server configuration.
type Config struct {
	// Network settings
	ListenAddr      string
	MaxConnections  int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	KeepAlive       time.Duration
	
	// Network security
	AllowCIDRs      []string
	BlockCIDRs      []string
	
	// TLS settings
	TLS             *TLSConfig
	
	// TCP Performance settings
	TCPReadBufferSize  int
	TCPWriteBufferSize int
	WriteDeadlineMS    int
	MaxWriteQueueSize  int
	
	// Protocol settings
	MaxMessageSize  uint32
	
	// Authentication
	AuthTimeout     time.Duration
	
	// Heartbeat settings
	HeartbeatInterval time.Duration
	HeartbeatTimeout  time.Duration
	
	// Data delivery settings
	BatchWindow    time.Duration
	MaxBatchSize   int
}

// DefaultConfig returns default server configuration.
func DefaultConfig() *Config {
	return &Config{
		ListenAddr:         ":8080",
		MaxConnections:     100000,
		ReadTimeout:        30 * time.Second,
		WriteTimeout:       5 * time.Second,
		KeepAlive:          30 * time.Second,
		TLS:                DefaultTLSConfig(),
		TCPReadBufferSize:  65536,  // 64KB
		TCPWriteBufferSize: 65536,  // 64KB
		WriteDeadlineMS:    5000,   // 5s default
		MaxWriteQueueSize:  1000,   // Max queued writes per connection
		MaxMessageSize:     protocol.DefaultMaxMessageSize,
		AuthTimeout:        10 * time.Second,
		HeartbeatInterval:  15 * time.Second,
		HeartbeatTimeout:   20 * time.Second,
		BatchWindow:        5 * time.Millisecond,
		MaxBatchSize:       100,
	}
}

// LoadConfigFromEnv loads configuration from environment variables.
func LoadConfigFromEnv(cfg *Config) {
	if port := os.Getenv("LISTEN_PORT"); port != "" {
		cfg.ListenAddr = ":" + port
	}

	// LISTEN_ADDR takes precedence if provided (e.g., "127.0.0.1:8080" or "[::1]:8080")
	if addr := os.Getenv("LISTEN_ADDR"); addr != "" {
		cfg.ListenAddr = addr
	} else if host := os.Getenv("LISTEN_HOST"); host != "" { // combine host + port
		port := os.Getenv("LISTEN_PORT")
		if port == "" {
			// try to derive from current ListenAddr, else default 8080
			if _, p, err := net.SplitHostPort(cfg.ListenAddr); err == nil && p != "" {
				port = p
			} else if strings.HasPrefix(cfg.ListenAddr, ":") && len(cfg.ListenAddr) > 1 {
				port = cfg.ListenAddr[1:]
			} else {
				port = "8080"
			}
		}
		cfg.ListenAddr = net.JoinHostPort(host, port)
	}
	
	// Load TLS configuration from environment
	if cfg.TLS != nil {
		LoadTLSConfigFromEnv(cfg.TLS)
	}
	
	if interval := os.Getenv("HEARTBEAT_INTERVAL"); interval != "" {
		if d, err := time.ParseDuration(interval); err == nil {
			cfg.HeartbeatInterval = d
		}
	}

	// Backward compatibility (ms variant takes precedence if set)
	if intervalMS := os.Getenv("HEARTBEAT_INTERVAL_MS"); intervalMS != "" {
		if ms, err := strconv.Atoi(intervalMS); err == nil {
			cfg.HeartbeatInterval = time.Duration(ms) * time.Millisecond
		}
	}
	
	if timeout := os.Getenv("HEARTBEAT_TIMEOUT"); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil {
			cfg.HeartbeatTimeout = d
		}
	}

	// Backward compatibility (ms variant takes precedence if set)
	if timeoutMS := os.Getenv("HEARTBEAT_TIMEOUT_MS"); timeoutMS != "" {
		if ms, err := strconv.Atoi(timeoutMS); err == nil {
			cfg.HeartbeatTimeout = time.Duration(ms) * time.Millisecond
		}
	}
	
	if batchWindow := os.Getenv("BATCH_WINDOW"); batchWindow != "" {
		if d, err := time.ParseDuration(batchWindow); err == nil {
			cfg.BatchWindow = d
		}
	}

	// Backward compatibility (ms variant takes precedence if set)
	if batchWindowMS := os.Getenv("BATCH_WINDOW_MS"); batchWindowMS != "" {
		if ms, err := strconv.Atoi(batchWindowMS); err == nil {
			cfg.BatchWindow = time.Duration(ms) * time.Millisecond
		}
	}
	
	// TCP Performance settings
	if readBufSize := os.Getenv("TCP_READ_BUFFER_SIZE"); readBufSize != "" {
		if size, err := strconv.Atoi(readBufSize); err == nil {
			cfg.TCPReadBufferSize = size
		}
	}
	
	if writeBufSize := os.Getenv("TCP_WRITE_BUFFER_SIZE"); writeBufSize != "" {
		if size, err := strconv.Atoi(writeBufSize); err == nil {
			cfg.TCPWriteBufferSize = size
		}
	}
	
	if writeDeadline := os.Getenv("WRITE_DEADLINE_MS"); writeDeadline != "" {
		if ms, err := strconv.Atoi(writeDeadline); err == nil {
			cfg.WriteDeadlineMS = ms
		}
	}
	
	if maxWriteQueue := os.Getenv("MAX_WRITE_QUEUE_SIZE"); maxWriteQueue != "" {
		if size, err := strconv.Atoi(maxWriteQueue); err == nil {
			cfg.MaxWriteQueueSize = size
		}
	}

	if maxBatchSize := os.Getenv("MAX_BATCH_SIZE"); maxBatchSize != "" {
		if size, err := strconv.Atoi(maxBatchSize); err == nil && size > 0 {
			cfg.MaxBatchSize = size
		}
	}
	
	if deadline := os.Getenv("WRITE_DEADLINE_MS"); deadline != "" {
		if d, err := time.ParseDuration(deadline + "ms"); err == nil {
			cfg.WriteTimeout = d
		}
	}

	// IP allow/block lists (comma-separated CIDRs or IPs)
	if v := os.Getenv("IP_ALLOWLIST"); v != "" {
		cfg.AllowCIDRs = splitAndTrimCSV(v)
	}
	if v := os.Getenv("IP_BLOCKLIST"); v != "" {
		cfg.BlockCIDRs = splitAndTrimCSV(v)
	}
}

// Server represents the TCP server.
type Server struct {
	config         *Config
	listener       net.Listener
	authenticator  *auth.Authenticator
	
	// Connection management
	mu             sync.RWMutex
	connections    map[string]*Connection
	activeConns    int32
	
	// Lifecycle management
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	closed         atomic.Bool
	
	// Metrics
	totalConns     uint64
	authSuccess    uint64
	authFailures   uint64
	authRateLimited uint64
	tlsMetrics     *TLSMetrics

	// Security
	ipFilter       *IPFilter
	ddosProtection *DDoSProtection
	
	// Resource management
	resourceMonitor     *ResourceMonitor
	resourceConstraints *ResourceConstraints
	breachHandler       *ResourceBreachHandler
	
	// Health checking
	healthChecker       *HealthChecker
	instanceID          string
	logger              *slog.Logger
	startTime           time.Time
}

// NewServer creates a new TCP server.
func NewServer(config *Config) *Server {
	if config == nil {
		config = DefaultConfig()
	}
	
	LoadConfigFromEnv(config)
	
	ctx, cancel := context.WithCancel(context.Background())
	
	logger := slog.Default()
	instanceID := generateInstanceID()
	
	s := &Server{
		config:         config,
		authenticator:  auth.NewAuthenticator(auth.DefaultConfig()),
		connections:    make(map[string]*Connection),
		ctx:            ctx,
		cancel:         cancel,
		tlsMetrics:     NewTLSMetrics(),
		ddosProtection: NewDDoSProtection(),
		instanceID:     instanceID,
		logger:         logger,
		startTime:      time.Now(),
	}
	
	// Initialize resource management components
	limits := ResourceLimits{
		MaxMemoryMB:       1024,  // 1GB default
		MaxFileDescriptors: 65536, // 64K file descriptors
		MaxGoroutines:     50000,  // 50K goroutines
		MaxConnections:    100000, // 100K connections
		WarningThreshold:  0.8,    // 80% warning
		CriticalThreshold: 0.9,    // 90% critical
	}
	s.resourceMonitor = NewResourceMonitor(limits)
	s.resourceConstraints = NewResourceConstraints()
	s.breachHandler = NewResourceBreachHandler(logger, s.resourceMonitor)
	
	// Initialize health checker
	s.healthChecker = NewHealthChecker(s)
	
	// Initialize auto-scaling support
	s.initAutoScaling()
	
	return s
}

// Start starts the TCP server.
func (s *Server) Start() error {
	if s.closed.Load() {
		return ErrServerClosed
	}
	
	// Validate TLS configuration if enabled
	if s.config.TLS != nil {
		if err := s.config.TLS.ValidateTLSConfig(); err != nil {
			return fmt.Errorf("TLS configuration validation failed: %w", err)
		}
	}
	
	// Build IP filter (no-op if no lists provided)
	if ipf, err := NewIPFilterFromStrings(s.config.AllowCIDRs, s.config.BlockCIDRs); err != nil {
		return fmt.Errorf("invalid IP filter configuration: %w", err)
	} else {
		s.ipFilter = ipf
	}
	
	// Create listener with TLS support if enabled
	listener, err := s.createListener()
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}
	
	s.listener = listener
	
	// Start DDoS protection cleanup routine
	s.ddosProtection.StartCleanupRoutine()
	
	// Start resource monitoring services
	if s.resourceMonitor != nil {
		s.resourceMonitor.Start()
	}
	if s.breachHandler != nil {
		go s.breachHandler.StartMonitoring(s.ctx)
	}
	
	// Start health check server on port 8081
	if err := s.StartHealthCheckServer(8081); err != nil {
		s.logger.Error("failed to start health check server", "error", err)
	}
	
	// Start accepting connections
	s.wg.Add(1)
	go s.acceptLoop()
	
	return nil
}

// createListener creates a network listener with optional TLS support
func (s *Server) createListener() (net.Listener, error) {
	// Create base TCP listener
	listener, err := net.Listen("tcp", s.config.ListenAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", s.config.ListenAddr, err)
	}
	
	// Wrap with TLS if enabled
	if s.config.TLS != nil && s.config.TLS.Enabled {
		tlsConfig, err := s.config.TLS.BuildTLSConfig()
		if err != nil {
			listener.Close()
			return nil, fmt.Errorf("failed to build TLS config: %w", err)
		}
		
		return tls.NewListener(listener, tlsConfig), nil
	}
	
	return listener, nil
}

// Shutdown gracefully shuts down the server without losing connections.
func (s *Server) Shutdown(ctx context.Context) error {
	if !s.closed.CompareAndSwap(false, true) {
		return ErrServerClosed
	}
	
	s.logger.Info("starting graceful shutdown")
	
	// Stop accepting new connections first
	if s.listener != nil {
		s.listener.Close()
		s.logger.Info("stopped accepting new connections")
	}
	
	// Allow existing connections to complete naturally
	// Wait for connections to finish or timeout
	shutdownTimeout := 30 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		shutdownTimeout = time.Until(deadline)
	}
	
	s.logger.Info("waiting for connections to complete", "timeout", shutdownTimeout)
	
	// Create a timeout context for graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()
	
	// Monitor connection count during shutdown
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	for {
		activeConns := atomic.LoadInt32(&s.activeConns)
		if activeConns == 0 {
			s.logger.Info("all connections closed gracefully")
			break
		}
		
		select {
		case <-shutdownCtx.Done():
			s.logger.Warn("shutdown timeout reached, forcing connection closure", 
				"remaining_connections", activeConns)
			s.cancel() // Cancel server context to force close remaining connections
			s.closeAllConnections()
			goto waitForGoroutines
		case <-ticker.C:
			s.logger.Info("waiting for connections to close", "active_connections", activeConns)
		}
	}
	
waitForGoroutines:
	// Wait for all goroutines to finish
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		s.logger.Info("graceful shutdown completed")
		return nil
	case <-ctx.Done():
		s.logger.Error("shutdown context expired")
		return ctx.Err()
	}
}

// Stop gracefully stops the server.
func (s *Server) Stop(ctx context.Context) error {
	if !s.closed.CompareAndSwap(false, true) {
		return nil // Already closed
	}
	
	// Stop accepting new connections
	if s.listener != nil {
		s.listener.Close()
	}
	
	// Cancel server context
	s.cancel()
	
	// Close all active connections
	s.closeAllConnections()
	
	// Wait for all goroutines to finish or context to expire
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// acceptLoop accepts incoming connections.
func (s *Server) acceptLoop() {
	defer s.wg.Done()
	
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.closed.Load() {
				return
			}
			
			// Check if it's a temporary error
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			
			return
		}
		
		// Enforce IP filtering if configured
		if s.ipFilter != nil {
			host, _, _ := net.SplitHostPort(conn.RemoteAddr().String())
			if ip := net.ParseIP(host); !s.ipFilter.Allow(ip) {
				GlobalMetrics.IncrementIPRejectedConnections()
				conn.Close()
				continue
			}
		}
		
		// Check DDoS protection
		if !s.ddosProtection.CheckConnectionAllowed(conn.RemoteAddr()) {
			conn.Close()
			continue
		}
		
		// Check resource breach handler
		if s.breachHandler != nil && s.breachHandler.ShouldRejectConnection() {
			s.breachHandler.RejectConnection(conn)
			continue
		}

		// Check connection limit
		if atomic.LoadInt32(&s.activeConns) >= int32(s.config.MaxConnections) {
			conn.Close()
			continue
		}
		
		// Handle connection
		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

// handleConnection handles a single client connection.
func (s *Server) handleConnection(netConn net.Conn) {
	defer s.wg.Done()
	
	// Record TLS connection metrics if applicable
	if tlsConn, ok := netConn.(*tls.Conn); ok {
		s.tlsMetrics.RecordTLSConnection()
		
		// Perform handshake and record metrics
		start := time.Now()
		err := tlsConn.Handshake()
		handshakeDuration := time.Since(start)
		
		s.tlsMetrics.RecordTLSHandshake(handshakeDuration, err)
		
		if err == nil {
			// Record TLS version and cipher suite
			state := tlsConn.ConnectionState()
			s.tlsMetrics.RecordTLSVersion(state.Version)
			s.tlsMetrics.RecordCipherSuite(state.CipherSuite)
		}
	}
	
	// Update metrics
	atomic.AddInt32(&s.activeConns, 1)
	atomic.AddUint64(&s.totalConns, 1)
	defer atomic.AddInt32(&s.activeConns, -1)
	
	// Configure TCP connection
	if tcpConn, ok := netConn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(s.config.KeepAlive)
		tcpConn.SetNoDelay(true) // Disable Nagle's algorithm for low latency
	}
	
	// Create connection wrapper
	conn := NewConnection(netConn, s.config)
	
	// Register connection
	s.registerConnection(conn)
	defer s.unregisterConnection(conn)
	
	// Record port access for DDoS protection
	if s.ddosProtection != nil {
		s.ddosProtection.RecordPortAccess(netConn.RemoteAddr(), 8080) // Use actual port from config
	}
	
	// Handle the connection
	if err := s.processConnection(conn); err != nil {
		// Log error (in production, use proper logging)
		if !errors.Is(err, context.Canceled) && !errors.Is(err, net.ErrClosed) {
			// Log the error
		}
	}
}

// processConnection processes a client connection.
func (s *Server) processConnection(conn *Connection) error {
	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()
	
	// Set authentication timeout
	authTimer := time.NewTimer(s.config.AuthTimeout)
	defer authTimer.Stop()
	
	// Read first frame (must be AUTH)
	select {
	case <-authTimer.C:
		return conn.SendError(pb.ErrorCode_ERROR_CODE_HEARTBEAT_TIMEOUT, "authentication timeout")
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	
	// Set read deadline for auth
	conn.SetReadDeadline(time.Now().Add(s.config.AuthTimeout))
	
	frame, err := conn.ReadFrame()
	if err != nil {
		return err
	}
	
	// Validate first frame is AUTH
	if err := s.authenticator.ValidateFirstFrame(frame); err != nil {
		// First message must be AUTH
		_ = conn.SendErrorCode(pb.ErrorCode_ERROR_CODE_AUTH_REQUIRED)
		atomic.AddUint64(&s.authFailures, 1)
		return err
	}
	
	// Authenticate
	session, err := s.authenticator.Authenticate(ctx, conn.RemoteAddr(), frame)
	if err != nil {
		// Send specific error codes for better observability
		switch {
		case errors.Is(err, auth.ErrRateLimited):
			_ = conn.SendErrorCode(pb.ErrorCode_ERROR_CODE_RATE_LIMITED)
			atomic.AddUint64(&s.authRateLimited, 1)
		case errors.Is(err, auth.ErrInvalidCredentials):
			_ = conn.SendErrorCode(pb.ErrorCode_ERROR_CODE_INVALID_AUTH)
			atomic.AddUint64(&s.authFailures, 1)
		case errors.Is(err, auth.ErrAlreadyAuthenticated):
			_ = conn.SendErrorCode(pb.ErrorCode_ERROR_CODE_ALREADY_AUTHENTICATED)
			atomic.AddUint64(&s.authFailures, 1)
		default:
			_ = conn.SendAuthError()
			atomic.AddUint64(&s.authFailures, 1)
		}
		return err
	}
	
	// Authentication successful
	atomic.AddUint64(&s.authSuccess, 1)
	conn.SetAuthenticated(session)
	
	// Send AUTH ACK
	if err := conn.SendAuthSuccess(); err != nil {
		return err
	}
	conn.SetReadDeadline(time.Time{})
	
	// Start connection handler
	handler := NewConnectionHandler(conn, s.config, s)
	return handler.Handle(ctx)
}

// registerConnection registers a connection.
func (s *Server) registerConnection(conn *Connection) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.connections[conn.ID()] = conn
}

// unregisterConnection unregisters a connection.
func (s *Server) unregisterConnection(conn *Connection) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	delete(s.connections, conn.ID())
	
	// Clean up authentication session
	s.authenticator.RemoveSession(conn.RemoteAddr())
}

// closeAllConnections closes all active connections.
func (s *Server) closeAllConnections() {
	s.mu.Lock()
	connections := make([]*Connection, 0, len(s.connections))
	for _, conn := range s.connections {
		connections = append(connections, conn)
	}
	s.mu.Unlock()
	
	// Close connections outside of lock
	for _, conn := range connections {
		conn.Close()
	}
}

// GetStats returns server statistics.
func (s *Server) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"active_connections":  atomic.LoadInt32(&s.activeConns),
		"total_connections":   atomic.LoadUint64(&s.totalConns),
		"auth_success":        atomic.LoadUint64(&s.authSuccess),
		"auth_failures":       atomic.LoadUint64(&s.authFailures),
		"auth_rate_limited":   atomic.LoadUint64(&s.authRateLimited),
		"max_connections":     s.config.MaxConnections,
		"listen_addr":         s.config.ListenAddr,
	}
	
	// Add DDoS protection metrics
	if s.ddosProtection != nil {
		ddosMetrics := s.ddosProtection.GetMetrics()
		for k, v := range ddosMetrics {
			stats["ddos_"+k] = v
		}
	}
	
	// Add resource breach handler metrics
	if s.breachHandler != nil {
		breachStats := s.breachHandler.GetBreachStats()
		for k, v := range breachStats {
			stats["resource_"+k] = v
		}
	}
	
	// Add TLS metrics if TLS is enabled
	if s.config.TLS != nil && s.config.TLS.Enabled {
		stats["tls"] = s.tlsMetrics.GetTLSMetrics()
		stats["tls_health"] = s.tlsMetrics.GetTLSHealthStatus()
		stats["tls_config"] = s.config.TLS.GetTLSInfo()
	}
	
	return stats
}

// ListenAddr returns the server's listen address.
func (s *Server) ListenAddr() string {
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return s.config.ListenAddr
}
