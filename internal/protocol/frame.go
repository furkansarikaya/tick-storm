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

const (
	// Protocol constants.
	MagicByte1 = 0xF5 // First magic byte
	MagicByte2 = 0x7D // Second magic byte
	Version    = 0x01 // Current protocol version

	// Frame structure sizes.
	HeaderSize = 8  // Magic(2) + Ver(1) + Type(1) + Len(4)
	CRCSize    = 4  // CRC32C(4)
	MinFrameSize = HeaderSize + CRCSize

	// Maximum message size (64KB default).
	DefaultMaxMessageSize = 64 * 1024
)

var (
	// ErrInvalidMagic indicates invalid magic bytes in frame header.
	ErrInvalidMagic = errors.New("invalid magic bytes")
	
	// ErrUnsupportedVersion indicates unsupported protocol version.
	ErrUnsupportedVersion = errors.New("unsupported protocol version")
	
	// ErrInvalidChecksum indicates CRC32C checksum mismatch.
	ErrInvalidChecksum = errors.New("invalid checksum")
	
	// ErrMessageTooLarge indicates message exceeds maximum size.
	ErrMessageTooLarge = errors.New("message too large")
	
	// ErrInvalidMessageType indicates unknown message type.
	ErrInvalidMessageType = errors.New("invalid message type")
	
	// ErrIncompleteFrame indicates incomplete frame data.
	ErrIncompleteFrame = errors.New("incomplete frame")
)

// Frame represents a protocol frame.
// Structure: [Magic(2B)][Ver(1B)][Type(1B)][Len(4B)][Payload][CRC32C(4B)]
type Frame struct {
	Version uint8
	Type    uint8
	Payload []byte
}

// Marshal serializes the frame into wire format.
func (f *Frame) Marshal() ([]byte, error) {
	if len(f.Payload) > DefaultMaxMessageSize {
		return nil, ErrMessageTooLarge
	}

	// Calculate total size
	totalSize := HeaderSize + len(f.Payload) + CRCSize
	buf := bytes.NewBuffer(make([]byte, 0, totalSize))

	// Write magic bytes
	buf.WriteByte(MagicByte1)
	buf.WriteByte(MagicByte2)

	// Write version
	buf.WriteByte(f.Version)

	// Write message type
	buf.WriteByte(f.Type)

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
	if f.Version != Version {
		return ErrUnsupportedVersion
	}

	// Extract message type
	f.Type = data[3]

	// Extract payload length
	payloadLen := binary.BigEndian.Uint32(data[4:8])
	if payloadLen > DefaultMaxMessageSize {
		return ErrMessageTooLarge
	}

	// Verify total frame size
	expectedSize := HeaderSize + int(payloadLen) + CRCSize
	if len(data) != expectedSize {
		return ErrIncompleteFrame
	}

	// Extract payload
	f.Payload = make([]byte, payloadLen)
	copy(f.Payload, data[HeaderSize:HeaderSize+payloadLen])

	// Verify CRC32C checksum
	checksumStart := HeaderSize + int(payloadLen)
	providedChecksum := binary.BigEndian.Uint32(data[checksumStart:])
	calculatedChecksum := crc32.Checksum(data[:checksumStart], crc32.MakeTable(crc32.Castagnoli))
	
	if providedChecksum != calculatedChecksum {
		return ErrInvalidChecksum
	}

	return nil
}

// Reader reads frames from an io.Reader.
type Reader struct {
	r             io.Reader
	maxMessageSize uint32
}

// NewReader creates a new frame reader.
func NewReader(r io.Reader) *Reader {
	return &Reader{
		r:             r,
		maxMessageSize: DefaultMaxMessageSize,
	}
}

// ReadFrame reads a single frame from the reader.
func (r *Reader) ReadFrame() (*Frame, error) {
	// Read header
	header := make([]byte, HeaderSize)
	if _, err := io.ReadFull(r.r, header); err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	// Verify magic bytes
	if header[0] != MagicByte1 || header[1] != MagicByte2 {
		return nil, ErrInvalidMagic
	}

	// Extract frame details
	version := header[2]
	if version != Version {
		return nil, ErrUnsupportedVersion
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
	checksumStart := HeaderSize + int(payloadLen)
	providedChecksum := binary.BigEndian.Uint32(fullFrame[checksumStart:])
	calculatedChecksum := crc32.Checksum(fullFrame[:checksumStart], crc32.MakeTable(crc32.Castagnoli))
	
	if providedChecksum != calculatedChecksum {
		return nil, ErrInvalidChecksum
	}

	// Create frame
	frame := &Frame{
		Version: version,
		Type:    msgType,
		Payload: make([]byte, payloadLen),
	}
	copy(frame.Payload, remainder[:payloadLen])

	return frame, nil
}

// Writer writes frames to an io.Writer.
type Writer struct {
	w              io.Writer
	maxMessageSize uint32
}

// NewWriter creates a new frame writer.
func NewWriter(w io.Writer) *Writer {
	return &Writer{
		w:              w,
		maxMessageSize: DefaultMaxMessageSize,
	}
}

// WriteFrame writes a single frame to the writer.
func (w *Writer) WriteFrame(frame *Frame) error {
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
func MarshalMessage(msgType uint8, msg proto.Message) (*Frame, error) {
	payload, err := proto.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal protobuf message: %w", err)
	}

	return &Frame{
		Version: Version,
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
