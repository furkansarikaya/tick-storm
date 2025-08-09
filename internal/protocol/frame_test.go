package protocol

import (
	"bytes"
	"testing"
)

func TestFrameMarshalUnmarshal(t *testing.T) {
	tests := []struct {
		name    string
		frame   Frame
		wantErr bool
	}{
		{
			name: "valid frame with small payload",
			frame: Frame{
				Version: ProtocolVersion,
				Type:    1, // AUTH
				Payload: []byte("test payload"),
			},
			wantErr: false,
		},
		{
			name: "valid frame with empty payload",
			frame: Frame{
				Version: ProtocolVersion,
				Type:    3, // HEARTBEAT
				Payload: []byte{},
			},
			wantErr: false,
		},
		{
			name: "valid frame with binary payload",
			frame: Frame{
				Version: ProtocolVersion,
				Type:    4, // DATA_BATCH
				Payload: []byte{0x00, 0x01, 0x02, 0x03, 0xFF},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal the frame
			data, err := tt.frame.Marshal()
			if (err != nil) != tt.wantErr {
				t.Errorf("Frame.Marshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			// Unmarshal the frame
			var decoded Frame
			if err := decoded.Unmarshal(data); err != nil {
				t.Errorf("Frame.Unmarshal() error = %v", err)
				return
			}

			// Verify the decoded frame matches the original
			if decoded.Version != tt.frame.Version {
				t.Errorf("Version mismatch: got %v, want %v", decoded.Version, tt.frame.Version)
			}
			if decoded.Type != tt.frame.Type {
				t.Errorf("Type mismatch: got %v, want %v", decoded.Type, tt.frame.Type)
			}
			if !bytes.Equal(decoded.Payload, tt.frame.Payload) {
				t.Errorf("Payload mismatch: got %v, want %v", decoded.Payload, tt.frame.Payload)
			}
		})
	}
}

func TestFrameUnmarshalErrors(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr error
	}{
		{
			name:    "incomplete frame",
			data:    []byte{MagicByte1, MagicByte2},
			wantErr: ErrIncompleteFrame,
		},
		{
			name:    "invalid magic bytes",
			data:    []byte{0xFF, 0xFF, ProtocolVersion, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			wantErr: ErrInvalidMagic,
		},
		{
			name:    "unsupported version",
			data:    []byte{MagicByte1, MagicByte2, 0xFF, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			wantErr: ErrUnsupportedVersion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var frame Frame
			err := frame.Unmarshal(tt.data)
			if err != tt.wantErr {
				t.Errorf("Frame.Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFrameChecksumValidation(t *testing.T) {
	// Create a valid frame
	frame := Frame{
		Version: ProtocolVersion,
		Type:    1,
		Payload: []byte("test"),
	}

	// Marshal it
	data, err := frame.Marshal()
	if err != nil {
		t.Fatalf("Failed to marshal frame: %v", err)
	}

	// Corrupt the checksum (last 4 bytes)
	data[len(data)-1] ^= 0xFF

	// Try to unmarshal - should fail with checksum error
	var decoded Frame
	err = decoded.Unmarshal(data)
	if err != ErrInvalidChecksum {
		t.Errorf("Expected ErrInvalidChecksum, got %v", err)
	}
}

func TestFrameMaxMessageSize(t *testing.T) {
	// Create a frame with payload exceeding max size
	frame := Frame{
		Version: ProtocolVersion,
		Type:    4,
		Payload: make([]byte, DefaultMaxMessageSize+1),
	}

	// Marshal should fail
	_, err := frame.Marshal()
	if err != ErrMessageTooLarge {
		t.Errorf("Expected ErrMessageTooLarge, got %v", err)
	}
}

func BenchmarkFrameMarshal(b *testing.B) {
	frame := Frame{
		Version: ProtocolVersion,
		Type:    4,
		Payload: make([]byte, 1024), // 1KB payload
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = frame.Marshal()
	}
}

func BenchmarkFrameUnmarshal(b *testing.B) {
	frame := Frame{
		Version: ProtocolVersion,
		Type:    4,
		Payload: make([]byte, 1024), // 1KB payload
	}
	data, _ := frame.Marshal()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var f Frame
		_ = f.Unmarshal(data)
	}
}
