// Package protocol implements the Tick-Storm binary framing protocol.
package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"

	"google.golang.org/protobuf/proto"
)

// MessageType represents the type of protocol message.
type MessageType uint8

const (
	// Protocol constants.
	MagicByte1      = 0xF5 // First magic byte
	MagicByte2      = 0x7D // Second magic byte
	ProtocolVersion = 0x01 // Current protocol version

	// Frame structure sizes.
	FrameHeaderSize = 8 // Magic(2) + Ver(1) + Type(1) + Len(4)
	CRCSize         = 4 // CRC32C(4)
	MinFrameSize    = FrameHeaderSize + CRCSize

	// Maximum message size (64KB default).
	DefaultMaxMessageSize = 64 * 1024

	// Message types
	MessageTypeAuth      MessageType = 0x01
	MessageTypeSubscribe MessageType = 0x02
	MessageTypeHeartbeat MessageType = 0x03
	MessageTypeDataBatch MessageType = 0x04
	MessageTypeError     MessageType = 0x05
	MessageTypeACK       MessageType = 0x06
	MessageTypePong      MessageType = 0x07
)

var (
	// ErrInvalidMagic indicates invalid magic bytes in frame header.
	ErrInvalidMagic = errors.New("invalid magic bytes")

	// ErrUnsupportedVersion indicates unsupported protocol version.
	ErrUnsupportedVersion = errors.New("unsupported protocol version")

	// ErrInvalidChecksum indicates checksum mismatch.
	ErrInvalidChecksum = errors.New("invalid checksum")

	// ErrMessageTooLarge indicates message exceeds maximum size.
	ErrMessageTooLarge = errors.New("message too large")

	// ErrAuthTimeout indicates authentication timeout.
	ErrAuthTimeout = errors.New("authentication timeout")

	// ErrInvalidSubscription indicates invalid subscription request.
	ErrInvalidSubscription = errors.New("invalid subscription")

	// ErrAlreadySubscribed indicates client already has a subscription.
	ErrAlreadySubscribed = errors.New("already subscribed")

	// ErrRateLimited indicates rate limit exceeded.
	ErrRateLimited = errors.New("rate limited")

	// ErrHeartbeatTimeout indicates heartbeat timeout.
	ErrHeartbeatTimeout = errors.New("heartbeat timeout")

	// ErrInvalidSequence indicates invalid message sequence.
	ErrInvalidSequence = errors.New("invalid message sequence")

	// ErrInvalidMessageType indicates invalid message type.
	ErrInvalidMessageType = errors.New("invalid message type")

	// ErrIncompleteFrame indicates incomplete frame data.
	ErrIncompleteFrame = errors.New("incomplete frame")
)

// MagicBytes represents the protocol magic bytes.
var MagicBytes = [2]byte{MagicByte1, MagicByte2}

// Frame represents a protocol frame.
type Frame struct {
	Magic   [2]byte
	Version uint8
	Type    MessageType
	Length  uint32
	Payload []byte
	CRC     uint32
}

// Marshal serializes the frame into wire format.
func (f *Frame) Marshal() ([]byte, error) {
	if len(f.Payload) > DefaultMaxMessageSize {
		return nil, ErrMessageTooLarge
	}

	// Calculate total size
	totalSize := FrameHeaderSize + len(f.Payload) + CRCSize
	buf := bytes.NewBuffer(make([]byte, 0, totalSize))

	// Write magic bytes
	buf.WriteByte(MagicByte1)
	buf.WriteByte(MagicByte2)

	// Write version
	buf.WriteByte(f.Version)

	// Write message type
	buf.WriteByte(uint8(f.Type))

	// Write payload length (big-endian)
	if err := binary.Write(buf, binary.BigEndian, uint32(len(f.Payload))); err != nil {
		return nil, fmt.Errorf("failed to write payload length: %w", err)
	}

	// Write payload
	buf.Write(f.Payload)

	// Calculate and write CRC32C checksum
	data := buf.Bytes()
	checksum := crc32.Checksum(data[:len(data)], crc32.MakeTable(crc32.Castagnoli))
	if err := binary.Write(buf, binary.BigEndian, checksum); err != nil {
		return nil, fmt.Errorf("failed to write checksum: %w", err)
	}

	return buf.Bytes(), nil
}

// Unmarshal deserializes a frame from wire format.
func (f *Frame) Unmarshal(data []byte) error {
	if len(data) < MinFrameSize {
		return ErrIncompleteFrame
	}

	// Verify magic bytes
	if data[0] != MagicByte1 || data[1] != MagicByte2 {
		return ErrInvalidMagic
	}

	// Extract version
	f.Version = data[2]
	if f.Version != ProtocolVersion {
		return ErrUnsupportedVersion
	}

	// Extract message type
	f.Type = MessageType(data[3])

	// Extract payload length
	payloadLen := binary.BigEndian.Uint32(data[4:8])
	if payloadLen > DefaultMaxMessageSize {
		return ErrMessageTooLarge
	}

	// Verify total frame size
	expectedSize := FrameHeaderSize + int(payloadLen) + CRCSize
	if len(data) != expectedSize {
		return ErrIncompleteFrame
	}

	// Extract payload
	f.Payload = make([]byte, payloadLen)
	copy(f.Payload, data[FrameHeaderSize:FrameHeaderSize+payloadLen])

	// Verify CRC32C checksum
	checksumStart := FrameHeaderSize + int(payloadLen)
	providedChecksum := binary.BigEndian.Uint32(data[checksumStart:])
	calculatedChecksum := crc32.Checksum(data[:checksumStart], crc32.MakeTable(crc32.Castagnoli))

	if providedChecksum != calculatedChecksum {
		return ErrInvalidChecksum
	}

	return nil
}

// FrameReader reads frames from an io.Reader.
type FrameReader struct {
	r              io.Reader
	maxMessageSize uint32
}

// NewFrameReader creates a new frame reader.
func NewFrameReader(r io.Reader, maxMessageSize uint32) *FrameReader {
	if maxMessageSize == 0 {
		maxMessageSize = DefaultMaxMessageSize
	}
	return &FrameReader{
		r:              r,
		maxMessageSize: maxMessageSize,
	}
}

// ReadFrame reads a single frame from the reader.
func (r *FrameReader) ReadFrame() (*Frame, error) {
	// Read header
	header := make([]byte, FrameHeaderSize)
	if _, err := io.ReadFull(r.r, header); err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	// Verify magic bytes
	if header[0] != MagicByte1 || header[1] != MagicByte2 {
		return nil, ErrInvalidMagic
	}

	// Extract frame details
	if err := ValidateVersion(header[2]); err != nil {
		return nil, fmt.Errorf("version validation failed: %w", err)
	}

	msgType := header[3]
	payloadLen := binary.BigEndian.Uint32(header[4:8])

	if payloadLen > r.maxMessageSize {
		return nil, ErrMessageTooLarge
	}

	// Read payload and checksum
	remainder := make([]byte, payloadLen+CRCSize)
	if _, err := io.ReadFull(r.r, remainder); err != nil {
		return nil, fmt.Errorf("failed to read payload and checksum: %w", err)
	}

	// Verify checksum
	fullFrame := append(header, remainder...)
	checksumStart := FrameHeaderSize + int(payloadLen)
	providedChecksum := binary.BigEndian.Uint32(fullFrame[checksumStart:])
	calculatedChecksum := crc32.Checksum(fullFrame[:checksumStart], crc32.MakeTable(crc32.Castagnoli))

	if providedChecksum != calculatedChecksum {
		return nil, ErrInvalidChecksum
	}

	// Create frame
	frame := &Frame{
		Version: header[2],
		Type:    MessageType(msgType),
		Payload: make([]byte, payloadLen),
	}
	copy(frame.Payload, remainder[:payloadLen])

	return frame, nil
}

// FrameWriter writes frames to an io.Writer.
type FrameWriter struct {
	w              io.Writer
	maxMessageSize uint32
}

// NewFrameWriter creates a new frame writer.
func NewFrameWriter(w io.Writer) *FrameWriter {
	return &FrameWriter{
		w:              w,
		maxMessageSize: DefaultMaxMessageSize,
	}
}

// WriteFrame writes a single frame to the writer.
func (w *FrameWriter) WriteFrame(frame *Frame) error {
	if len(frame.Payload) > int(w.maxMessageSize) {
		return ErrMessageTooLarge
	}

	data, err := frame.Marshal()
	if err != nil {
		return err
	}

	if _, err := w.w.Write(data); err != nil {
		return fmt.Errorf("failed to write frame: %w", err)
	}

	return nil
}

// MarshalMessage marshals a protobuf message into a frame.
func MarshalMessage(msgType MessageType, msg proto.Message) (*Frame, error) {
	payload, err := proto.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal protobuf message: %w", err)
	}

	return &Frame{
		Version: ProtocolVersion,
		Type:    msgType,
		Payload: payload,
	}, nil
}

// UnmarshalMessage unmarshals a frame payload into a protobuf message.
func UnmarshalMessage(frame *Frame, msg proto.Message) error {
	if err := proto.Unmarshal(frame.Payload, msg); err != nil {
		return fmt.Errorf("failed to unmarshal protobuf message: %w", err)
	}
	return nil
}
