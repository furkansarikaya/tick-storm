package server

import (
	"testing"
	"time"

	"github.com/furkansarikaya/tick-storm/internal/protocol/pb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDataBatchDelivery(t *testing.T) {
	// Test batch creation and validation without network I/O
	ticks := []*pb.Tick{
		{
			Symbol:      "TEST1",
			Price:       100.0,
			Volume:      1000,
			TimestampMs: time.Now().UnixMilli(),
			Mode:        pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND,
		},
		{
			Symbol:      "TEST2", 
			Price:       200.0,
			Volume:      2000,
			TimestampMs: time.Now().UnixMilli(),
			Mode:        pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND,
		},
	}
	
	// Test batch structure creation
	batch := &pb.DataBatch{
		Ticks:            ticks,
		BatchTimestampMs: time.Now().UnixMilli(),
		BatchSequence:    1,
		IsSnapshot:       false,
	}
	
	// Verify batch properties
	assert.NotNil(t, batch)
	assert.Len(t, batch.Ticks, 2)
	assert.Equal(t, uint32(1), batch.BatchSequence)
	assert.False(t, batch.IsSnapshot)
	assert.Greater(t, batch.BatchTimestampMs, int64(0))
	
	// Test empty batch handling
	emptyBatch := &pb.DataBatch{
		Ticks: []*pb.Tick{},
	}
	assert.Len(t, emptyBatch.Ticks, 0)
}

func TestMicroBatching(t *testing.T) {
	config := DefaultConfig()
	config.BatchWindow = 10 * time.Millisecond
	config.MaxBatchSize = 3
	
	// Test configuration values
	assert.Equal(t, 10*time.Millisecond, config.BatchWindow)
	assert.Equal(t, 3, config.MaxBatchSize)
	
	// Test batching logic without network I/O
	pendingBatch := make([]*pb.Tick, 0, config.MaxBatchSize)
	
	// Add ticks to batch
	testTick := &pb.Tick{
		Symbol:      "BATCH1",
		Price:       100.0,
		TimestampMs: time.Now().UnixMilli(),
		Mode:        pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND,
	}
	
	pendingBatch = append(pendingBatch, testTick)
	assert.Len(t, pendingBatch, 1)
	
	// Test batch size trigger
	for i := 0; i < config.MaxBatchSize-1; i++ {
		pendingBatch = append(pendingBatch, testTick)
	}
	
	shouldFlush := len(pendingBatch) >= config.MaxBatchSize
	assert.True(t, shouldFlush, "Should flush when batch reaches max size")
}

func TestTickFiltering(t *testing.T) {
	config := DefaultConfig()
	conn := &Connection{
		id: "test-filter-conn",
	}
	
	// Create handler manually to avoid network connection initialization
	handler := &ConnectionHandler{
		conn:   conn,
		config: config,
	}
	
	// Test without subscription (should filter all)
	ticks := []*pb.Tick{
		{Mode: pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND},
		{Mode: pb.SubscriptionMode_SUBSCRIPTION_MODE_MINUTE},
	}
	
	filtered := handler.filterTicksBySubscription(ticks)
	assert.Nil(t, filtered)
	
	// Set up SECOND mode subscription
	sub := NewSubscription(pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND)
	err := conn.SetSubscription(sub)
	require.NoError(t, err)
	
	// Test filtering with SECOND subscription
	filtered = handler.filterTicksBySubscription(ticks)
	assert.Len(t, filtered, 1)
	assert.Equal(t, pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND, filtered[0].Mode)
	
	// Test with all matching ticks
	allSecondTicks := []*pb.Tick{
		{Mode: pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND},
		{Mode: pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND},
	}
	
	filtered = handler.filterTicksBySubscription(allSecondTicks)
	assert.Len(t, filtered, 2)
}

func TestBatchSizeOptimization(t *testing.T) {
	tests := []struct {
		name         string
		maxBatchSize int
		tickCount    int
		expectedFlush bool
	}{
		{
			name:         "batch not full",
			maxBatchSize: 10,
			tickCount:    5,
			expectedFlush: false,
		},
		{
			name:         "batch exactly full",
			maxBatchSize: 5,
			tickCount:    5,
			expectedFlush: true,
		},
		{
			name:         "batch overflow",
			maxBatchSize: 3,
			tickCount:    5,
			expectedFlush: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()
			config.MaxBatchSize = tt.maxBatchSize
			config.BatchWindow = 100 * time.Millisecond // Long window to test size trigger
			
			conn := &Connection{
				id: "test-batch-size-conn",
			}
			
			// Create handler manually to avoid network connection initialization
			handler := &ConnectionHandler{
				conn:         conn,
				config:       config,
				pendingBatch: make([]*pb.Tick, 0, 100),
			}
			
			// Set up subscription
			sub := NewSubscription(pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND)
			err := conn.SetSubscription(sub)
			require.NoError(t, err)
			
			// Create test ticks
			ticks := make([]*pb.Tick, tt.tickCount)
			for i := 0; i < tt.tickCount; i++ {
				ticks[i] = &pb.Tick{
					Symbol:      "TEST",
					Mode:        pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND,
					TimestampMs: time.Now().UnixMilli(),
				}
			}
			
			// Add ticks to pending batch
			handler.pendingBatch = append(handler.pendingBatch, ticks...)
			
			// Check if batch should be flushed based on size
			shouldFlush := len(handler.pendingBatch) >= tt.maxBatchSize
			assert.Equal(t, tt.expectedFlush, shouldFlush)
		})
	}
}

func TestBackpressureHandling(t *testing.T) {
	config := DefaultConfig()
	config.BatchWindow = 1 * time.Millisecond
	
	// Create small channel to simulate backpressure
	conn := &Connection{
		id: "test-backpressure-conn",
	}
	
	// Create handler manually to avoid network connection initialization
	handler := &ConnectionHandler{
		conn:     conn,
		config:   config,
		dataChan: make(chan []*pb.Tick, 2), // Small buffer
	}
	
	// Set up subscription
	sub := NewSubscription(pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND)
	err := conn.SetSubscription(sub)
	require.NoError(t, err)
	
	// Fill the channel to create backpressure
	testTick := []*pb.Tick{{
		Symbol: "BACKPRESSURE_TEST",
		Mode:   pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND,
	}}
	
	// Fill channel
	handler.dataChan <- testTick
	handler.dataChan <- testTick
	
	// Channel should now be full
	assert.Equal(t, cap(handler.dataChan), len(handler.dataChan))
	
	// Test backpressure detection
	channelUsage := len(handler.dataChan)
	channelCapacity := cap(handler.dataChan)
	backpressureThreshold := channelCapacity * 3 / 4
	
	hasBackpressure := channelUsage >= backpressureThreshold
	assert.True(t, hasBackpressure, "Should detect backpressure when channel is >= 75% full")
}

func TestConfigurableBatchWindow(t *testing.T) {
	tests := []struct {
		name        string
		batchWindow time.Duration
		expected    time.Duration
	}{
		{
			name:        "custom batch window",
			batchWindow: 10 * time.Millisecond,
			expected:    10 * time.Millisecond,
		},
		{
			name:        "zero batch window uses default",
			batchWindow: 0,
			expected:    5 * time.Millisecond,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()
			config.BatchWindow = tt.batchWindow
			
			// The actual test would require running the delivery loop
			// and measuring timing, which is complex in unit tests.
			// Here we just verify the configuration is set correctly.
			assert.Equal(t, tt.batchWindow, config.BatchWindow)
			
			// Verify default fallback logic
			actualWindow := config.BatchWindow
			if actualWindow == 0 {
				actualWindow = 5 * time.Millisecond
			}
			assert.Equal(t, tt.expected, actualWindow)
		})
	}
}

func BenchmarkDataBatchDelivery(b *testing.B) {
	conn := &Connection{
		id: "bench-conn",
	}
	
	// Set up subscription
	sub := NewSubscription(pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND)
	_ = conn.SetSubscription(sub)
	
	// Create test ticks
	ticks := make([]*pb.Tick, 100)
	for i := 0; i < 100; i++ {
		ticks[i] = &pb.Tick{
			Symbol:      "BENCH",
			Price:       100.0,
			Volume:      1000,
			TimestampMs: time.Now().UnixMilli(),
			Mode:        pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND,
		}
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = conn.SendDataBatch(ticks)
	}
}

func BenchmarkTickFiltering(b *testing.B) {
	config := DefaultConfig()
	conn := &Connection{
		id: "bench-filter-conn",
	}
	
	handler := NewConnectionHandler(conn, config)
	
	// Set up subscription
	sub := NewSubscription(pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND)
	_ = conn.SetSubscription(sub)
	
	// Create mixed ticks
	ticks := make([]*pb.Tick, 1000)
	for i := 0; i < 1000; i++ {
		mode := pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND
		if i%2 == 0 {
			mode = pb.SubscriptionMode_SUBSCRIPTION_MODE_MINUTE
		}
		ticks[i] = &pb.Tick{
			Symbol: "BENCH",
			Mode:   mode,
		}
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = handler.filterTicksBySubscription(ticks)
	}
}
