package protocol

import (
	"bytes"
	"fmt"
	"hash/crc32"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCRC32CValidation(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
	}{
		{
			name:    "empty payload",
			payload: []byte{},
		},
		{
			name:    "small payload",
			payload: []byte("hello world"),
		},
		{
			name:    "large payload",
			payload: bytes.Repeat([]byte("test data "), 1000),
		},
		{
			name:    "binary payload",
			payload: []byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE, 0xFD},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create frame
			frame := &Frame{
				Version: ProtocolVersion,
				Type:    MessageTypeHeartbeat,
				Payload: tt.payload,
			}

			// Marshal frame
			data, err := frame.Marshal()
			require.NoError(t, err)

			// Unmarshal frame
			parsedFrame := &Frame{}
			err = parsedFrame.Unmarshal(data)
			require.NoError(t, err)

			// Verify frame contents
			assert.Equal(t, frame.Version, parsedFrame.Version)
			assert.Equal(t, frame.Type, parsedFrame.Type)
			assert.Equal(t, frame.Payload, parsedFrame.Payload)
		})
	}
}

func TestCRC32CCorruption(t *testing.T) {
	// Create a valid frame
	frame := &Frame{
		Version: ProtocolVersion,
		Type:    MessageTypeHeartbeat,
		Payload: []byte("test payload"),
	}

	data, err := frame.Marshal()
	require.NoError(t, err)

	tests := []struct {
		name         string
		corruptIndex int
		corruptValue byte
	}{
		{
			name:         "corrupt magic byte 1",
			corruptIndex: 0,
			corruptValue: 0x00,
		},
		{
			name:         "corrupt magic byte 2",
			corruptIndex: 1,
			corruptValue: 0x00,
		},
		{
			name:         "corrupt version",
			corruptIndex: 2,
			corruptValue: 0xFF,
		},
		{
			name:         "corrupt message type",
			corruptIndex: 3,
			corruptValue: 0xFF,
		},
		{
			name:         "corrupt payload length",
			corruptIndex: 4,
			corruptValue: 0xFF,
		},
		{
			name:         "corrupt payload data",
			corruptIndex: FrameHeaderSize,
			corruptValue: 0xFF,
		},
		{
			name:         "corrupt checksum",
			corruptIndex: len(data) - 1,
			corruptValue: 0xFF,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Corrupt the data
			corruptedData := make([]byte, len(data))
			copy(corruptedData, data)
			corruptedData[tt.corruptIndex] = tt.corruptValue

			// Try to unmarshal corrupted frame
			parsedFrame := &Frame{}
			err := parsedFrame.Unmarshal(corruptedData)
			
			// Should fail with appropriate error
			assert.Error(t, err)
			
			// Verify specific error types
			switch tt.corruptIndex {
			case 0, 1:
				assert.Equal(t, ErrInvalidMagic, err)
			case 2:
				assert.Equal(t, ErrUnsupportedVersion, err)
			case len(data) - 1, len(data) - 2, len(data) - 3, len(data) - 4:
				assert.Equal(t, ErrInvalidChecksum, err)
			}
		})
	}
}

func TestFrameReaderChecksumValidation(t *testing.T) {
	// Create valid frame
	frame := &Frame{
		Version: ProtocolVersion,
		Type:    MessageTypeHeartbeat,
		Payload: []byte("test payload for frame reader"),
	}

	data, err := frame.Marshal()
	require.NoError(t, err)

	// Test valid frame
	t.Run("valid frame", func(t *testing.T) {
		reader := NewFrameReader(bytes.NewReader(data), DefaultMaxMessageSize)
		parsedFrame, err := reader.ReadFrame()
		require.NoError(t, err)
		assert.Equal(t, frame.Type, parsedFrame.Type)
		assert.Equal(t, frame.Payload, parsedFrame.Payload)
	})

	// Test corrupted frame
	t.Run("corrupted checksum", func(t *testing.T) {
		corruptedData := make([]byte, len(data))
		copy(corruptedData, data)
		// Corrupt the last byte (checksum)
		corruptedData[len(corruptedData)-1] ^= 0xFF

		reader := NewFrameReader(bytes.NewReader(corruptedData), DefaultMaxMessageSize)
		_, err := reader.ReadFrame()
		assert.Equal(t, ErrInvalidChecksum, err)
	})
}

func TestCRC32CPerformance(t *testing.T) {
	// Create frames of different sizes
	sizes := []int{100, 1000, 10000, 64000} // Up to max message size

	for _, size := range sizes {
		t.Run(fmt.Sprintf("size_%d", size), func(t *testing.T) {
			payload := make([]byte, size)
			for i := range payload {
				payload[i] = byte(i % 256)
			}

			frame := &Frame{
				Version: ProtocolVersion,
				Type:    MessageTypeDataBatch,
				Payload: payload,
			}

			// Benchmark marshaling (includes checksum calculation)
			data, err := frame.Marshal()
			require.NoError(t, err)

			// Benchmark unmarshaling (includes checksum validation)
			parsedFrame := &Frame{}
			err = parsedFrame.Unmarshal(data)
			require.NoError(t, err)
			assert.Equal(t, payload, parsedFrame.Payload)
		})
	}
}

func BenchmarkCRC32CCalculation(b *testing.B) {
	sizes := []int{100, 1000, 10000, 64000}
	
	for _, size := range sizes {
		b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
			data := make([]byte, size)
			for i := range data {
				data[i] = byte(i % 256)
			}
			
			table := crc32.MakeTable(crc32.Castagnoli)
			
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = crc32.Checksum(data, table)
			}
		})
	}
}

func BenchmarkFrameMarshalUnmarshal(b *testing.B) {
	payload := make([]byte, 1000)
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	frame := &Frame{
		Version: ProtocolVersion,
		Type:    MessageTypeDataBatch,
		Payload: payload,
	}

	b.Run("marshal", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := frame.Marshal()
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	data, _ := frame.Marshal()
	
	b.Run("unmarshal", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			parsedFrame := &Frame{}
			err := parsedFrame.Unmarshal(data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
