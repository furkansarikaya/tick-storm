package auth

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/furkansarikaya/tick-storm/internal/protocol"
	pb "github.com/furkansarikaya/tick-storm/internal/protocol/pb"
	"google.golang.org/protobuf/proto"
)

func TestAuthenticator_ValidateFirstFrame(t *testing.T) {
	auth := NewAuthenticator(nil)

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
			err := auth.ValidateFirstFrame(tt.frame)
			if err != tt.wantErr {
				t.Errorf("ValidateFirstFrame() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAuthenticator_Authenticate(t *testing.T) {
	// Set test credentials
	os.Setenv("STREAM_USER", "testuser")
	os.Setenv("STREAM_PASS", "testpass")
	defer os.Unsetenv("STREAM_USER")
	defer os.Unsetenv("STREAM_PASS")

	config := DefaultConfig()
	auth := NewAuthenticator(config)
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
			name:      "invalid credentials",
			clientAddr: "192.168.1.1:12345",
			frame: &protocol.Frame{
				Type:    protocol.MessageTypeAuth,
				Payload: []byte("invalid"),
			},
			wantErr: errors.New("invalid auth payload"),
		},
		{
			name:      "invalid auth request",
			clientAddr: "192.168.1.1:12345",
			frame: &protocol.Frame{
				Type: protocol.MessageTypeAuth,
				Payload: func() []byte {
					invalidAuthReq := &pb.AuthRequest{
						Username: "wronguser",
						Password: "wrongpass",
						ClientId: "test-client-2",
					}
					invalidPayload, _ := proto.Marshal(invalidAuthReq)
					return invalidPayload
				}(),
			},
			wantErr: errors.New("invalid auth payload"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, err := auth.Authenticate(ctx, tt.clientAddr, tt.frame)
			
			if err != tt.wantErr {
				t.Errorf("Authenticate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if tt.wantErr == nil {
				if session == nil {
					t.Error("Expected session to be non-nil for successful auth")
				} else if !session.Authenticated {
					t.Error("Expected session to be authenticated")
				}
			}
		})
	}
}

func TestAuthenticator_AlreadyAuthenticated(t *testing.T) {
	os.Setenv("STREAM_USER", "testuser")
	os.Setenv("STREAM_PASS", "testpass")
	defer os.Unsetenv("STREAM_USER")
	defer os.Unsetenv("STREAM_PASS")

	config := DefaultConfig()
	auth := NewAuthenticator(config)
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
	_, err := auth.Authenticate(ctx, clientAddr, frame)
	if err != nil {
		t.Fatalf("First authentication failed: %v", err)
	}

	// Second authentication should fail with already authenticated error
	_, err = auth.Authenticate(ctx, clientAddr, frame)
	if err != ErrAlreadyAuthenticated {
		t.Errorf("Expected ErrAlreadyAuthenticated, got %v", err)
	}
}

func TestAuthenticator_SessionManagement(t *testing.T) {
	auth := NewAuthenticator(nil)
	clientAddr := "192.168.1.1:12345"

	// Initially no session
	session, exists := auth.GetSession(clientAddr)
	if exists {
		t.Error("Expected no session initially")
	}

	// Add session manually for testing
	testSession := &Session{
		ClientID:      "test-client",
		Username:      "testuser",
		Authenticated: true,
		AuthTime:      time.Now(),
		LastActivity:  time.Now(),
	}
	auth.sessions[clientAddr] = testSession

	// Should find session
	session, exists = auth.GetSession(clientAddr)
	if !exists {
		t.Error("Expected to find session")
	}
	if session.ClientID != "test-client" {
		t.Errorf("Expected ClientID 'test-client', got %s", session.ClientID)
	}

	// Check if authenticated
	if !auth.IsAuthenticated(clientAddr) {
		t.Error("Expected client to be authenticated")
	}

	// Update activity
	oldActivity := session.LastActivity
	time.Sleep(10 * time.Millisecond)
	auth.UpdateActivity(clientAddr)
	
	session, _ = auth.GetSession(clientAddr)
	if !session.LastActivity.After(oldActivity) {
		t.Error("Expected LastActivity to be updated")
	}

	// Remove session
	auth.RemoveSession(clientAddr)
	_, exists = auth.GetSession(clientAddr)
	if exists {
		t.Error("Expected session to be removed")
	}
}

func TestRateLimiter_Allow(t *testing.T) {
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

func TestRateLimiter_RecordFailure(t *testing.T) {
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
	// Test creating auth response frame
	authResp := &pb.AuthRequest{
		Username: "testuser",
		Password: "testpass",
		ClientId: "test-session",
	}
	payload, err := proto.Marshal(authResp)
	if err != nil {
		t.Fatalf("Failed to marshal auth response: %v", err)
	}
	frame := &protocol.Frame{
		Type:    protocol.MessageTypeAuth,
		Payload: payload,
	}

	if frame.Type != protocol.MessageTypeAuth {
		t.Errorf("Expected AUTH type, got %d", frame.Type)
	}

	// Unmarshal and verify
	var ack pb.AckResponse
	if err := proto.Unmarshal(frame.Payload, &ack); err != nil {
		t.Fatalf("Failed to unmarshal ACK response: %v", err)
	}

	if ack.AckType != pb.MessageType_MESSAGE_TYPE_AUTH {
		t.Errorf("Expected AUTH ack type, got %v", ack.AckType)
	}
	if !ack.Success {
		t.Error("Expected success to be true")
	}
}

func TestCreateErrorResponse(t *testing.T) {
	// Test creating error response frame
	errorResp := &pb.ErrorResponse{
		Code:    pb.ErrorCode_ERROR_CODE_INVALID_MESSAGE,
		Message: "test error",
	}
	payload, err := proto.Marshal(errorResp)
	if err != nil {
		t.Fatalf("Failed to marshal error response: %v", err)
	}
	frame := &protocol.Frame{
		Type:    protocol.MessageTypeError,
		Payload: payload,
	}

	// Verify frame structure
	if frame.Type != protocol.MessageTypeAuth {
		t.Errorf("Expected frame type %d, got %d", protocol.MessageTypeAuth, frame.Type)
	}
	if len(frame.Payload) == 0 {
		t.Error("Expected non-empty payload")
	}
}
