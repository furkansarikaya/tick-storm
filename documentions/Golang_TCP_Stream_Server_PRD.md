# PRODUCT REQUIREMENTS DOCUMENT (PRD)
## Golang TCP Stream Server (Binary Framing + Protobuf)

**Version:** 1.0  
**Date:** 2025-08-08  

## 1) Overview

**Purpose:** High-performance, low-latency TCP stream server that delivers time-based tick data (every second or minute) to authenticated clients. Uses binary length-prefixed framing with Protobuf serialization for minimal overhead. Supports high-frequency, large-volume data streaming with mandatory heartbeat checks.

**Scope:** Raw TCP with custom binary protocol. Clients must authenticate upon connection, select a subscription mode (second or minute), and maintain heartbeats to stay connected.

**Goals:**
- p50 publish latency < 1ms (LAN), p95 < 5ms
- Support 100k+ concurrent connections on optimized hardware
- Micro-batching for bursts of high-frequency data
- Protocol versioning for forward compatibility

**Out of Scope:**
- Persistent user management
- WebSocket, HTTP, or gRPC transport
- Complex message routing

## 2) Stakeholders

- Product Owner
- Backend Developer(s) (Go)
- DevOps/SRE
- QA/Performance Testing

## 3) User Stories

**US-1:** As a client, I want to authenticate immediately after connecting via TCP using binary protocol.

**US-2:** As a client, I want to subscribe to second or minute tick streams.

**US-3:** As a client, I want my connection dropped if I miss heartbeat intervals.

**US-4:** As a client, I want to receive batched ticks to reduce per-message overhead.

**US-5:** As an operator, I want to monitor connection count, throughput, and latency.

## 4) Functional Requirements

### FR-1 Authentication (Mandatory):
- First frame after connection: AUTH message (Protobuf) in binary frame.
- If invalid, send ERROR frame and close connection.
- Credentials stored in environment variables.

### FR-2 Subscription:
- After auth success: SUBSCRIBE message (mode: SECOND or MINUTE).
- Server responds with ACK.

### FR-3 Data Delivery:
- DATA_BATCH message contains multiple Tick records (micro-batching).
- Tick: mode, epoch_ms.
- Framing: length-prefixed (4 bytes, big-endian) or varint (Protobuf-native).

### FR-4 Heartbeat:
- Client sends HEARTBEAT message at configured interval (e.g., every 15s).
- Server closes connection if no heartbeat within timeout (20s).

### FR-5 Error Handling:
- ERROR message: code + message fields.

## 5) Protocol Specification

**Transport:** TCP (TLS optional/mTLS supported)

**Framing:** `[Magic(2B)][Ver(1B)][Type(1B)][Len(4B)][Payload(Len B)][CRC32C(4B)]`

**Message Types:**
- 0x01 = AUTH
- 0x02 = SUBSCRIBE
- 0x03 = HEARTBEAT
- 0x04 = DATA_BATCH
- 0x05 = ERROR

## 6) Protobuf Message Schema

```protobuf
syntax = "proto3";

package stream;

enum Mode { 
    SECOND = 0; 
    MINUTE = 1; 
}

enum MsgType { 
    AUTH = 0; 
    SUBSCRIBE = 1; 
    HEARTBEAT = 2; 
    DATA_BATCH = 3; 
    ERROR = 4; 
}

message Auth { 
    string username = 1; 
    string password = 2; 
}

message Subscribe { 
    Mode mode = 1; 
}

message Tick { 
    Mode mode = 1; 
    int64 epoch_ms = 2; 
}

message DataBatch { 
    uint32 schema_version = 1; 
    repeated Tick ticks = 2; 
}

message ErrorMsg { 
    string code = 1; 
    string message = 2; 
}
```

## 7) Performance Requirements

- **Target throughput:** 1M+ messages/sec (aggregate) with batching
- **Target latency:** p50 < 1ms, p95 < 5ms on local network
- **CPU utilization** under 70% at peak load
- **Memory utilization** under 1GB for 100k connections

## 8) Scalability

- Event-driven I/O model (Go netpoll or goroutine-per-conn depending on target)
- Horizontal scaling via TCP load balancers
- Micro-batching window: configurable (default 5ms)

## 9) Observability

**Metrics:**
- active_connections
- messages_sent_total
- bytes_sent_total
- heartbeat_timeouts_total
- publish_latency_ms

**Logging:** structured JSON  
**Optional tracing** with OpenTelemetry

## 10) Security

- TLS 1.3 for encrypted transport (optional plain TCP in trusted networks)
- mTLS for client authentication (optional)
- Rate limiting on auth attempts
- Payload CRC32C for data integrity

## 11) Acceptance Criteria

**AC-1:** No AUTH on first frame → close connection

**AC-2:** Valid AUTH + SUBSCRIBE → receive ticks as configured

**AC-3:** Missed heartbeat → close connection within timeout

**AC-4:** Handle bursts of 10k messages/sec without packet loss

**AC-5:** Pass load test of 100k concurrent connections with target latency met

## 12) Example Flow

1. Client connects (TCP)
2. → AUTH frame (Protobuf)
3. ← ACK frame
4. → SUBSCRIBE frame
5. ← ACK frame
6. ← DATA_BATCH frames (binary, length-prefixed)
7. → HEARTBEAT frames at interval