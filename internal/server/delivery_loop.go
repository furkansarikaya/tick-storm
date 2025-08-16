package server

import (
	"context"
	"time"
)

// deliveryLoop handles data delivery with micro-batching.
func (h *ConnectionHandler) deliveryLoop(ctx context.Context, errChan chan<- error) {
	batchWindow := 5 * time.Millisecond
	maxBatchSize := 100
	
	for {
		select {
		case <-ctx.Done():
			return
			
		case ticks := <-h.dataChan:
			// Add ticks to pending batch
			h.pendingBatch = append(h.pendingBatch, ticks...)
			
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
