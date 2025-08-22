package auth

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/furkansarikaya/tick-storm/internal/protocol"
	"github.com/furkansarikaya/tick-storm/internal/protocol/pb"
	"google.golang.org/protobuf/proto"
)

func TestAuthenticatorValidateFirstFrame(t *testing.T) {
	authenticator := NewAuthenticator(DefaultConfig())

	tests := []struct {
		name    string
		frame   *protocol.Frame
		wantErr error
	}{
		{
			name: "valid auth frame",
			frame: &protocol.Frame{
				Type: protocol.MessageTypeAuth,
			},
			wantErr: nil,
		},
		{
			name: "non-auth frame",
			frame: &protocol.Frame{
				Type: protocol.MessageTypeSubscribe,
			},
			wantErr: errors.New("first frame must be AUTH"),
		},
		{
			name: "invalid first frame - HEARTBEAT",
			frame: &protocol.Frame{
				Type: protocol.MessageTypeHeartbeat,
			},
			wantErr: errors.New("first frame must be AUTH"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := authenticator.ValidateFirstFrame(tt.frame)
			if tt.wantErr != nil {
				if err == nil || err.Error() != tt.wantErr.Error() {
					t.Errorf("ValidateFirstFrame() error = %v, wantErr %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("ValidateFirstFrame() unexpected error = %v", err)
			}
		})
	}
}

func TestAuthenticatorAuthenticate(t *testing.T) {
	// Set test credentials
	os.Setenv("STREAM_USER", "testuser")
	os.Setenv("STREAM_PASS", "testpass")
	defer os.Unsetenv("STREAM_USER")
	defer os.Unsetenv("STREAM_PASS")

	config := DefaultConfig()
	ctx := context.Background()

	// Create valid auth request
	validAuthReq := &pb.AuthRequest{
		Username: "testuser",
		Password: "testpass",
		ClientId: "test-client-1",
		Version:  "1.0.0",
	}
	validPayload, _ := proto.Marshal(validAuthReq)

	tests := []struct {
		name      string
		clientAddr string
		frame     *protocol.Frame
		wantErr   error
	}{
		{
			name:      "valid credentials",
			clientAddr: "192.168.1.1:12345",
			frame: &protocol.Frame{
				Type:    protocol.MessageTypeAuth,
				Payload: validPayload,
			},
			wantErr: nil,
		},
		{
			name: "invalid credentials",
			frame: &protocol.Frame{
				Type:    protocol.MessageTypeAuth,
				Payload: []byte("invalid"),
			},
			wantErr: errors.New("failed to unmarshal auth request"),
		},
		{
			name: "invalid auth request",
			frame: &protocol.Frame{
				Type:    protocol.MessageTypeAuth,
				Payload: []byte("invalid"),
			},
			wantErr: errors.New("failed to unmarshal auth request"),
		},
		{
			name:      "invalid credentials with valid protobuf",
			clientAddr: "192.168.1.1:12345",
			frame: &protocol.Frame{
				Type: protocol.MessageTypeAuth,
				Payload: func() []byte {
					invalidAuthReq := &pb.AuthRequest{
						Username: "wronguser",
						Password: "wrongpass",
						ClientId: "test-client-2",
						Version:  "1.0.0",
					}
					invalidPayload, _ := proto.Marshal(invalidAuthReq)
					return invalidPayload
				}(),
			},
			wantErr: errors.New("invalid credentials"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh authenticator for each test to avoid state pollution
			testAuth := NewAuthenticator(config)
			remoteAddr := "127.0.0.1:12345"
			_, err := testAuth.Authenticate(ctx, remoteAddr, tt.frame)
			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("Authenticate() expected error %v, got nil", tt.wantErr)
				} else if !strings.Contains(err.Error(), tt.wantErr.Error()) {
					t.Errorf("Authenticate() error = %v, wantErr %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("Authenticate() unexpected error = %v", err)
			}
		})
	}
}

func TestAuthenticatorAlreadyAuthenticated(t *testing.T) {
	os.Setenv("STREAM_USER", "testuser")
	os.Setenv("STREAM_PASS", "testpass")
	defer os.Unsetenv("STREAM_USER")
	defer os.Unsetenv("STREAM_PASS")

	config := DefaultConfig()
	authenticator := NewAuthenticator(config)
	ctx := context.Background()
	clientAddr := "192.168.1.1:12345"

	authReq := &pb.AuthRequest{
		Username: "testuser",
		Password: "testpass",
		ClientId: "test-client",
	}
	payload, _ := proto.Marshal(authReq)
	frame := &protocol.Frame{
		Type:    protocol.MessageTypeAuth,
		Payload: payload,
	}

	// First authentication should succeed
	_, err := authenticator.Authenticate(ctx, clientAddr, frame)
	if err != nil {
		t.Fatalf("First authentication failed: %v", err)
	}

	// Second authentication should fail with already authenticated error
	_, err = authenticator.Authenticate(ctx, clientAddr, frame)
	if err != ErrAlreadyAuthenticated {
		t.Errorf("Expected ErrAlreadyAuthenticated, got %v", err)
	}
}

func TestAuthenticatorSessionManagement(t *testing.T) {
	// Set test credentials first
	os.Setenv("STREAM_USER", "testuser")
	os.Setenv("STREAM_PASS", "testpass")
	defer os.Unsetenv("STREAM_USER")
	defer os.Unsetenv("STREAM_PASS")

	authenticator := NewAuthenticator(DefaultConfig())
	clientAddr := "192.168.1.1:12345"

	// Create auth frame
	frame := &protocol.Frame{
		Type: protocol.MessageTypeAuth,
		Payload: func() []byte {
			authReq := &pb.AuthRequest{
				Username: "testuser",
				Password: "testpass",
				ClientId: "test-client",
				Version:  "1.0.0",
			}
			payload, _ := proto.Marshal(authReq)
			return payload
		}(),
	}

	ctx := context.Background()
	session, err := authenticator.Authenticate(ctx, clientAddr, frame)
	if err != nil {
		t.Fatalf("Authentication failed: %v", err)
	}

	// Check session after authentication
	if !session.Authenticated {
		t.Error("Expected session to be authenticated")
	}

	// Remove session
	authenticator.RemoveSession(clientAddr)

	// Remove session
	authenticator.RemoveSession(clientAddr)
}

func TestAuthenticatorSessionAfterAuth(t *testing.T) {
	// Set test credentials
	os.Setenv("STREAM_USER", "testuser")
	os.Setenv("STREAM_PASS", "testpass")
	defer os.Unsetenv("STREAM_USER")
	defer os.Unsetenv("STREAM_PASS")

	// Authenticate
	ctx := context.Background()
	clientAddr := "192.168.1.1:12345"
	authenticator := NewAuthenticator(DefaultConfig())
	frame := &protocol.Frame{
		Type: protocol.MessageTypeAuth,
		Payload: func() []byte {
			authReq := &pb.AuthRequest{
				Username: "testuser",
				Password: "testpass",
				ClientId: "test-client",
				Version:  "1.0.0",
			}
			payload, _ := proto.Marshal(authReq)
			return payload
		}(),
	}
	session, err := authenticator.Authenticate(ctx, clientAddr, frame)
	if err != nil {
		t.Fatalf("Authentication failed: %v", err)
	}

	// Check session after authentication
	if !session.Authenticated {
		t.Error("Expected session to be authenticated")
	}

	// Remove session
	authenticator.RemoveSession(clientAddr)
}

func TestRateLimiterAllow(t *testing.T) {
	rl := NewRateLimiter(3, 1*time.Minute)
	clientAddr := "192.168.1.1:12345"

	// First 3 attempts should be allowed
	for i := 0; i < 3; i++ {
		if !rl.Allow(clientAddr) {
			t.Errorf("Attempt %d should be allowed", i+1)
		}
	}

	// 4th attempt should be blocked
	if rl.Allow(clientAddr) {
		t.Error("4th attempt should be blocked")
	}

	// Reset should allow attempts again
	rl.Reset(clientAddr)
	if !rl.Allow(clientAddr) {
		t.Error("Should allow after reset")
	}
}

func TestRateLimiterRecordFailure(t *testing.T) {
	rl := NewRateLimiter(2, 1*time.Minute)
	clientAddr := "192.168.1.1:12345"

	// Use up attempts
	rl.Allow(clientAddr)
	rl.Allow(clientAddr)
	
	// Record failure should increase blocking time
	rl.RecordFailure(clientAddr)
	
	// Should still be blocked
	if rl.Allow(clientAddr) {
		t.Error("Should be blocked after recording failure")
	}
}

func TestCreateAuthResponse(t *testing.T) {
	// Test creating ACK response
	frame := CreateAckResponse()
	
	if frame == nil {
		t.Fatal("Expected non-nil frame")
	}
	
	if frame.Type != protocol.MessageTypeACK {
		t.Errorf("Expected ACK type, got %d", frame.Type)
	}
	
	// Unmarshal and verify
	var ackResp pb.AckResponse
	if err := proto.Unmarshal(frame.Payload, &ackResp); err != nil {
		t.Fatalf("Failed to unmarshal ACK response: %v", err)
	}
	
	if ackResp.AckType != pb.MessageType_MESSAGE_TYPE_AUTH {
		t.Errorf("Expected AUTH ack type, got %v", ackResp.AckType)
	}
	if !ackResp.Success {
		t.Error("Expected success to be true")
	}
	if ackResp.Message != "Authentication successful" {
		t.Errorf("Expected message 'Authentication successful', got %q", ackResp.Message)
	}
}

func TestCreateErrorResponse(t *testing.T) {
	frame := CreateErrorResponse(pb.ErrorCode_ERROR_CODE_INVALID_AUTH, "test error")
	
	if frame == nil {
		t.Fatal("Expected non-nil frame")
	}
	
	if frame.Type != protocol.MessageTypeError {
		t.Errorf("Expected ERROR type %d, got %d", protocol.MessageTypeError, frame.Type)
	}
	
	// Unmarshal and verify
	var errorResp pb.ErrorResponse
	if err := proto.Unmarshal(frame.Payload, &errorResp); err != nil {
		t.Fatalf("Failed to unmarshal error response: %v", err)
	}
	
	if errorResp.Code != pb.ErrorCode_ERROR_CODE_INVALID_AUTH {
		t.Errorf("Expected error code %v, got %v", pb.ErrorCode_ERROR_CODE_INVALID_AUTH, errorResp.Code)
	}
	if errorResp.Message != "test error" {
		t.Errorf("Expected error message %q, got %q", "test error", errorResp.Message)
	}
}

func TestAuthenticatorRateLimitingPerIPIgnoresPort(t *testing.T) {
    // Configure credentials (we will send wrong creds to avoid limiter reset)
    os.Setenv("STREAM_USER", "gooduser")
    os.Setenv("STREAM_PASS", "goodpass")
    os.Setenv("AUTH_MAX_ATTEMPTS", "1")
    os.Setenv("AUTH_RATE_LIMIT_WINDOW", "50ms")
    defer os.Unsetenv("STREAM_USER")
    defer os.Unsetenv("STREAM_PASS")
    defer os.Unsetenv("AUTH_MAX_ATTEMPTS")
    defer os.Unsetenv("AUTH_RATE_LIMIT_WINDOW")

    cfg := DefaultConfig()
    a := NewAuthenticator(cfg)
    ctx := context.Background()

    // Build an AUTH frame with invalid credentials
    badReq := &pb.AuthRequest{Username: "wrong", Password: "wrong", ClientId: "c1", Version: "1.0.0"}
    badPayload, _ := proto.Marshal(badReq)
    frame := &protocol.Frame{Type: protocol.MessageTypeAuth, Payload: badPayload}

    // First attempt from IP 10.0.0.1 (port 10000) should be allowed by limiter,
    // but fail with invalid credentials.
    if _, err := a.Authenticate(ctx, "10.0.0.1:10000", frame); err != ErrInvalidCredentials {
        t.Fatalf("expected ErrInvalidCredentials on first attempt, got %v", err)
    }

    // Second attempt from same IP but different port should be rate limited.
    if _, err := a.Authenticate(ctx, "10.0.0.1:20000", frame); err != ErrRateLimited {
        t.Fatalf("expected ErrRateLimited on second attempt (same IP, different port), got %v", err)
    }

    // Attempt from a different IP should not be affected by the limiter state of 10.0.0.1.
    if _, err := a.Authenticate(ctx, "10.0.0.2:30000", frame); err != ErrInvalidCredentials {
        t.Fatalf("expected ErrInvalidCredentials for different IP, got %v", err)
    }
}

func TestAuthenticatorEnvOverridesAffectRateLimiter(t *testing.T) {
    os.Setenv("STREAM_USER", "gooduser")
    os.Setenv("STREAM_PASS", "goodpass")
    os.Setenv("AUTH_MAX_ATTEMPTS", "2")
    os.Setenv("AUTH_RATE_LIMIT_WINDOW", "100ms")
    defer os.Unsetenv("STREAM_USER")
    defer os.Unsetenv("STREAM_PASS")
    defer os.Unsetenv("AUTH_MAX_ATTEMPTS")
    defer os.Unsetenv("AUTH_RATE_LIMIT_WINDOW")

    cfg := DefaultConfig()
    if cfg.MaxAttempts != 2 {
        t.Fatalf("expected MaxAttempts from env to be 2, got %d", cfg.MaxAttempts)
    }
    if cfg.RateLimitWindow != 100*time.Millisecond {
        t.Fatalf("expected RateLimitWindow from env to be 100ms, got %v", cfg.RateLimitWindow)
    }

    a := NewAuthenticator(cfg)
    ctx := context.Background()

    // Build an AUTH frame with invalid credentials
    badReq := &pb.AuthRequest{Username: "wrong", Password: "wrong", ClientId: "c2", Version: "1.0.0"}
    badPayload, _ := proto.Marshal(badReq)
    frame := &protocol.Frame{Type: protocol.MessageTypeAuth, Payload: badPayload}

    // Two attempts within window should be allowed by limiter (then invalid creds error)
    if _, err := a.Authenticate(ctx, "192.168.100.1:1111", frame); err != ErrInvalidCredentials {
        t.Fatalf("first attempt expected ErrInvalidCredentials, got %v", err)
    }
    if _, err := a.Authenticate(ctx, "192.168.100.1:2222", frame); err != ErrInvalidCredentials {
        t.Fatalf("second attempt expected ErrInvalidCredentials, got %v", err)
    }

    // Next attempt should be rate limited (blocked)
    if _, err := a.Authenticate(ctx, "192.168.100.1:3333", frame); err != ErrRateLimited {
        t.Fatalf("third attempt expected ErrRateLimited, got %v", err)
    }

    // Block period is window*3 due to RecordFailure penalty; wait a bit longer
    time.Sleep(350 * time.Millisecond)

    // After block period, attempts should be allowed again by limiter
    if _, err := a.Authenticate(ctx, "192.168.100.1:4444", frame); err != ErrInvalidCredentials {
        t.Fatalf("post-block attempt expected ErrInvalidCredentials, got %v", err)
    }
}
