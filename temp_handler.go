// Package server implements the TCP server for Tick-Storm.
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	logger         *slog.Logger
	subscriptionTimer *time.Timer  // Timer for subscription timeout
}

// NewConnectionHandler creates a new connection handler.
func NewConnectionHandler(conn *Connection, config *Config) *ConnectionHandler {
	logger := slog.Default().With(
		"connection_id", conn.ID(),
		"remote_addr", conn.RemoteAddr(),
	)
	
	return &ConnectionHandler{
		conn:           conn,
		config:         config,
		heartbeatTimer: time.NewTimer(config.HeartbeatTimeout),
		dataChan:       make(chan []*pb.Tick, 100),
		batchTimer:     time.NewTimer(5 * time.Millisecond),
		pendingBatch:   make([]*pb.Tick, 0, 100),
		logger:         logger,
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
	
	// Log heartbeat received
	h.logger.Debug("heartbeat received",
		"timestamp_ms", hb.TimestampMs,
		"sequence", hb.Sequence,
	)
	
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
		h.logger.Error("failed to unmarshal subscribe request",
			"error", err,
		)
		return fmt.Errorf("failed to unmarshal subscribe: %w", err)
	}
	
	// Log subscription attempt
	h.logger.Info("subscription request received",
		"mode", sub.Mode.String(),
		"symbols", sub.Symbols,
		"start_time_ms", sub.StartTimeMs,
	)
	
	// Validate subscription mode
	if sub.Mode != pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND && sub.Mode != pb.SubscriptionMode_SUBSCRIPTION_MODE_MINUTE {
		h.logger.Warn("invalid subscription mode",
			"mode", sub.Mode.String(),
		)
		return protocol.ErrInvalidSubscription
	}
	
	// Check if already subscribed
	existingSub := h.conn.GetSubscription()
	if existingSub != nil {
		// Check if trying to switch modes
		if existingSub.Mode != sub.Mode {
			h.logger.Warn("subscription mode switching attempted",
				"current_mode", existingSub.Mode.String(),
				"requested_mode", sub.Mode.String(),
			)
			return fmt.Errorf("subscription mode switching not allowed: already subscribed to %s mode", existingSub.Mode.String())
		}
		h.logger.Warn("duplicate subscription attempt",
			"existing_mode", existingSub.Mode.String(),
		)
		return protocol.ErrAlreadySubscribed
	}
	
	// Create subscription
	subscription := NewSubscription(sub.Mode)
	if err := h.conn.SetSubscription(subscription); err != nil {
		h.logger.Error("failed to set subscription",
			"error", err,
		)
		return err
	}
	
	// Set up subscription timeout (30 seconds to receive first data)
	if h.subscriptionTimer != nil {
		h.subscriptionTimer.Stop()
	}
	h.subscriptionTimer = time.AfterFunc(30*time.Second, func() {
		h.logger.Warn("subscription timeout - no data generated within 30 seconds")
		// Could implement additional handling here if needed
	})
	
	// Send subscription confirmation
	if err := h.conn.SendSubscriptionConfirmed(); err != nil {
		h.logger.Error("failed to send subscription confirmation",
			"error", err,
		)
		return err
	}
	
	// Log successful subscription
	h.logger.Info("subscription confirmed",
		"mode", sub.Mode.String(),
		"created_at", subscription.CreatedAt,
	)
	
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
		h.logger.Info("starting tick generation", "mode", "SECOND", "interval", "1s")
	case pb.SubscriptionMode_SUBSCRIPTION_MODE_MINUTE:
		ticker = time.NewTicker(1 * time.Minute)
		h.logger.Info("starting tick generation", "mode", "MINUTE", "interval", "1m")
	default:
		h.logger.Error("invalid subscription mode for data generation", "mode", subscription.Mode.String())
		return
	}
	
	defer ticker.Stop()
	defer func() {
		if h.subscriptionTimer != nil {
			h.subscriptionTimer.Stop()
		}
		h.logger.Info("stopping tick generation", "mode", subscription.Mode.String())
	}()
	
	var i int
	for {
		select {
		case <-ticker.C:
			// Reset subscription timeout on successful data generation
			if h.subscriptionTimer != nil {
				h.subscriptionTimer.Stop()
			}
			
			// Generate tick data (placeholder - in production, get real data)
			tick := &pb.Tick{
				Symbol:      fmt.Sprintf("TICK_%d", i),
				Price:       100.0 + rand.Float64()*10,
				Volume:      float64(rand.Intn(1000)),
				TimestampMs: time.Now().UnixMilli(),
				Mode:        subscription.Mode,
			}
			
			// Send to data channel for batching
			select {
			case h.dataChan <- []*pb.Tick{tick}:
				h.logger.Debug("tick generated",
					"symbol", tick.Symbol,
					"price", tick.Price,
					"mode", subscription.Mode.String(),
				)
				i++
			default:
				// Channel full, drop tick (or handle backpressure)
				h.logger.Warn("data channel full, dropping tick",
					"symbol", tick.Symbol,
				)
			
		case <-time.After(time.Second):
			// Connection closed
			return
		}
	}
}

