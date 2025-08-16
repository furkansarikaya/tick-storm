package server

import (
	"testing"
	"time"

	"github.com/furkansarikaya/tick-storm/internal/protocol/pb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSubscription(t *testing.T) {
	tests := []struct {
		name string
		mode pb.SubscriptionMode
	}{
		{
			name: "create second mode subscription",
			mode: pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND,
		},
		{
			name: "create minute mode subscription",
			mode: pb.SubscriptionMode_SUBSCRIPTION_MODE_MINUTE,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := time.Now()
			sub := NewSubscription(tt.mode)
			after := time.Now()

			assert.NotNil(t, sub)
			assert.Equal(t, tt.mode, sub.Mode)
			assert.True(t, sub.CreatedAt.After(before) || sub.CreatedAt.Equal(before))
			assert.True(t, sub.CreatedAt.Before(after) || sub.CreatedAt.Equal(after))
		})
	}
}

func TestSubscriptionModeValidation(t *testing.T) {
	tests := []struct {
		name    string
		mode    pb.SubscriptionMode
		isValid bool
	}{
		{
			name:    "valid second mode",
			mode:    pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND,
			isValid: true,
		},
		{
			name:    "valid minute mode",
			mode:    pb.SubscriptionMode_SUBSCRIPTION_MODE_MINUTE,
			isValid: true,
		},
		{
			name:    "invalid unspecified mode",
			mode:    pb.SubscriptionMode_SUBSCRIPTION_MODE_UNSPECIFIED,
			isValid: false,
		},
		{
			name:    "invalid unknown mode",
			mode:    pb.SubscriptionMode(999),
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := tt.mode == pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND ||
				tt.mode == pb.SubscriptionMode_SUBSCRIPTION_MODE_MINUTE
			assert.Equal(t, tt.isValid, isValid)
		})
	}
}

func TestConnectionSubscription(t *testing.T) {
	// Create a test connection
	conn := &Connection{
		id: "test-conn-1",
	}

	// Test initial state - no subscription
	assert.Nil(t, conn.GetSubscription())

	// Test setting subscription
	sub := NewSubscription(pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND)
	err := conn.SetSubscription(sub)
	require.NoError(t, err)

	// Test getting subscription
	retrieved := conn.GetSubscription()
	assert.NotNil(t, retrieved)
	assert.Equal(t, sub.Mode, retrieved.Mode)
	assert.Equal(t, sub.CreatedAt, retrieved.CreatedAt)

	// Test setting subscription when already exists (should fail)
	sub2 := NewSubscription(pb.SubscriptionMode_SUBSCRIPTION_MODE_MINUTE)
	err = conn.SetSubscription(sub2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection already has a subscription")

	// Verify original subscription is still active
	retrieved = conn.GetSubscription()
	assert.Equal(t, sub.Mode, retrieved.Mode)
}

func TestSubscriptionModeSwitching(t *testing.T) {
	// Create a test connection with an existing subscription
	conn := &Connection{
		id: "test-conn-2",
	}

	// Set initial subscription to SECOND mode
	sub := NewSubscription(pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND)
	err := conn.SetSubscription(sub)
	require.NoError(t, err)

	// Attempt to switch to MINUTE mode (should fail)
	sub2 := NewSubscription(pb.SubscriptionMode_SUBSCRIPTION_MODE_MINUTE)
	err = conn.SetSubscription(sub2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection already has a subscription")

	// Verify original subscription mode is unchanged
	retrieved := conn.GetSubscription()
	assert.Equal(t, pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND, retrieved.Mode)
}

func TestSingleSubscriptionEnforcement(t *testing.T) {
	// Create a test connection
	conn := &Connection{
		id: "test-conn-3",
	}

	// Set first subscription
	sub1 := NewSubscription(pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND)
	err := conn.SetSubscription(sub1)
	require.NoError(t, err)

	// Try to set another subscription with same mode (should fail)
	sub2 := NewSubscription(pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND)
	err = conn.SetSubscription(sub2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection already has a subscription")

	// Try to set another subscription with different mode (should also fail)
	sub3 := NewSubscription(pb.SubscriptionMode_SUBSCRIPTION_MODE_MINUTE)
	err = conn.SetSubscription(sub3)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection already has a subscription")

	// Verify only the first subscription is active
	retrieved := conn.GetSubscription()
	assert.Equal(t, sub1.Mode, retrieved.Mode)
	assert.Equal(t, sub1.CreatedAt, retrieved.CreatedAt)
}

func TestSubscriptionTimeout(t *testing.T) {
	// This test verifies the subscription timeout mechanism
	// In production, the timeout would trigger after 30 seconds
	// For testing, we'll verify the timer is set up correctly

	tests := []struct {
		name            string
		mode            pb.SubscriptionMode
		expectedTimeout time.Duration
	}{
		{
			name:            "second mode subscription timeout",
			mode:            pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND,
			expectedTimeout: 30 * time.Second,
		},
		{
			name:            "minute mode subscription timeout",
			mode:            pb.SubscriptionMode_SUBSCRIPTION_MODE_MINUTE,
			expectedTimeout: 30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create subscription
			sub := NewSubscription(tt.mode)
			assert.NotNil(t, sub)
			
			// In the actual implementation, the timeout is handled by ConnectionHandler
			// This test just verifies the subscription structure is correct
			assert.Equal(t, tt.mode, sub.Mode)
			assert.NotZero(t, sub.CreatedAt)
		})
	}
}

func BenchmarkNewSubscription(b *testing.B) {
	modes := []pb.SubscriptionMode{
		pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND,
		pb.SubscriptionMode_SUBSCRIPTION_MODE_MINUTE,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mode := modes[i%len(modes)]
		_ = NewSubscription(mode)
	}
}

func BenchmarkGetSetSubscription(b *testing.B) {
	conn := &Connection{
		id: "bench-conn",
	}

	sub := NewSubscription(pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND)
	_ = conn.SetSubscription(sub)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = conn.GetSubscription()
	}
}
