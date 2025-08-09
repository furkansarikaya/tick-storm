# FEATURE LIST
## Golang TCP Stream Server (Binary Framing + Protobuf)

**Version:** 1.0  
**Date:** 2025-08-08  

This feature list aligns strictly with the "Golang TCP Stream Server (Binary Framing + Protobuf)" PRD.

## 1) Core Features

### 1.1 TCP Transport
- Raw TCP server with custom binary framing.
- Length-prefixed protocol with header and CRC32C checksum.
- Optional TLS 1.3 and mTLS support.

### 1.2 Authentication
- Mandatory authentication on first frame after connection.
- Credentials validated against environment variables.
- Invalid credentials trigger ERROR frame and connection close.

### 1.3 Subscription Management
- Client specifies mode: SECOND or MINUTE.
- ACK confirmation upon successful subscription.
- Only one active subscription mode per connection.

### 1.4 Data Delivery
- DATA_BATCH frames containing one or more Tick messages.
- Micro-batching to reduce per-message overhead.
- Each Tick includes mode and epoch_ms timestamp.

### 1.5 Heartbeat Mechanism
- Client must send HEARTBEAT at configured interval (default: 15s).
- Server closes connection if heartbeat missing within timeout (20s).
- Optional ACK (PONG) response to heartbeat.

## 2) Protocol Features

### 2.1 Framing
- Header format: `[Magic(2B)][Ver(1B)][Type(1B)][Len(4B)][Payload][CRC32C(4B)]`.
- Protobuf serialization for payloads.
- Protocol version field for backward compatibility.

### 2.2 Message Types
- AUTH (0x01): username/password.
- SUBSCRIBE (0x02): mode selection.
- HEARTBEAT (0x03): connection keepalive.
- DATA_BATCH (0x04): batched tick data.
- ERROR (0x05): error reporting.

## 3) Performance & Scalability

### 3.1 Performance
- p50 latency < 1ms, p95 latency < 5ms in local network tests.
- Handle bursts of 10k+ messages/sec without packet loss.
- Efficient buffer reuse to minimize GC impact.

### 3.2 Scalability
- Supports 100k+ concurrent connections on optimized hardware.
- Horizontal scaling through TCP load balancers.
- Configurable micro-batching window (default: 5ms).

## 4) Observability & Monitoring

### 4.1 Metrics
- active_connections (gauge).
- messages_sent_total (counter).
- bytes_sent_total (counter).
- heartbeat_timeouts_total (counter).
- publish_latency_ms (histogram).

### 4.2 Logging
- Structured JSON logs.
- Configurable log levels.

### 4.3 Tracing
- Optional OpenTelemetry integration.

## 5) Security Features
- TLS 1.3 encryption (optional plain TCP in trusted environments).
- Optional mTLS for client authentication.
- Rate limiting on authentication attempts.
- CRC32C payload checksum for data integrity.

## 6) Reliability Features
- Write deadline enforcement to handle slow clients.
- Backpressure control with bounded write queues.
- Safe connection termination on protocol violations.

## 7) Acceptance Criteria Mapping
- Each feature directly maps to an acceptance criterion in the PRD.
- No features beyond the PRD scope are included.