# TECHNICAL SPECIFICATION
## Golang TCP Stream Server (Binary Framing + Protobuf)

**Version:** 1.0  
**Date:** 2025-08-08  

This technical specification is aligned with the PRD and Feature List for the Golang TCP Stream Server.

## 1) Technology Stack

- **Language:** Go (>= 1.22)
- **Serialization:** Protobuf v3
- **Transport:** Raw TCP (net package) with optional TLS 1.3 (crypto/tls)
- **Build:** Go modules
- **Deployment:** Docker + Kubernetes (optional)
- **Monitoring:** Prometheus, OpenTelemetry (optional)

## 2) Architecture

**Single binary application with internal modules:**
- **server:** TCP listener, connection accept loop
- **protocol:** framing, message parsing, serialization/deserialization
- **auth:** credential validation
- **subscription:** mode management
- **delivery:** tick generation, batching, sending
- **heartbeat:** timeout detection
- **metrics:** Prometheus exporters
- **logging:** structured JSON logging

**Concurrency Model:**
- goroutine per connection (for <= 100k connections on tuned hardware)
- netpoll/event loop approach considered for > 100k scale
- channels or lock-free queues for internal event passing

## 3) Protocol Implementation

### 3.1 Framing
- **Header:** 2B Magic, 1B Version, 1B Type, 4B Length, Payload, 4B CRC32C
- **Length:** big-endian uint32
- **CRC32C:** Castagnoli polynomial for speed and collision resistance
- **Payload:** Protobuf-encoded message

### 3.2 Message Types (Type Field)
- **0x01 AUTH:** Auth { username, password }
- **0x02 SUBSCRIBE:** Subscribe { mode }
- **0x03 HEARTBEAT:** Empty message
- **0x04 DATA_BATCH:** DataBatch { schema_version, repeated Tick }
- **0x05 ERROR:** ErrorMsg { code, message }

## 4) Serialization

- Protobuf encoding for all payloads
- Precompiled .proto definitions using protoc-gen-go
- Memory pooling for Protobuf message objects to reduce GC pressure

## 5) Data Flow

1. Client connects via TCP.
2. Server reads first frame:
   - If AUTH: validate credentials
   - Else: close connection (protocol violation)
3. On successful auth, wait for SUBSCRIBE.
4. Start sending DATA_BATCH according to mode (SECOND/MINUTE).
5. Maintain heartbeat timer; close if missed.

## 6) Performance Optimizations

- TCP_NODELAY enabled to avoid Nagle delays.
- TCP keepalive enabled to detect dead connections.
- Write buffering with pre-allocated byte slices.
- sync.Pool for reusable buffers and Protobuf objects.
- Micro-batching window: default 5ms to aggregate ticks.
- Asynchronous write loop per connection to avoid blocking readers.
- Configurable maximum message size (e.g., 64KB).

## 7) Scalability Considerations

- Horizontal scaling behind L4 TCP load balancer.
- Stateless server; session state stored in connection struct.
- Configurable goroutine scheduling and GOMAXPROCS tuning.
- Optional gnet/netpoll-based event loop for extreme scale.

## 8) Observability

**Prometheus metrics:**
- active_connections
- messages_sent_total
- bytes_sent_total
- heartbeat_timeouts_total
- publish_latency_ms

- Structured JSON logs with log level control.
- Optional OpenTelemetry tracing for debugging.

## 9) Security

- TLS 1.3 optional; mTLS for mutual authentication.
- Rate limiting on AUTH attempts to prevent brute-force attacks.
- CRC32C checksum verification for payload integrity.

## 10) Resource Management

- Connection write queue size limit (backpressure).
- Write deadline (default 5s) to drop slow clients.
- Read deadline (heartbeat timeout + grace period).
- Graceful shutdown with context cancellation.

## 11) Configuration Parameters

- **STREAM_USER, STREAM_PASS:** authentication credentials.
- **HEARTBEAT_INTERVAL** (default: 15s)
- **HEARTBEAT_TIMEOUT** (default: 20s)
- **BATCH_WINDOW_MS** (default: 5ms)
- **MAX_MSG_SIZE** (default: 64KB)
- **WRITE_DEADLINE_MS** (default: 5000ms)

## 12) Testing Strategy

- Unit tests for framing, serialization, and auth logic.
- Integration tests for end-to-end TCP sessions.
- Load testing with 100k+ simulated clients using custom Go load generator.
- Soak testing for memory leaks and performance degradation.

## 13) Deployment & Operations

- Containerized deployment (Docker).
- Health checks: TCP accept test + internal goroutine health metrics.
- Rolling updates with zero downtime using Kubernetes or similar.

## 14) Acceptance Criteria Alignment

- All performance, security, and functional targets in PRD and Feature List met.
- Verified under load and latency conditions described.