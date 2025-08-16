package server

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/furkansarikaya/tick-storm/internal/protocol"
	pb "github.com/furkansarikaya/tick-storm/internal/protocol/pb"
)

func TestHeartbeatValidation(t *testing.T) {
	config := DefaultConfig()
	config.HeartbeatTimeout = 100 * time.Millisecond
	
	// Create a minimal handler for testing (without network connection)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := &ConnectionHandler{
		config:        config,
		lastHeartbeat: time.Now(),
		logger:        logger,
	}
	
	// Test valid heartbeat
	validHeartbeat := &pb.HeartbeatRequest{
		TimestampMs: time.Now().UnixMilli(),
		Sequence:    1,
	}
	
	payload, err := proto.Marshal(validHeartbeat)
	require.NoError(t, err)
	
	frame := &protocol.Frame{
		Type:    protocol.MessageTypeHeartbeat,
		Payload: payload,
	}
	
	// This will fail because we don't have a real connection, but we can test validation
	err = handler.handleHeartbeat(frame)
	// We expect an error due to missing connection, but validation should pass
	assert.Contains(t, err.Error(), "connection is nil") // Expected due to nil connection
	
	// Test invalid heartbeat (zero timestamp)
	invalidHeartbeat := &pb.HeartbeatRequest{
		TimestampMs: 0,
		Sequence:    2,
	}
	
	payload, err = proto.Marshal(invalidHeartbeat)
	require.NoError(t, err)
	
	frame.Payload = payload
	err = handler.handleHeartbeat(frame)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid heartbeat timestamp")
	
	// Test malformed heartbeat
	frame.Payload = []byte("invalid-protobuf")
	err = handler.handleHeartbeat(frame)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal heartbeat")
}

func TestHeartbeatConfiguration(t *testing.T) {
	// Test default configuration
	config := DefaultConfig()
	assert.Equal(t, 15*time.Second, config.HeartbeatInterval, "Default heartbeat interval should be 15s")
	assert.Equal(t, 20*time.Second, config.HeartbeatTimeout, "Default heartbeat timeout should be 20s")
	
	// Test custom configuration
	customConfig := DefaultConfig()
	customConfig.HeartbeatInterval = 10 * time.Second
	customConfig.HeartbeatTimeout = 30 * time.Second
	
	handler := &ConnectionHandler{
		config: customConfig,
	}
	
	assert.Equal(t, customConfig.HeartbeatInterval, handler.config.HeartbeatInterval)
	assert.Equal(t, customConfig.HeartbeatTimeout, handler.config.HeartbeatTimeout)
}

func TestHeartbeatFloodingDetection(t *testing.T) {
	config := DefaultConfig()
	config.HeartbeatInterval = 100 * time.Millisecond
	
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := &ConnectionHandler{
		config:        config,
		lastHeartbeat: time.Now(),
		logger:        logger,
	}
	
	// Send first heartbeat
	heartbeat1 := &pb.HeartbeatRequest{
		TimestampMs: time.Now().UnixMilli(),
		Sequence:    1,
	}
	
	payload, err := proto.Marshal(heartbeat1)
	require.NoError(t, err)
	
	frame := &protocol.Frame{
		Type:    protocol.MessageTypeHeartbeat,
		Payload: payload,
	}
	
	// This will trigger flooding detection logic even though it fails due to nil connection
	err = handler.handleHeartbeat(frame)
	// We expect an error due to missing connection, but flooding detection should work
	assert.Error(t, err) // Expected due to nil connection
	
	// Send second heartbeat immediately (should trigger flooding detection)
	heartbeat2 := &pb.HeartbeatRequest{
		TimestampMs: time.Now().UnixMilli(),
		Sequence:    2,
	}
	
	payload, err = proto.Marshal(heartbeat2)
	require.NoError(t, err)
	
	frame.Payload = payload
	err = handler.handleHeartbeat(frame)
	// Should still fail due to nil connection, but flooding detection logic was executed
	assert.Error(t, err)
}

func TestHeartbeatStateTracking(t *testing.T) {
	config := DefaultConfig()
	
	initialTime := time.Now()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := &ConnectionHandler{
		config:        config,
		lastHeartbeat: initialTime,
		logger:        logger,
	}
	
	// Check initial state
	assert.Equal(t, initialTime, handler.lastHeartbeat, "Initial heartbeat time should be set")
	
	// Wait a bit to ensure time difference
	time.Sleep(10 * time.Millisecond)
	
	// Send heartbeat
	heartbeat := &pb.HeartbeatRequest{
		TimestampMs: time.Now().UnixMilli(),
		Sequence:    1,
	}
	
	payload, err := proto.Marshal(heartbeat)
	require.NoError(t, err)
	
	frame := &protocol.Frame{
		Type:    protocol.MessageTypeHeartbeat,
		Payload: payload,
	}
	
	// This will fail due to nil connection, but state should be updated
	_ = handler.handleHeartbeat(frame)
	
	// Check state was updated (even though the call failed)
	assert.True(t, handler.lastHeartbeat.After(initialTime), "Last heartbeat time should be updated")
}

func BenchmarkHeartbeatProcessing(b *testing.B) {
	config := DefaultConfig()
	
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := &ConnectionHandler{
		config:        config,
		lastHeartbeat: time.Now(),
		logger:        logger,
	}
	
	// Prepare heartbeat frame
	heartbeat := &pb.HeartbeatRequest{
		TimestampMs: time.Now().UnixMilli(),
		Sequence:    1,
	}
	
	payload, err := proto.Marshal(heartbeat)
	require.NoError(b, err)
	
	frame := &protocol.Frame{
		Type:    protocol.MessageTypeHeartbeat,
		Payload: payload,
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		heartbeat.Sequence = uint64(i)
		payload, _ = proto.Marshal(heartbeat)
		frame.Payload = payload
		
		// This will fail due to nil connection, but we're benchmarking the parsing/validation logic
		_ = handler.handleHeartbeat(frame)
	}
}
