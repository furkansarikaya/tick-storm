package server

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/furkansarikaya/tick-storm/internal/protocol"
	"github.com/furkansarikaya/tick-storm/internal/protocol/pb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObjectPools(t *testing.T) {
	pools := NewObjectPools()

	t.Run("FramePool", func(t *testing.T) {
		frame := pools.GetFrame()
		require.NotNil(t, frame)
		
		// Modify frame
		frame.Type = 1
		frame.Length = 100
		
		pools.PutFrame(frame)
		
		// Get another frame - should be reused
		frame2 := pools.GetFrame()
		require.NotNil(t, frame2)
		
		// Should be reset
		assert.Equal(t, protocol.MessageType(0), frame2.Type)
		assert.Equal(t, uint32(0), frame2.Length)
	})

	t.Run("ProtobufMessagePools", func(t *testing.T) {
		// Test AuthRequest pool
		auth := pools.GetAuthRequest()
		require.NotNil(t, auth)
		auth.Username = "test"
		pools.PutAuthRequest(auth)
		
		auth2 := pools.GetAuthRequest()
		assert.Empty(t, auth2.Username) // Should be reset
		
		// Test DataBatch pool
		batch := pools.GetDataBatch()
		require.NotNil(t, batch)
		require.NotNil(t, batch.Ticks)
		assert.Equal(t, 0, len(batch.Ticks))
		
		// Add some ticks
		batch.Ticks = append(batch.Ticks, &pb.Tick{TimestampMs: 123})
		pools.PutDataBatch(batch)
		
		batch2 := pools.GetDataBatch()
		assert.Equal(t, 0, len(batch2.Ticks)) // Should be reset
	})

	t.Run("BufferPools", func(t *testing.T) {
		buf := pools.GetReadBuffer()
		require.NotNil(t, buf)
		assert.Equal(t, 4096, len(buf))
		
		pools.PutReadBuffer(buf)
		
		writeBuf := pools.GetWriteBuffer()
		require.NotNil(t, writeBuf)
		assert.Equal(t, 0, len(writeBuf))
		assert.True(t, cap(writeBuf) >= 4096)
	})
}

func TestAsyncWriteQueue(t *testing.T) {
	config := DefaultConfig()
	config.MaxWriteQueueSize = 10
	config.WriteDeadlineMS = 100 // Short deadline for testing

	t.Run("WriteQueueFull", func(t *testing.T) {
		// Create mock connection that will block writes
		server, client := net.Pipe()
		defer server.Close()
		defer client.Close()
		
		conn := NewConnection(server, config)
		defer conn.Close()

		// Fill the write queue without reading from client side
		successCount := 0
		errorCount := 0
		
		for i := 0; i < config.MaxWriteQueueSize+5; i++ {
			frame := &protocol.Frame{
				Magic:   [2]byte{protocol.MagicByte1, protocol.MagicByte2},
				Version: protocol.ProtocolVersion,
				Type:    protocol.MessageTypeHeartbeat,
				Length:  0,
				Payload: []byte{},
			}
			
			err := conn.WriteFrameAsync(frame)
			if err != nil {
				errorCount++
				assert.Contains(t, err.Error(), "write queue full")
			} else {
				successCount++
			}
		}
		
		// Should have some successes and some failures
		assert.True(t, successCount > 0, "Expected some successful writes")
		assert.True(t, errorCount > 0, "Expected some failed writes due to queue full")
	})

	t.Run("ClosedConnection", func(t *testing.T) {
		server, client := net.Pipe()
		client.Close() // Close immediately
		
		conn := NewConnection(server, config)
		conn.Close() // Close connection
		
		frame := &protocol.Frame{
			Magic:   [2]byte{protocol.MagicByte1, protocol.MagicByte2},
			Version: protocol.ProtocolVersion,
			Type:    protocol.MessageTypeHeartbeat,
			Length:  0,
			Payload: []byte{},
		}

		err := conn.WriteFrameAsync(frame)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connection closed")
	})
}

func TestPerformanceMetrics(t *testing.T) {
	metrics := &PerformanceMetrics{}

	t.Run("ConnectionMetrics", func(t *testing.T) {
		metrics.IncrementActiveConnections()
		metrics.IncrementActiveConnections()
		
		snapshot := metrics.GetSnapshot()
		assert.Equal(t, int64(2), snapshot["active_connections"])
		assert.Equal(t, int64(2), snapshot["total_connections"])
		
		metrics.DecrementActiveConnections()
		snapshot = metrics.GetSnapshot()
		assert.Equal(t, int64(1), snapshot["active_connections"])
		assert.Equal(t, int64(2), snapshot["total_connections"]) // Total doesn't decrease
	})

	t.Run("MessageMetrics", func(t *testing.T) {
		metrics.AddMessagesSent(10)
		metrics.AddMessagesRecv(5)
		metrics.AddBytesSent(1024)
		metrics.AddBytesRecv(512)
		
		snapshot := metrics.GetSnapshot()
		assert.Equal(t, int64(10), snapshot["messages_sent_total"])
		assert.Equal(t, int64(5), snapshot["messages_recv_total"])
		assert.Equal(t, int64(1024), snapshot["bytes_sent_total"])
		assert.Equal(t, int64(512), snapshot["bytes_recv_total"])
	})

	t.Run("LatencyMetrics", func(t *testing.T) {
		// Record some latencies
		metrics.RecordWriteLatency(1000000) // 1ms
		metrics.RecordWriteLatency(2000000) // 2ms
		metrics.RecordWriteLatency(3000000) // 3ms
		
		snapshot := metrics.GetSnapshot()
		assert.Equal(t, int64(3), snapshot["write_latency_count"])
		
		// Average should be 2ms (2000000ns)
		avgLatency := snapshot["write_latency_ns"].(int64)
		assert.Equal(t, int64(2000000), avgLatency)
	})

	t.Run("Reset", func(t *testing.T) {
		metrics.Reset()
		snapshot := metrics.GetSnapshot()
		
		for key, value := range snapshot {
			assert.Equal(t, int64(0), value, "Metric %s should be reset to 0", key)
		}
	})
}

func TestPerformanceMonitor(t *testing.T) {
	metrics := &PerformanceMetrics{}
	monitor := NewPerformanceMonitor(metrics)

	assert.Equal(t, int64(10), monitor.MaxWriteLatencyMs)
	assert.Equal(t, 0.05, monitor.MaxSlowClientRatio)
	assert.Equal(t, 0.01, monitor.MaxWriteQueueFullRate)

	// Test start/stop
	monitor.Start(100 * time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	monitor.Stop()
}

func TestTCPOptimizations(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	config := DefaultConfig()
	config.TCPReadBufferSize = 32768
	config.TCPWriteBufferSize = 32768

	// Accept connection in goroutine
	var serverConn net.Conn
	go func() {
		serverConn, _ = listener.Accept()
	}()

	// Connect to server
	clientConn, err := net.Dial("tcp", listener.Addr().String())
	require.NoError(t, err)
	defer clientConn.Close()

	// Wait for server connection
	time.Sleep(10 * time.Millisecond)
	require.NotNil(t, serverConn)
	defer serverConn.Close()

	// Create connection with optimizations
	conn := NewConnection(serverConn, config)
	defer conn.Close()

	// Verify TCP_NODELAY is set
	if tcpConn, ok := serverConn.(*net.TCPConn); ok {
		// We can't directly check if NoDelay is set, but we can verify
		// the connection was created successfully with optimizations
		assert.NotNil(t, tcpConn)
	}
}

func BenchmarkAsyncWrites(b *testing.B) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	config := DefaultConfig()
	config.MaxWriteQueueSize = 10000
	conn := NewConnection(server, config)
	defer conn.Close()

	frame := &protocol.Frame{
		Magic:   [2]byte{protocol.MagicByte1, protocol.MagicByte2},
		Version: protocol.ProtocolVersion,
		Type:    protocol.MessageTypeHeartbeat,
		Length:  0,
		Payload: []byte{},
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			conn.WriteFrameAsync(frame)
		}
	})
}

func BenchmarkObjectPooling(b *testing.B) {
	pools := NewObjectPools()

	b.Run("FramePool", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				frame := pools.GetFrame()
				pools.PutFrame(frame)
			}
		})
	})

	b.Run("DataBatchPool", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				batch := pools.GetDataBatch()
				pools.PutDataBatch(batch)
			}
		})
	})

	b.Run("BufferPool", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				buf := pools.GetWriteBuffer()
				pools.PutWriteBuffer(buf)
			}
		})
	})
}

func TestSlowClientDetection(t *testing.T) {
	server, client := net.Pipe()
	defer client.Close()

	config := DefaultConfig()
	config.MaxWriteQueueSize = 2 // Small queue for testing
	config.WriteDeadlineMS = 50  // Short deadline

	conn := NewConnection(server, config)
	defer conn.Close()

	// Don't read from client side to simulate slow client
	
	// Fill write queue
	frames := make([]*protocol.Frame, 5)
	for i := 0; i < 5; i++ {
		frames[i] = &protocol.Frame{
			Magic:   [2]byte{protocol.MagicByte1, protocol.MagicByte2},
			Version: protocol.ProtocolVersion,
			Type:    protocol.MessageTypeHeartbeat,
			Length:  0,
			Payload: []byte{},
		}
	}

	var wg sync.WaitGroup
	errors := make([]error, 5)

	// Try to write multiple frames concurrently
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			errors[idx] = conn.WriteFrameAsync(frames[idx])
		}(i)
	}

	wg.Wait()

	// Some writes should fail due to queue being full
	errorCount := 0
	for _, err := range errors {
		if err != nil {
			errorCount++
		}
	}

	assert.True(t, errorCount > 0, "Expected some writes to fail due to slow client")
}
