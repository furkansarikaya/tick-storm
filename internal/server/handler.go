// Package server implements the TCP server for Tick-Storm.
package server

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/furkansarikaya/tick-storm/internal/protocol"
	"github.com/furkansarikaya/tick-storm/internal/protocol/pb"
	"google.golang.org/protobuf/proto"
)

// ConnectionHandler handles the connection lifecycle
type ConnectionHandler struct {
	conn           *Connection
	config         *Config
	subscription   *Subscription
	lastHeartbeat  time.Time
	heartbeatTimer *time.Timer
	ctx            context.Context
	cancel         context.CancelFunc
	authenticated  bool
	pendingBatch   []*pb.Tick
	dataChan       chan []*pb.Tick
	batchTimer     *time.Timer
}

// NewConnectionHandler creates a new connection handler.
func NewConnectionHandler(conn *Connection, config *Config) *ConnectionHandler {
	return &ConnectionHandler{
		conn:           conn,
		config:         config,
		heartbeatTimer: time.NewTimer(config.HeartbeatTimeout),
		dataChan:       make(chan []*pb.Tick, 100),
		batchTimer:     time.NewTimer(5 * time.Millisecond),
		pendingBatch:   make([]*pb.Tick, 0, 100),
	}
}

// Handle handles the connection after authentication.
func (h *ConnectionHandler) Handle(ctx context.Context) error {
	// Start heartbeat monitoring
	h.heartbeatTimer = time.NewTimer(h.config.HeartbeatTimeout)
	defer h.heartbeatTimer.Stop()
	
	// Start batch timer
	h.batchTimer = time.NewTimer(5 * time.Millisecond) // Default batch window
	defer h.batchTimer.Stop()
	
	// Create error channel for goroutines
	errChan := make(chan error, 2)
	
	// Start data delivery goroutine
	go h.deliveryLoop(ctx, errChan)
	
	// Main message processing loop
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
			
		case <-h.heartbeatTimer.C:
			// Heartbeat timeout
			h.conn.SendError(pb.ErrorCode_ERROR_CODE_HEARTBEAT_TIMEOUT, "heartbeat timeout")
			return fmt.Errorf("heartbeat timeout")
			
		case err := <-errChan:
			return err
			
		default:
			// Set read deadline for next message
			h.conn.SetReadDeadline(time.Now().Add(h.config.ReadTimeout))
			
			// Read next frame
			frame, err := h.conn.ReadFrame()
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return nil
				}
				if sendErr := h.conn.SendError(pb.ErrorCode_ERROR_CODE_INVALID_MESSAGE, err.Error()); sendErr != nil {
					return sendErr
				}
				return err
			}
			
			// First frame must be auth
			if !h.authenticated && frame.Type != protocol.MessageTypeAuth {
				if sendErr := h.conn.SendError(pb.ErrorCode_ERROR_CODE_INVALID_MESSAGE, "first frame must be auth"); sendErr != nil {
					return sendErr
				}
				return fmt.Errorf("first frame must be auth")
			}
			
			// Process the frame
			if err := h.processFrame(ctx, frame); err != nil {
				if sendErr := h.conn.SendError(pb.ErrorCode_ERROR_CODE_INVALID_MESSAGE, err.Error()); sendErr != nil {
					return sendErr
				}
				return err
			}
		}
	}
}

// processFrame processes an incoming frame.
func (h *ConnectionHandler) processFrame(ctx context.Context, frame *protocol.Frame) error {
	switch frame.Type {
	case protocol.MessageTypeHeartbeat:
		return h.handleHeartbeat(frame)
		
	case protocol.MessageTypeSubscribe:
		return h.handleSubscribe(frame)
		
	case protocol.MessageTypeAuth:
		// AUTH is only allowed as first frame
		return protocol.ErrInvalidSequence
		
	default:
		return protocol.ErrInvalidMessageType
	}
}

// handleHeartbeat handles a heartbeat message.
func (h *ConnectionHandler) handleHeartbeat(frame *protocol.Frame) error {
	var hb pb.HeartbeatRequest
	if err := proto.Unmarshal(frame.Payload, &hb); err != nil {
		return fmt.Errorf("failed to unmarshal heartbeat: %w", err)
	}
	
	// Log heartbeat received (logger will be added later)
	
	// Update last heartbeat time
	h.lastHeartbeat = time.Now()
	
	// Reset heartbeat timer
	if h.heartbeatTimer != nil {
		h.heartbeatTimer.Reset(h.config.HeartbeatTimeout)
	}
	
	// Send pong response
	return h.conn.SendPong(hb.TimestampMs, hb.Sequence)
}

// handleSubscribe handles a subscription request.
func (h *ConnectionHandler) handleSubscribe(frame *protocol.Frame) error {
	var sub pb.SubscribeRequest
	if err := proto.Unmarshal(frame.Payload, &sub); err != nil {
		return fmt.Errorf("failed to unmarshal subscribe: %w", err)
	}
	
	// Validate subscription mode
	if sub.Mode != pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND && sub.Mode != pb.SubscriptionMode_SUBSCRIPTION_MODE_MINUTE {
		return protocol.ErrInvalidSubscription
	}
	
	// Check if already subscribed
	if h.conn.GetSubscription() != nil {
		return protocol.ErrAlreadySubscribed
	}
	
	// Create subscription
	subscription := NewSubscription(sub.Mode)
	if err := h.conn.SetSubscription(subscription); err != nil {
		return err
	}
	
	// Send subscription confirmation
	if err := h.conn.SendSubscriptionConfirmed(); err != nil {
		return err
	}
	
	// Start data generation based on subscription mode
	go h.startDataGeneration(subscription)
	
	return nil
}

// startDataGeneration starts generating tick data based on subscription.
func (h *ConnectionHandler) startDataGeneration(subscription *Subscription) {
	var ticker *time.Ticker
	
	switch subscription.Mode {
	case pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND:
		ticker = time.NewTicker(1 * time.Second)
	case pb.SubscriptionMode_SUBSCRIPTION_MODE_MINUTE:
		ticker = time.NewTicker(1 * time.Minute)
	default:
		return
	}
	
	defer ticker.Stop()
	
	var i int
	for {
		select {
		case <-ticker.C:
			// Generate tick data (placeholder - in production, get real data)
			tick := &pb.Tick{
				Symbol:      fmt.Sprintf("TICK_%d", i),
				Price:       100.0 + rand.Float64()*10,
				Volume:      float64(rand.Intn(1000)),
				TimestampMs: time.Now().UnixMilli(),
			}
			
			// Send to data channel for batching
			select {
			case h.dataChan <- []*pb.Tick{tick}:
			default:
				// Channel full, drop tick (or handle backpressure)
			}
			
		case <-time.After(time.Second):
			// Connection closed
			return
		}
	}
}

// deliveryLoop handles data delivery with micro-batching.
func (h *ConnectionHandler) deliveryLoop(ctx context.Context, errChan chan<- error) {
	batchWindow := 5 * time.Millisecond
	maxBatchSize := 100
	
	for {
		select {
		case <-ctx.Done():
			return
			
		case ticks := <-h.dataChan:
			// Add to pending batch
			h.pendingBatch = append(h.pendingBatch, ticks...)
			
			// Check if batch is full
			if len(h.pendingBatch) >= maxBatchSize {
				if err := h.flushBatch(); err != nil {
					errChan <- err
					return
				}
			} else {
				// Reset batch timer
				h.batchTimer.Reset(batchWindow)
			}
			
		case <-h.batchTimer.C:
			// Flush pending batch
			if len(h.pendingBatch) > 0 {
				if err := h.flushBatch(); err != nil {
					errChan <- err
					return
				}
			}
			h.batchTimer.Reset(batchWindow)
		}
	}
}

// flushBatch sends the pending batch.
func (h *ConnectionHandler) flushBatch() error {
	if len(h.pendingBatch) == 0 {
		return nil
	}
	
	// Send batch
	err := h.conn.SendDataBatch(h.pendingBatch)
	
	// Clear batch
	h.pendingBatch = h.pendingBatch[:0]
	
	return err
}
