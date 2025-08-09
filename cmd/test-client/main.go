package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/furkansarikaya/tick-storm/internal/protocol"
	pb "github.com/furkansarikaya/tick-storm/internal/protocol/pb"
	"google.golang.org/protobuf/proto"
)

func main() {
	// Connect to server
	serverAddr := "localhost:8080"
	if len(os.Args) > 1 {
		serverAddr = os.Args[1]
	}

	log.Printf("Connecting to %s...", serverAddr)
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	log.Println("Connected successfully")

	// Test 1: Send AUTH frame
	authReq := &pb.AuthRequest{
		Username: os.Getenv("STREAM_USER"),
		Password: os.Getenv("STREAM_PASS"),
		ClientId: "test-client-001",
		Version:  "1.0.0",
	}

	if authReq.Username == "" || authReq.Password == "" {
		log.Fatal("STREAM_USER and STREAM_PASS environment variables must be set")
	}

	payload, err := proto.Marshal(authReq)
	if err != nil {
		log.Fatalf("Failed to marshal auth request: %v", err)
	}

	authFrame := &protocol.Frame{
		Type:    protocol.MessageTypeAuth,
		Payload: payload,
	}

	// Send AUTH frame
	if err := sendFrame(conn, authFrame); err != nil {
		log.Fatalf("Failed to send AUTH frame: %v", err)
	}
	log.Println("Sent AUTH frame")

	// Read AUTH response
	respFrame, err := readFrame(conn)
	if err != nil {
		log.Fatalf("Failed to read AUTH response: %v", err)
	}

	if respFrame.Type == protocol.MessageTypeACK {
		var ack pb.AckResponse
		if err := proto.Unmarshal(respFrame.Payload, &ack); err != nil {
			log.Fatalf("Failed to unmarshal ACK: %v", err)
		}
		log.Printf("AUTH successful: %s", ack.Message)
	} else if respFrame.Type == protocol.MessageTypeError {
		var errResp pb.ErrorResponse
		if err := proto.Unmarshal(respFrame.Payload, &errResp); err != nil {
			log.Fatalf("Failed to unmarshal error: %v", err)
		}
		log.Fatalf("AUTH failed: %s", errResp.Message)
	} else {
		log.Fatalf("Unexpected response type: %d", respFrame.Type)
	}

	// Test 2: Send SUBSCRIBE frame
	subReq := &pb.SubscriptionRequest{
		Mode: pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND,
	}

	subPayload, err := proto.Marshal(subReq)
	if err != nil {
		log.Fatalf("Failed to marshal subscription request: %v", err)
	}

	subFrame := &protocol.Frame{
		Type:    protocol.MessageTypeSubscribe,
		Payload: subPayload,
	}

	if err := sendFrame(conn, subFrame); err != nil {
		log.Fatalf("Failed to send SUBSCRIBE frame: %v", err)
	}
	log.Println("Sent SUBSCRIBE frame")

	// Read subscription confirmation
	subResp, err := readFrame(conn)
	if err != nil {
		log.Fatalf("Failed to read subscription response: %v", err)
	}

	if subResp.Type == protocol.MessageTypeACK {
		var ack pb.AckResponse
		if err := proto.Unmarshal(subResp.Payload, &ack); err != nil {
			log.Fatalf("Failed to unmarshal subscription ACK: %v", err)
		}
		log.Printf("Subscription successful: %s", ack.Message)
	} else {
		log.Fatalf("Unexpected subscription response type: %d", subResp.Type)
	}

	// Test 3: Receive data and heartbeats
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	log.Println("Waiting for data and heartbeats...")
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				frame, err := readFrame(conn)
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					log.Printf("Read error: %v", err)
					return
				}

				switch frame.Type {
				case protocol.MessageTypeHeartbeat:
					log.Println("Received HEARTBEAT, sending PONG...")
					// Send PONG
					pongFrame := &protocol.Frame{
						Type:    protocol.MessageTypeHeartbeat,
						Payload: []byte{},
					}
					if err := sendFrame(conn, pongFrame); err != nil {
						log.Printf("Failed to send PONG: %v", err)
					}

				case protocol.MessageTypeDataBatch:
					var batch pb.DataBatch
					if err := proto.Unmarshal(frame.Payload, &batch); err != nil {
						log.Printf("Failed to unmarshal data batch: %v", err)
						continue
					}
					log.Printf("Received data batch with %d ticks", len(batch.Ticks))
					for i, tick := range batch.Ticks {
						if i < 3 { // Show first 3 ticks
							log.Printf("  Tick %d: Symbol=%s, Price=%.2f, Volume=%d, Timestamp=%d",
								i+1, tick.Symbol, tick.Price, tick.Volume, tick.Timestamp)
						}
					}

				default:
					log.Printf("Received frame type: %d", frame.Type)
				}
			}
		}
	}()

	// Wait for context or user interrupt
	<-ctx.Done()
	log.Println("Test client shutting down...")
}

func sendFrame(conn net.Conn, frame *protocol.Frame) error {
	data, err := frame.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal frame: %w", err)
	}

	if _, err := conn.Write(data); err != nil {
		return fmt.Errorf("failed to write frame: %w", err)
	}

	return nil
}

func readFrame(conn net.Conn) (*protocol.Frame, error) {
	// Read frame header first (12 bytes)
	header := make([]byte, 12)
	if _, err := conn.Read(header); err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	// Parse header to get payload length
	frame := &protocol.Frame{}
	if err := frame.Unmarshal(header); err != nil {
		// Need to read full frame
		payloadLen := uint32(header[4]) | uint32(header[5])<<8 | uint32(header[6])<<16 | uint32(header[7])<<24
		
		// Read remaining data
		remaining := make([]byte, payloadLen+4) // payload + checksum
		if _, err := conn.Read(remaining); err != nil {
			return nil, fmt.Errorf("failed to read remaining frame: %w", err)
		}

		// Combine and unmarshal
		fullFrame := append(header, remaining...)
		frame = &protocol.Frame{}
		if err := frame.Unmarshal(fullFrame); err != nil {
			return nil, fmt.Errorf("failed to unmarshal frame: %w", err)
		}
	}

	return frame, nil
}
