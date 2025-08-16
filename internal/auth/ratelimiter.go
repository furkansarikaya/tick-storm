// Package auth implements authentication mechanisms for the Tick-Storm server.
package auth

import (
	"sync"
	"time"
)

// RateLimiter implements a simple rate limiter for authentication attempts.
type RateLimiter struct {
	maxAttempts int
	window      time.Duration
	mu          sync.RWMutex
	attempts    map[string]*attemptRecord
}

// attemptRecord tracks authentication attempts for a client.
type attemptRecord struct {
	count      int
	firstTime  time.Time
	lastTime   time.Time
	blocked    bool
	blockUntil time.Time
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(maxAttempts int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		maxAttempts: maxAttempts,
		window:      window,
		attempts:    make(map[string]*attemptRecord),
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

// Allow checks if a client is allowed to attempt authentication.
func (rl *RateLimiter) Allow(clientAddr string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	record, exists := rl.attempts[clientAddr]
	if !exists {
		// First attempt
		rl.attempts[clientAddr] = &attemptRecord{
			count:     1,
			firstTime: now,
			lastTime:  now,
		}
		return true
	}

	// Check if blocked
	if record.blocked && now.Before(record.blockUntil) {
		return false
	}

	// Reset if outside window
	if now.Sub(record.firstTime) > rl.window {
		record.count = 1
		record.firstTime = now
		record.lastTime = now
		record.blocked = false
		return true
	}

	// Check attempt count
	record.count++
	record.lastTime = now

	if record.count > rl.maxAttempts {
		// Block for extended period after exceeding attempts
		record.blocked = true
		record.blockUntil = now.Add(rl.window * 2) // Double the window for blocking
		return false
	}

	return true
}

// RecordFailure records a failed authentication attempt.
func (rl *RateLimiter) RecordFailure(clientAddr string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	record, exists := rl.attempts[clientAddr]
	if !exists {
		return
	}

	// Increase penalty for failures
	if record.count >= rl.maxAttempts {
		record.blocked = true
		record.blockUntil = time.Now().Add(rl.window * 3) // Triple window for repeated failures
	}
}

// Reset resets the rate limiter for a client after successful authentication.
func (rl *RateLimiter) Reset(clientAddr string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	delete(rl.attempts, clientAddr)
}

// cleanup periodically removes old entries to prevent memory leaks.
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()

		for addr, record := range rl.attempts {
			// Remove entries older than 10 times the window
			if now.Sub(record.lastTime) > rl.window*10 {
				delete(rl.attempts, addr)
			}
		}

		rl.mu.Unlock()
	}
}

// GetStats returns current rate limiter statistics.
func (rl *RateLimiter) GetStats() map[string]interface{} {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	blockedCount := 0
	for _, record := range rl.attempts {
		if record.blocked && time.Now().Before(record.blockUntil) {
			blockedCount++
		}
	}

	return map[string]interface{}{
		"total_tracked": len(rl.attempts),
		"blocked_count": blockedCount,
		"max_attempts":  rl.maxAttempts,
		"window":        rl.window.String(),
	}
}
