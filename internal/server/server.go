// Package server implements the TCP server for Tick-Storm.
package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
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
	
	if interval := os.Getenv("HEARTBEAT_INTERVAL"); interval != "" {
		if d, err := time.ParseDuration(interval); err == nil {
			cfg.HeartbeatInterval = d
		}
	}
	
	if timeout := os.Getenv("HEARTBEAT_TIMEOUT"); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil {
			cfg.HeartbeatTimeout = d
		}
	}
	
	if batchWindow := os.Getenv("BATCH_WINDOW"); batchWindow != "" {
		if d, err := time.ParseDuration(batchWindow); err == nil {
			cfg.BatchWindow = d
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
}

// NewServer creates a new TCP server.
func NewServer(config *Config) *Server {
	if config == nil {
		config = DefaultConfig()
	}
	
	LoadConfigFromEnv(config)
	
	ctx, cancel := context.WithCancel(context.Background())
	
	return &Server{
		config:        config,
		authenticator: auth.NewAuthenticator(auth.DefaultConfig()),
		connections:   make(map[string]*Connection),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start starts the TCP server.
func (s *Server) Start() error {
	if s.closed.Load() {
		return ErrServerClosed
	}
	
	// Create listener with support for both IPv4 and IPv6
	listener, err := net.Listen("tcp", s.config.ListenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.config.ListenAddr, err)
	}
	
	s.listener = listener
	
	// Start accepting connections
	s.wg.Add(1)
	go s.acceptLoop()
	
	return nil
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
		conn.SendAuthError()
		atomic.AddUint64(&s.authFailures, 1)
		return err
	}
	
	// Authenticate
	session, err := s.authenticator.Authenticate(ctx, conn.RemoteAddr(), frame)
	if err != nil {
		conn.SendAuthError()
		atomic.AddUint64(&s.authFailures, 1)
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
	handler := NewConnectionHandler(conn, s.config)
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
	return map[string]interface{}{
		"active_connections":  atomic.LoadInt32(&s.activeConns),
		"total_connections":   atomic.LoadUint64(&s.totalConns),
		"auth_success":        atomic.LoadUint64(&s.authSuccess),
		"auth_failures":       atomic.LoadUint64(&s.authFailures),
		"max_connections":     s.config.MaxConnections,
		"listen_addr":         s.config.ListenAddr,
	}
}

// ListenAddr returns the server's listen address.
func (s *Server) ListenAddr() string {
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return s.config.ListenAddr
}
