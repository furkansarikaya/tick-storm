package server

import (
	"net"
	"testing"

	"github.com/furkansarikaya/tick-storm/internal/protocol"
	"github.com/furkansarikaya/tick-storm/internal/protocol/pb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewVersionHandler(t *testing.T) {
	vh := NewVersionHandler()
	require.NotNil(t, vh)
	assert.True(t, vh.IsVersionSupported(0x01))
	assert.False(t, vh.IsVersionSupported(0x99))
}

func TestVersionHandler_ValidateFrameVersion(t *testing.T) {
	vh := NewVersionHandler()

	t.Run("valid frame", func(t *testing.T) {
		frame := &protocol.Frame{
			Version: 0x01,
			Type:    protocol.MessageTypeAuth,
			Payload: []byte("test"),
		}

		err := vh.ValidateFrameVersion(frame)
		assert.NoError(t, err)
	})

	t.Run("nil frame", func(t *testing.T) {
		err := vh.ValidateFrameVersion(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "frame is nil")
	})

	t.Run("unsupported version", func(t *testing.T) {
		frame := &protocol.Frame{
			Version: 0x99,
			Type:    protocol.MessageTypeAuth,
			Payload: []byte("test"),
		}

		err := vh.ValidateFrameVersion(frame)
		assert.Error(t, err)
	})
}

func TestVersionHandler_GetVersionCapabilities(t *testing.T) {
	vh := NewVersionHandler()

	t.Run("valid version", func(t *testing.T) {
		capabilities, err := vh.GetVersionCapabilities(0x01)
		require.NoError(t, err)
		assert.True(t, capabilities.Authentication)
		assert.True(t, capabilities.CRC32Checksum)
	})

	t.Run("invalid version", func(t *testing.T) {
		capabilities, err := vh.GetVersionCapabilities(0x99)
		assert.Error(t, err)
		assert.Nil(t, capabilities)
	})
}

func TestVersionHandler_HandleVersionSpecificMessage(t *testing.T) {
	vh := NewVersionHandler()

	t.Run("valid message", func(t *testing.T) {
		frame := &protocol.Frame{
			Version: 0x01,
			Type:    protocol.MessageTypeAuth,
			Payload: []byte("test"),
		}

		handlerCalled := false
		handler := func(f *protocol.Frame) error {
			handlerCalled = true
			assert.Equal(t, frame, f)
			return nil
		}

		err := vh.HandleVersionSpecificMessage(frame, handler)
		assert.NoError(t, err)
		assert.True(t, handlerCalled)
	})

	t.Run("unsupported message type", func(t *testing.T) {
		frame := &protocol.Frame{
			Version: 0x01,
			Type:    protocol.MessageType(0xFF), // Unsupported type
			Payload: []byte("test"),
		}

		handler := func(f *protocol.Frame) error {
			return nil
		}

		err := vh.HandleVersionSpecificMessage(frame, handler)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not supported in version")
	})
}

func TestVersionHandler_isMessageTypeSupported(t *testing.T) {
	vh := NewVersionHandler()
	capabilities := &protocol.VersionFeatures{
		Authentication: true,
		Subscription:   true,
		Heartbeat:     true,
		DataBatch:     true,
		ErrorReporting: true,
	}

	tests := []struct {
		name     string
		msgType  protocol.MessageType
		expected bool
	}{
		{"auth", protocol.MessageTypeAuth, true},
		{"subscribe", protocol.MessageTypeSubscribe, true},
		{"heartbeat", protocol.MessageTypeHeartbeat, true},
		{"data_batch", protocol.MessageTypeDataBatch, true},
		{"error", protocol.MessageTypeError, true},
		{"ack", protocol.MessageTypeACK, true},
		{"pong", protocol.MessageTypePong, true},
		{"unknown", protocol.MessageType(0xFF), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := vh.isMessageTypeSupported(tt.msgType, capabilities)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestVersionHandler_CreateVersionSpecificErrorResponse(t *testing.T) {
	vh := NewVersionHandler()

	t.Run("valid version with error reporting", func(t *testing.T) {
		errorResp, err := vh.CreateVersionSpecificErrorResponse(0x01, pb.ErrorCode_ERROR_CODE_INVALID_AUTH, "test error")
		require.NoError(t, err)
		assert.Equal(t, pb.ErrorCode_ERROR_CODE_INVALID_AUTH, errorResp.Code)
		assert.Equal(t, "test error", errorResp.Message)
		assert.True(t, errorResp.TimestampMs > 0)
		assert.Contains(t, errorResp.Details, "Version 0x01 error")
	})

	t.Run("unsupported version", func(t *testing.T) {
		errorResp, err := vh.CreateVersionSpecificErrorResponse(0x99, pb.ErrorCode_ERROR_CODE_INVALID_AUTH, "test error")
		assert.Error(t, err)
		assert.Nil(t, errorResp)
	})
}

func TestVersionHandler_NegotiateVersion(t *testing.T) {
	vh := NewVersionHandler()

	t.Run("compatible version", func(t *testing.T) {
		version, err := vh.NegotiateVersion(0x01)
		assert.NoError(t, err)
		assert.Equal(t, uint8(0x01), version)
	})

	t.Run("incompatible version", func(t *testing.T) {
		version, err := vh.NegotiateVersion(0x99)
		assert.Error(t, err)
		assert.Equal(t, uint8(0), version)
		assert.Contains(t, err.Error(), "no compatible version found")
	})
}

func TestVersionHandler_GetVersionMetrics(t *testing.T) {
	vh := NewVersionHandler()

	// Record some usage
	frame := &protocol.Frame{Version: 0x01, Type: protocol.MessageTypeAuth, Payload: []byte("test")}
	vh.ValidateFrameVersion(frame)
	vh.ValidateFrameVersion(frame)

	metrics := vh.GetVersionMetrics()
	require.NotNil(t, metrics)
	
	versionCounts := metrics["version_counts"].(map[uint8]int64)
	assert.Equal(t, int64(2), versionCounts[0x01])
}

func TestVersionAwareConnectionHandler(t *testing.T) {
	// Create mock connection
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	config := DefaultConfig()
	conn := NewConnection(server, config)
	defer conn.Close()

	vh := NewVersionAwareConnectionHandler(conn)
	require.NotNil(t, vh)

	t.Run("initial state", func(t *testing.T) {
		assert.Equal(t, uint8(protocol.CurrentProtocolVersion), vh.GetClientVersion())
	})

	t.Run("set client version", func(t *testing.T) {
		vh.SetClientVersion(0x01)
		assert.Equal(t, uint8(0x01), vh.GetClientVersion())
	})

	t.Run("get version capabilities", func(t *testing.T) {
		capabilities, err := vh.GetVersionCapabilities()
		require.NoError(t, err)
		assert.True(t, capabilities.Authentication)
	})
}

func TestVersionAwareConnectionHandler_ProcessFrameWithVersionCheck(t *testing.T) {
	// Create mock connection
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	config := DefaultConfig()
	conn := NewConnection(server, config)
	defer conn.Close()

	vh := NewVersionAwareConnectionHandler(conn)

	t.Run("valid frame", func(t *testing.T) {
		frame := &protocol.Frame{
			Version: 0x01,
			Type:    protocol.MessageTypeAuth,
			Payload: []byte("test"),
		}

		// This will fail due to missing auth handler, but version check should pass
		err := vh.ProcessFrameWithVersionCheck(frame)
		// We expect an error from the handler, not from version validation
		if err != nil {
			assert.NotContains(t, err.Error(), "version")
		}
	})

	t.Run("incompatible version", func(t *testing.T) {
		frame := &protocol.Frame{
			Version: 0x99,
			Type:    protocol.MessageTypeAuth,
			Payload: []byte("test"),
		}

		err := vh.ProcessFrameWithVersionCheck(frame)
		assert.Error(t, err)
		// Should fail at version negotiation or validation
	})
}

func TestVersionAwareConnectionHandler_SendVersionSpecificError(t *testing.T) {
	// Create mock connection
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	config := DefaultConfig()
	conn := NewConnection(server, config)
	defer conn.Close()

	vh := NewVersionAwareConnectionHandler(conn)

	// This will fail due to connection issues, but we can test the error creation
	err := vh.SendVersionSpecificError(pb.ErrorCode_ERROR_CODE_INVALID_AUTH, "test error")
	// We expect a connection error, not a version error
	if err != nil {
		assert.NotContains(t, err.Error(), "version")
	}
}

func TestGlobalVersionHandler(t *testing.T) {
	vh1 := GetGlobalVersionHandler()
	vh2 := GetGlobalVersionHandler()

	// Should return the same instance
	assert.Same(t, vh1, vh2)
	assert.True(t, vh1.IsVersionSupported(0x01))
}

func BenchmarkVersionHandler_ValidateFrameVersion(b *testing.B) {
	vh := NewVersionHandler()
	frame := &protocol.Frame{
		Version: 0x01,
		Type:    protocol.MessageTypeAuth,
		Payload: []byte("test"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vh.ValidateFrameVersion(frame)
	}
}

func BenchmarkVersionHandler_HandleVersionSpecificMessage(b *testing.B) {
	vh := NewVersionHandler()
	frame := &protocol.Frame{
		Version: 0x01,
		Type:    protocol.MessageTypeAuth,
		Payload: []byte("test"),
	}

	handler := func(f *protocol.Frame) error {
		return nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vh.HandleVersionSpecificMessage(frame, handler)
	}
}

func BenchmarkVersionAwareConnectionHandler_ProcessFrameWithVersionCheck(b *testing.B) {
	// Create mock connection
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	config := DefaultConfig()
	conn := NewConnection(server, config)
	defer conn.Close()

	vh := NewVersionAwareConnectionHandler(conn)
	frame := &protocol.Frame{
		Version: 0x01,
		Type:    protocol.MessageTypeAuth,
		Payload: []byte("test"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vh.ProcessFrameWithVersionCheck(frame)
	}
}
