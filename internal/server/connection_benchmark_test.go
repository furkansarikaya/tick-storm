// Package server provides benchmarking tests for connection handling performance.
package server

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const (
	benchmarkServerStartError = "Failed to start server: %v"
	benchmarkConnectError     = "Failed to connect: %v"
)

// BenchmarkConnectionHandling benchmarks the goroutine-per-connection model
func BenchmarkConnectionHandling(b *testing.B) {
	config := DefaultConfig()
	config.MaxConnections = 100000
	config.ListenAddr = ":0" // Use random port
	
	server := NewServer(config)
	
	// Start server
	errorCh := make(chan error, 1)
	go func() {
		if err := server.Start(); err != nil {
			errorCh <- fmt.Errorf(benchmarkServerStartError, err)
			return
		}
	}()
	
	// Wait for server to start
	time.Sleep(100 * time.Millisecond)
	
	// Check for startup errors
	select {
	case err := <-errorCh:
		b.Fatal(err)
	default:
	}
	
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Stop(ctx)
	}()
	
	// Get actual listening address
	addr := server.listener.Addr().String()
	
	b.ResetTimer()
	
	b.Run("Sequential", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			conn, err := net.Dial("tcp", server.listener.Addr().String())
			if err != nil {
				b.Errorf(benchmarkConnectError, err)
				return
			}
			conn.Close()
		}
	})
	
	b.Run("Concurrent-100", func(b *testing.B) {
		benchmarkConcurrentConnections(b, addr, 100)
	})
	
	b.Run("Concurrent-1000", func(b *testing.B) {
		benchmarkConcurrentConnections(b, addr, 1000)
	})
	
	b.Run("Concurrent-10000", func(b *testing.B) {
		benchmarkConcurrentConnections(b, addr, 10000)
	})
}

func benchmarkConcurrentConnections(b *testing.B, addr string, concurrency int) {
	var wg sync.WaitGroup
	var successful int64
	var failed int64
	
	semaphore := make(chan struct{}, concurrency)
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		wg.Add(1)
		semaphore <- struct{}{}
		
		go func() {
			defer wg.Done()
			defer func() { <-semaphore }()
			
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				atomic.AddInt64(&failed, 1)
				return
			}
			
			atomic.AddInt64(&successful, 1)
			conn.Close()
		}()
	}
	
	wg.Wait()
	
}

// BenchmarkMemoryOverhead measures memory overhead per connection
func BenchmarkMemoryOverhead(b *testing.B) {
	config := DefaultConfig()
	config.MaxConnections = 10000
	config.ListenAddr = ":0"
	
	server := NewServer(config)
	
	// Start server
	errorCh := make(chan error, 1)
	go func() {
		if err := server.Start(); err != nil {
			errorCh <- fmt.Errorf(benchmarkServerStartError, err)
		}
	}()
	
	time.Sleep(100 * time.Millisecond)
	addr := server.listener.Addr().String()
	
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Stop(ctx)
	}()
	
	// Measure baseline memory
	runtime.GC()
	var baseline runtime.MemStats
	runtime.ReadMemStats(&baseline)
	
	connections := make([]net.Conn, 0, b.N)
	
	b.ResetTimer()
	
	// Create connections
	for i := 0; i < b.N; i++ {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			b.Fatalf("Failed to connect: %v", err)
		}
		connections = append(connections, conn)
		
		// Measure memory every 1000 connections
		if i%1000 == 0 && i > 0 {
			runtime.GC()
			var current runtime.MemStats
			runtime.ReadMemStats(&current)
			
			memoryPerConn := (current.Alloc - baseline.Alloc) / uint64(i)
			b.Logf("Connections: %d, Memory per connection: %d bytes", i, memoryPerConn)
		}
	}
	
	// Final memory measurement
	runtime.GC()
	var final runtime.MemStats
	runtime.ReadMemStats(&final)
	
	totalMemory := final.Alloc - baseline.Alloc
	memoryPerConn := totalMemory / uint64(b.N)
	
	b.Logf("Total connections: %d", b.N)
	b.Logf("Total memory overhead: %d bytes", totalMemory)
	b.Logf("Memory per connection: %d bytes", memoryPerConn)
	
	// Close all connections
	for _, conn := range connections {
		conn.Close()
	}
}

// BenchmarkGoroutineOverhead measures goroutine overhead
func BenchmarkGoroutineOverhead(b *testing.B) {
	// Measure baseline goroutines
	baseline := runtime.NumGoroutine()
	
	config := DefaultConfig()
	config.MaxConnections = 100000
	config.ListenAddr = ":0"
	
	server := NewServer(config)
	
	go func() {
		if err := server.Start(); err != nil {
			b.Fatalf("Failed to start server: %v", err)
		}
	}()
	
	time.Sleep(100 * time.Millisecond)
	addr := server.listener.Addr().String()
	
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Stop(ctx)
	}()
	
	connections := make([]net.Conn, 0, b.N)
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			b.Fatalf("Failed to connect: %v", err)
		}
		connections = append(connections, conn)
		
		// Measure goroutines every 1000 connections
		if i%1000 == 0 && i > 0 {
			current := runtime.NumGoroutine()
			goroutinesPerConn := float64(current-baseline) / float64(i)
			b.Logf("Connections: %d, Goroutines per connection: %.2f", i, goroutinesPerConn)
		}
	}
	
	final := runtime.NumGoroutine()
	totalGoroutines := final - baseline
	goroutinesPerConn := float64(totalGoroutines) / float64(b.N)
	
	b.Logf("Total connections: %d", b.N)
	b.Logf("Total goroutines: %d", totalGoroutines)
	b.Logf("Goroutines per connection: %.2f", goroutinesPerConn)
	
	// Close all connections
	for _, conn := range connections {
		conn.Close()
	}
}

// BenchmarkConnectionLatency measures connection establishment latency
func BenchmarkConnectionLatency(b *testing.B) {
	config := DefaultConfig()
	config.ListenAddr = ":0"
	
	server := NewServer(config)
	
	go func() {
		if err := server.Start(); err != nil {
			b.Fatalf("Failed to start server: %v", err)
		}
	}()
	
	time.Sleep(100 * time.Millisecond)
	addr := server.listener.Addr().String()
	
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Stop(ctx)
	}()
	
	latencies := make([]time.Duration, b.N)
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		start := time.Now()
		conn, err := net.Dial("tcp", addr)
		latency := time.Since(start)
		
		if err != nil {
			b.Fatalf("Failed to connect: %v", err)
		}
		
		latencies[i] = latency
		conn.Close()
	}
	
	// Calculate statistics
	var total time.Duration
	min := latencies[0]
	max := latencies[0]
	
	for _, lat := range latencies {
		total += lat
		if lat < min {
			min = lat
		}
		if lat > max {
			max = lat
		}
	}
	
	avg := total / time.Duration(b.N)
	
	b.Logf("Connection latency - Min: %v, Max: %v, Avg: %v", min, max, avg)
}

// BenchmarkThroughput measures message throughput
func BenchmarkThroughput(b *testing.B) {
	config := DefaultConfig()
	config.ListenAddr = ":0"
	
	server := NewServer(config)
	
	go func() {
		if err := server.Start(); err != nil {
			b.Fatalf("Failed to start server: %v", err)
		}
	}()
	
	time.Sleep(100 * time.Millisecond)
	addr := server.listener.Addr().String()
	
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Stop(ctx)
	}()
	
	// Create test message
	message := make([]byte, 1024) // 1KB message
	for i := range message {
		message[i] = byte(i % 256)
	}
	
	b.ResetTimer()
	
	b.Run("SingleConnection", func(b *testing.B) {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			b.Fatalf("Failed to connect: %v", err)
		}
		defer conn.Close()
		
		b.SetBytes(int64(len(message)))
		
		for i := 0; i < b.N; i++ {
			_, err := conn.Write(message)
			if err != nil {
				b.Fatalf("Failed to write: %v", err)
			}
		}
	})
	
	b.Run("MultipleConnections", func(b *testing.B) {
		concurrency := 100
		messagesPerConn := b.N / concurrency
		
		var wg sync.WaitGroup
		
		b.SetBytes(int64(len(message)))
		
		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				
				conn, err := net.Dial("tcp", addr)
				if err != nil {
					b.Errorf("Failed to connect: %v", err)
					return
				}
				defer conn.Close()
				
				for j := 0; j < messagesPerConn; j++ {
					_, err := conn.Write(message)
					if err != nil {
						b.Errorf("Failed to write: %v", err)
						return
					}
				}
			}()
		}
		
		wg.Wait()
	})
}

// TestConnectionScaling tests connection scaling capabilities
func TestConnectionScaling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping scaling test in short mode")
	}
	
	config := DefaultConfig()
	config.MaxConnections = 50000 // Reduced for testing
	config.ListenAddr = ":0"
	
	server := NewServer(config)
	
	go func() {
		if err := server.Start(); err != nil {
			t.Errorf("Failed to start server: %v", err)
		}
	}()
	
	time.Sleep(100 * time.Millisecond)
	addr := server.listener.Addr().String()
	
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		server.Stop(ctx)
	}()
	
	// Test scaling up to different connection counts
	testCases := []int{100, 1000, 5000, 10000}
	
	for _, connCount := range testCases {
		t.Run(fmt.Sprintf("Connections-%d", connCount), func(t *testing.T) {
			connections := make([]net.Conn, 0, connCount)
			
			// Create connections
			start := time.Now()
			for i := 0; i < connCount; i++ {
				conn, err := net.Dial("tcp", addr)
				if err != nil {
					t.Fatalf("Failed to connect at %d: %v", i, err)
				}
				connections = append(connections, conn)
				
				// Brief pause to avoid overwhelming
				if i%100 == 0 {
					time.Sleep(time.Millisecond)
				}
			}
			
			establishTime := time.Since(start)
			
			// Verify all connections are active
			activeConns := atomic.LoadInt32(&server.activeConns)
			if activeConns != int32(connCount) {
				t.Errorf("Expected %d active connections, got %d", connCount, activeConns)
			}
			
			t.Logf("Established %d connections in %v", connCount, establishTime)
			t.Logf("Average time per connection: %v", establishTime/time.Duration(connCount))
			
			// Close all connections
			for _, conn := range connections {
				conn.Close()
			}
			
			// Wait for cleanup
			time.Sleep(100 * time.Millisecond)
		})
	}
}
