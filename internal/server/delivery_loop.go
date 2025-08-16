package server

import (
	"context"
	"fmt"
	"time"

	"github.com/furkansarikaya/tick-storm/internal/protocol/pb"
)

// deliveryLoop handles data delivery with micro-batching.
func (h *ConnectionHandler) deliveryLoop(ctx context.Context, errChan chan<- error) {
	// Configurable batching parameters
	batchWindow := h.config.BatchWindow
	if batchWindow == 0 {
		batchWindow = 5 * time.Millisecond // Default 5ms window
	}
	
	maxBatchSize := h.config.MaxBatchSize
	if maxBatchSize == 0 {
		maxBatchSize = 100 // Default max batch size
	}
	
	// Backpressure tracking
	var consecutiveDrops int
	const maxConsecutiveDrops = 10
	
	h.logger.Info("starting delivery loop",
		"batch_window", batchWindow,
		"max_batch_size", maxBatchSize,
	)
	
	for {
		select {
		case <-ctx.Done():
			h.logger.Info("delivery loop stopped")
			return
			
		case ticks := <-h.dataChan:
			// Filter ticks based on subscription mode if needed
			filteredTicks := h.filterTicksBySubscription(ticks)
			if len(filteredTicks) == 0 {
				continue
			}
			
			// Add ticks to pending batch
			h.pendingBatch = append(h.pendingBatch, filteredTicks...)
			
			// Reset consecutive drops on successful data reception
			consecutiveDrops = 0
			
			// Reset batch timer
			if h.batchTimer != nil {
				h.batchTimer.Stop()
			}
			h.batchTimer = time.AfterFunc(batchWindow, func() {
				h.flushBatch(errChan)
			})
			
			// Check if batch is full
			if len(h.pendingBatch) >= maxBatchSize {
				h.batchTimer.Stop()
				h.flushBatch(errChan)
			}
			
		case <-h.batchTimer.C:
			// Timer expired, flush batch
			h.flushBatch(errChan)
			
		default:
			// Check for backpressure - if data channel is full
			select {
			case ticks := <-h.dataChan:
				// Process normally
				filteredTicks := h.filterTicksBySubscription(ticks)
				if len(filteredTicks) > 0 {
					h.pendingBatch = append(h.pendingBatch, filteredTicks...)
				}
			default:
				// Data channel is empty, check for backpressure
				if len(h.dataChan) >= cap(h.dataChan)*3/4 {
					consecutiveDrops++
					h.logger.Warn("backpressure detected",
						"channel_usage", len(h.dataChan),
						"channel_capacity", cap(h.dataChan),
						"consecutive_drops", consecutiveDrops,
					)
					
					// If too many consecutive drops, consider connection slow
					if consecutiveDrops >= maxConsecutiveDrops {
						h.logger.Error("connection too slow, considering disconnect",
							"consecutive_drops", consecutiveDrops,
						)
						select {
						case errChan <- fmt.Errorf("connection backpressure exceeded threshold"):
						default:
						}
						return
					}
				}
				time.Sleep(time.Millisecond) // Brief pause to avoid busy waiting
			}
		}
	}
}

// flushBatch sends the pending batch to the client.
func (h *ConnectionHandler) flushBatch(errChan chan<- error) {
	if len(h.pendingBatch) == 0 {
		return
	}
	
	// Send batch
	if err := h.conn.SendDataBatch(h.pendingBatch); err != nil {
		select {
		case errChan <- err:
		default:
		}
		return
	}
	
	// Clear pending batch
	h.pendingBatch = h.pendingBatch[:0]
}

// filterTicksBySubscription filters ticks based on the connection's subscription mode.
func (h *ConnectionHandler) filterTicksBySubscription(ticks []*pb.Tick) []*pb.Tick {
	subscription := h.conn.GetSubscription()
	if subscription == nil {
		// No subscription, drop all ticks
		return nil
	}
	
	// Filter ticks that match the subscription mode
	filtered := make([]*pb.Tick, 0, len(ticks))
	for _, tick := range ticks {
		if tick.Mode == subscription.Mode {
			filtered = append(filtered, tick)
		}
	}
	
	return filtered
}
