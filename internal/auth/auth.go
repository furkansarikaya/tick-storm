// Package auth implements authentication mechanisms for the Tick-Storm server.
package auth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/furkansarikaya/tick-storm/internal/protocol"
	pb "github.com/furkansarikaya/tick-storm/internal/protocol/pb"
	"google.golang.org/protobuf/proto"
)

var (
	// ErrInvalidCredentials indicates authentication failed due to invalid credentials.
	ErrInvalidCredentials = errors.New("invalid credentials")
	
	// ErrAuthRequired indicates authentication is required but not provided.
	ErrAuthRequired = errors.New("authentication required")
	
	// ErrAlreadyAuthenticated indicates the connection is already authenticated.
	ErrAlreadyAuthenticated = errors.New("already authenticated")
	
	// ErrAuthTimeout indicates authentication timed out.
	ErrAuthTimeout = errors.New("authentication timeout")
	
	// ErrRateLimited indicates too many authentication attempts.
	ErrRateLimited = errors.New("rate limited")
	
	// ErrFirstFrameMustBeAuth indicates the first frame must be an AUTH frame.
	ErrFirstFrameMustBeAuth = errors.New("first frame must be AUTH")
)

// Config holds authentication configuration.
type Config struct {
	Username        string
	Password        string
	Timeout         time.Duration
	MaxAttempts     int
	RateLimitWindow time.Duration
}

// DefaultConfig returns default authentication configuration.
func DefaultConfig() *Config {
	return &Config{
		Username:        os.Getenv("STREAM_USER"),
		Password:        os.Getenv("STREAM_PASS"),
		Timeout:         30 * time.Second,
		MaxAttempts:     3,
		RateLimitWindow: 1 * time.Minute,
	}
}

// Authenticator handles authentication for connections.
type Authenticator struct {
	config      *Config
	rateLimiter *RateLimiter
	mu          sync.RWMutex
	sessions    map[string]*Session
}

// Session represents an authenticated session.
type Session struct {
	ClientID      string
	Username      string
	Authenticated bool
	AuthTime      time.Time
	LastActivity  time.Time
}

// NewAuthenticator creates a new authenticator.
func NewAuthenticator(config *Config) *Authenticator {
	if config == nil {
		config = DefaultConfig()
	}
	
	return &Authenticator{
		config:      config,
		rateLimiter: NewRateLimiter(config.MaxAttempts, config.RateLimitWindow),
		sessions:    make(map[string]*Session),
	}
}

// ValidateFirstFrame validates that the first frame is an AUTH frame.
func (a *Authenticator) ValidateFirstFrame(frame *protocol.Frame) error {
	if frame.Type != protocol.MessageTypeAuth {
		return ErrFirstFrameMustBeAuth
	}
	return nil
}

// Authenticate processes an authentication request.
func (a *Authenticator) Authenticate(ctx context.Context, clientAddr string, frame *protocol.Frame) (*Session, error) {
	// Check rate limiting
	if !a.rateLimiter.Allow(clientAddr) {
		return nil, ErrRateLimited
	}
	
	// Check if already authenticated
	a.mu.RLock()
	if session, exists := a.sessions[clientAddr]; exists && session.Authenticated {
		a.mu.RUnlock()
		return nil, ErrAlreadyAuthenticated
	}
	a.mu.RUnlock()
	
	// Parse AUTH request
	var authReq pb.AuthRequest
	if err := proto.Unmarshal(frame.Payload, &authReq); err != nil {
		return nil, fmt.Errorf("failed to unmarshal auth request: %w", err)
	}
	
	// Validate credentials
	if authReq.Username != a.config.Username || authReq.Password != a.config.Password {
		a.rateLimiter.RecordFailure(clientAddr)
		return nil, ErrInvalidCredentials
	}
	
	// Create session
	session := &Session{
		ClientID:      authReq.ClientId,
		Username:      authReq.Username,
		Authenticated: true,
		AuthTime:      time.Now(),
		LastActivity:  time.Now(),
	}
	
	// Store session
	a.mu.Lock()
	a.sessions[clientAddr] = session
	a.mu.Unlock()
	
	// Reset rate limiter on successful auth
	a.rateLimiter.Reset(clientAddr)
	
	return session, nil
}

// GetSession retrieves a session for a client.
func (a *Authenticator) GetSession(clientAddr string) (*Session, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	session, exists := a.sessions[clientAddr]
	return session, exists
}

// RemoveSession removes a session.
func (a *Authenticator) RemoveSession(clientAddr string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	delete(a.sessions, clientAddr)
}

// UpdateActivity updates the last activity time for a session.
func (a *Authenticator) UpdateActivity(clientAddr string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	if session, exists := a.sessions[clientAddr]; exists {
		session.LastActivity = time.Now()
	}
}

// IsAuthenticated checks if a client is authenticated.
func (a *Authenticator) IsAuthenticated(clientAddr string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	session, exists := a.sessions[clientAddr]
	return exists && session.Authenticated
}

// CreateAckResponse creates an ACK response frame.
func CreateAckResponse() *protocol.Frame {
	ack := &pb.AckResponse{
		AckType:     pb.MessageType_MESSAGE_TYPE_AUTH,
		Success:     true,
		Message:     "Authentication successful",
		TimestampMs: time.Now().UnixMilli(),
	}
	
	payload, _ := proto.Marshal(ack)
	return &protocol.Frame{
		Version: protocol.ProtocolVersion,
		Type:    protocol.MessageTypeACK,
		Payload: payload,
	}
}

// CreateErrorResponse creates an ERROR response frame.
func CreateErrorResponse(code pb.ErrorCode, message string) *protocol.Frame {
	errorMsg := &pb.ErrorResponse{
		Code:        code,
		Message:     message,
		TimestampMs: time.Now().UnixMilli(),
	}

	payload, _ := proto.Marshal(errorMsg)
	return &protocol.Frame{
		Version: protocol.ProtocolVersion,
		Type:    protocol.MessageTypeError,
		Payload: payload,
	}
}
