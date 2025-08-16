// Package server implements the TCP server for Tick-Storm.
package server

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/furkansarikaya/tick-storm/internal/auth"
	"github.com/furkansarikaya/tick-storm/internal/protocol"
	"github.com/furkansarikaya/tick-storm/internal/protocol/pb"
	"google.golang.org/protobuf/proto"
)

// WriteQueueItem represents an item in the write queue
type WriteQueueItem struct {
	frame    *protocol.Frame
	deadline time.Time
	done     chan error
}

// Connection represents a client connection.
type Connection struct {
	id            string
	conn          net.Conn
	reader        *protocol.FrameReader
	writer        *protocol.FrameWriter
	config        *Config
	pools         *ObjectPools
	
	// Authentication
	authenticated bool
	session       *auth.Session
	
	// State management
	mu            sync.RWMutex
	closed        atomic.Bool
	subscription  *Subscription
	
	// Write queue for async writes
	writeQueue    chan *WriteQueueItem
	writeQueueWg  sync.WaitGroup
	
	// Metrics
	messagesRecv  uint64
	messagesSent  uint64
	bytesRecv     uint64
	bytesSent     uint64
	lastActivity  time.Time
	writeQueueLen int32 // Atomic counter for queue length
}

// NewConnection creates a new connection wrapper.
func NewConnection(conn net.Conn, config *Config) *Connection {
	id := fmt.Sprintf("%s-%d", conn.RemoteAddr().String(), time.Now().UnixNano())
	
	// Apply TCP optimizations
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		// Enable TCP_NODELAY to disable Nagle's algorithm for low latency
		if err := tcpConn.SetNoDelay(true); err != nil {
			// Log error but continue - not critical
		}
		
		// Set optimized buffer sizes
		if err := tcpConn.SetReadBuffer(config.TCPReadBufferSize); err != nil {
			// Log error but continue
		}
		if err := tcpConn.SetWriteBuffer(config.TCPWriteBufferSize); err != nil {
			// Log error but continue
		}
	}
	
	c := &Connection{
		id:           id,
		conn:         conn,
		reader:       protocol.NewFrameReader(conn, config.MaxMessageSize),
		writer:       protocol.NewFrameWriter(conn),
		config:       config,
		pools:        GetGlobalPools(),
		writeQueue:   make(chan *WriteQueueItem, config.MaxWriteQueueSize),
		lastActivity: time.Now(),
	}
	
	// Start async write loop
	c.writeQueueWg.Add(1)
	go c.writeLoop()
	
	return c
}

// ID returns the connection ID.
func (c *Connection) ID() string {
	return c.id
}

// RemoteAddr returns the remote address.
func (c *Connection) RemoteAddr() string {
	if c == nil || c.conn == nil {
		return "unknown"
	}
	return c.conn.RemoteAddr().String()
}

// SetAuthenticated marks the connection as authenticated.
func (c *Connection) SetAuthenticated(session *auth.Session) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.authenticated = true
	c.session = session
}

// IsAuthenticated returns whether the connection is authenticated.
func (c *Connection) IsAuthenticated() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return c.authenticated
}

// SetSubscription sets the connection's subscription.
func (c *Connection) SetSubscription(sub *Subscription) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.subscription != nil {
		return fmt.Errorf("connection already has a subscription")
	}
	
	c.subscription = sub
	return nil
}

// GetSubscription returns the connection's subscription.
func (c *Connection) GetSubscription() *Subscription {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return c.subscription
}

// ReadFrame reads a frame from the connection.
func (c *Connection) ReadFrame() (*protocol.Frame, error) {
	if c.closed.Load() {
		return nil, net.ErrClosed
	}
	
	frame, err := c.reader.ReadFrame()
	if err != nil {
		return nil, err
	}
	
	// Update metrics
	atomic.AddUint64(&c.messagesRecv, 1)
	atomic.AddUint64(&c.bytesRecv, uint64(len(frame.Payload)+protocol.FrameHeaderSize))
	
	c.mu.Lock()
	c.lastActivity = time.Now()
	c.mu.Unlock()
	
	return frame, nil
}

// WriteFrame writes a frame to the connection using async write queue.
func (c *Connection) WriteFrame(frame *protocol.Frame) error {
	return c.WriteFrameAsync(frame)
}

// SendMessage sends a protobuf message with the given type.
func (c *Connection) SendMessage(msgType protocol.MessageType, msg proto.Message) error {
	frame, err := protocol.MarshalMessage(msgType, msg)
	if err != nil {
		return err
	}
	
	return c.WriteFrame(frame)
}

// SendAuthSuccess sends an authentication success ACK.
func (c *Connection) SendAuthSuccess() error {
	ack := &pb.AckResponse{
		AckType: pb.MessageType_MESSAGE_TYPE_AUTH,
		Success: true,
		Message: "Authentication successful",
		TimestampMs: time.Now().UnixMilli(),
	}
	
	frame, err := protocol.MarshalMessage(protocol.MessageTypeACK, ack)
	if err != nil {
		return err
	}
	return c.WriteFrame(frame)
}

// SendAuthError sends an authentication error message.
func (c *Connection) SendAuthError() error {
	errMsg := &pb.ErrorResponse{
		Code:        pb.ErrorCode_ERROR_CODE_INVALID_AUTH,
		Message:     "Authentication failed",
		TimestampMs: time.Now().UnixMilli(),
	}
	
	frame, err := protocol.MarshalMessage(protocol.MessageTypeError, errMsg)
	if err != nil {
		return err
	}
	return c.WriteFrame(frame)
}

// SendError sends an error message with optional details.
func (c *Connection) SendError(code pb.ErrorCode, message string) error {
	return c.SendErrorWithDetails(code, message, "")
}

// SendErrorWithDetails sends an error message with detailed information.
func (c *Connection) SendErrorWithDetails(code pb.ErrorCode, message, details string) error {
	errMsg := &pb.ErrorResponse{
		Code:        code,
		Message:     message,
		Details:     details,
		TimestampMs: time.Now().UnixMilli(),
	}
	
	frame, err := protocol.MarshalMessage(protocol.MessageTypeError, errMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal error response: %w", err)
	}
	return c.WriteFrame(frame)
}

// SendErrorCode sends a predefined error with standard message.
func (c *Connection) SendErrorCode(code pb.ErrorCode) error {
	message, details := getStandardErrorMessage(code)
	return c.SendErrorWithDetails(code, message, details)
}

// getStandardErrorMessage returns standard error messages and details for error codes.
func getStandardErrorMessage(code pb.ErrorCode) (message, details string) {
	switch code {
	case pb.ErrorCode_ERROR_CODE_INVALID_AUTH:
		return "Authentication failed", "Invalid username or password provided"
	case pb.ErrorCode_ERROR_CODE_AUTH_REQUIRED:
		return "Authentication required", "AUTH frame must be the first message sent"
	case pb.ErrorCode_ERROR_CODE_ALREADY_AUTHENTICATED:
		return "Already authenticated", "Connection has already been authenticated"
	case pb.ErrorCode_ERROR_CODE_INVALID_SUBSCRIPTION:
		return "Invalid subscription request", "Subscription mode or parameters are invalid"
	case pb.ErrorCode_ERROR_CODE_ALREADY_SUBSCRIBED:
		return "Already subscribed", "Connection already has an active subscription"
	case pb.ErrorCode_ERROR_CODE_NOT_SUBSCRIBED:
		return "Not subscribed", "No active subscription found for this connection"
	case pb.ErrorCode_ERROR_CODE_HEARTBEAT_TIMEOUT:
		return "Heartbeat timeout", "Client failed to send heartbeat within configured interval"
	case pb.ErrorCode_ERROR_CODE_INVALID_MESSAGE:
		return "Invalid message format", "Message could not be parsed or contains invalid data"
	case pb.ErrorCode_ERROR_CODE_CHECKSUM_FAILED:
		return "Checksum validation failed", "Frame CRC32C checksum does not match calculated value"
	case pb.ErrorCode_ERROR_CODE_PROTOCOL_VERSION:
		return "Unsupported protocol version", "Client protocol version is not supported by server"
	case pb.ErrorCode_ERROR_CODE_MESSAGE_TOO_LARGE:
		return "Message too large", "Message size exceeds maximum allowed limit"
	case pb.ErrorCode_ERROR_CODE_RATE_LIMITED:
		return "Rate limited", "Too many requests sent within the allowed time window"
	case pb.ErrorCode_ERROR_CODE_INTERNAL_ERROR:
		return "Internal server error", "An unexpected error occurred on the server"
	default:
		return "Unknown error", "An unrecognized error code was encountered"
	}
}

// SendSubscriptionConfirmed sends subscription confirmation.
func (c *Connection) SendSubscriptionConfirmed() error {
	ack := &pb.AckResponse{
		AckType: pb.MessageType_MESSAGE_TYPE_SUBSCRIBE,
		Success: true,
		Message: "Subscription confirmed",
		TimestampMs: time.Now().UnixMilli(),
	}
	
	frame, err := protocol.MarshalMessage(protocol.MessageTypeACK, ack)
	if err != nil {
		return err
	}
	return c.WriteFrame(frame)
}

// SendPong sends a pong response.
func (c *Connection) SendPong(clientTimestamp int64, sequence uint64) error {
	pong := &pb.HeartbeatResponse{
		ClientTimestampMs: clientTimestamp,
		ServerTimestampMs: time.Now().UnixMilli(),
		Sequence:          sequence,
	}
	
	frame, err := protocol.MarshalMessage(protocol.MessageTypePong, pong)
	if err != nil {
		return err
	}
	return c.WriteFrame(frame)
}

// SendDataBatch sends a batch of tick data.
func (c *Connection) SendDataBatch(ticks []*pb.Tick) error {
	if len(ticks) == 0 {
		return nil
	}
	
	batch := &pb.DataBatch{
		Ticks:            ticks,
		BatchTimestampMs: time.Now().UnixMilli(),
		BatchSequence:    uint32(atomic.AddUint64(&c.messagesSent, 1)),
		IsSnapshot:       false,
	}
	
	// Update metrics
	atomic.AddUint64(&c.bytesSent, uint64(len(ticks)*64)) // Approximate bytes per tick
	
	return c.SendMessage(protocol.MessageTypeDataBatch, batch)
}

// SetReadDeadline sets the read deadline.
func (c *Connection) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

// SetWriteDeadline sets the write deadline.
func (c *Connection) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}

// writeLoop handles asynchronous writes to prevent blocking
func (c *Connection) writeLoop() {
	defer c.writeQueueWg.Done()
	
	for item := range c.writeQueue {
		// Check if connection is closed
		if c.closed.Load() {
			if item.done != nil {
				item.done <- fmt.Errorf("connection closed")
				close(item.done)
			}
			c.pools.PutFrame(item.frame)
			atomic.AddInt32(&c.writeQueueLen, -1)
			continue
		}
		
		// Check if deadline has passed
		if time.Now().After(item.deadline) {
			if item.done != nil {
				item.done <- fmt.Errorf("write deadline exceeded")
				close(item.done)
			}
			c.pools.PutFrame(item.frame)
			atomic.AddInt32(&c.writeQueueLen, -1)
			continue
		}
		
		// Set write deadline
		c.conn.SetWriteDeadline(item.deadline)
		
		// Write frame
		err := c.writer.WriteFrame(item.frame)
		
		// Update metrics
		if err == nil {
			atomic.AddUint64(&c.messagesSent, 1)
			atomic.AddUint64(&c.bytesSent, uint64(len(item.frame.Payload)+protocol.FrameHeaderSize+protocol.CRCSize))
		}
		
		// Signal completion
		if item.done != nil {
			item.done <- err
			close(item.done)
		}
		
		// Return frame to pool
		c.pools.PutFrame(item.frame)
		atomic.AddInt32(&c.writeQueueLen, -1)
		
		// Break on error to prevent further writes
		if err != nil {
			break
		}
	}
}

// WriteFrameAsync writes a frame asynchronously through the write queue
func (c *Connection) WriteFrameAsync(frame *protocol.Frame) error {
	if c == nil {
		return fmt.Errorf("connection is nil")
	}
	
	if c.closed.Load() {
		return fmt.Errorf("connection closed")
	}
	
	// Check queue capacity for backpressure
	queueLen := atomic.LoadInt32(&c.writeQueueLen)
	if int(queueLen) >= c.config.MaxWriteQueueSize {
		return fmt.Errorf("write queue full - slow client detected")
	}
	
	deadline := time.Now().Add(time.Duration(c.config.WriteDeadlineMS) * time.Millisecond)
	item := &WriteQueueItem{
		frame:    frame,
		deadline: deadline,
	}
	
	atomic.AddInt32(&c.writeQueueLen, 1)
	
	select {
	case c.writeQueue <- item:
		return nil
	default:
		atomic.AddInt32(&c.writeQueueLen, -1)
		return fmt.Errorf("write queue full")
	}
}

// WriteFrameSync writes a frame synchronously with deadline
func (c *Connection) WriteFrameSync(frame *protocol.Frame) error {
	if c.closed.Load() {
		return fmt.Errorf("connection closed")
	}
	
	deadline := time.Now().Add(time.Duration(c.config.WriteDeadlineMS) * time.Millisecond)
	done := make(chan error, 1)
	
	item := &WriteQueueItem{
		frame:    frame,
		deadline: deadline,
		done:     done,
	}
	
	atomic.AddInt32(&c.writeQueueLen, 1)
	
	select {
	case c.writeQueue <- item:
		return <-done
	case <-time.After(time.Duration(c.config.WriteDeadlineMS) * time.Millisecond):
		atomic.AddInt32(&c.writeQueueLen, -1)
		return fmt.Errorf("write timeout")
	}
}

// Close closes the connection.
func (c *Connection) Close() error {
	if c.closed.CompareAndSwap(false, true) {
		// Close write queue
		close(c.writeQueue)
		// Wait for write loop to finish
		c.writeQueueWg.Wait()
		return c.conn.Close()
	}
	return nil
}

// ...

// GetStats returns connection statistics.
func (c *Connection) GetStats() map[string]interface{} {
	c.mu.RLock()
	lastActivity := c.lastActivity
	c.mu.RUnlock()
	
	return map[string]interface{}{
		"id":             c.id,
		"remote_addr":    c.RemoteAddr(),
		"authenticated":  c.IsAuthenticated(),
		"messages_recv":  atomic.LoadUint64(&c.messagesRecv),
		"messages_sent":  atomic.LoadUint64(&c.messagesSent),
		"bytes_recv":     atomic.LoadUint64(&c.bytesRecv),
		"bytes_sent":     atomic.LoadUint64(&c.bytesSent),
		"last_activity":  lastActivity,
		"has_subscription": c.GetSubscription() != nil,
	}
}

// Subscription represents a client subscription.
type Subscription struct {
	Mode      pb.SubscriptionMode
	CreatedAt time.Time
}

// NewSubscription creates a new subscription.
func NewSubscription(mode pb.SubscriptionMode) *Subscription {
	return &Subscription{
		Mode:      mode,
		CreatedAt: time.Now(),
	}
}
