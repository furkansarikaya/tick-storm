# ARCHITECTURE DIAGRAM + DEPLOYMENT & OPS RUNBOOK
## Golang TCP Stream Server (Binary Framing + Protobuf)

**Version:** 1.0 | **Date:** 2025-08-08

## High-Level Architecture

The system uses raw TCP with a custom binary protocol. Clients connect through an L4 TCP load balancer to one of the stateless TCP Stream Server instances. The server validates AUTH on the first frame, accepts SUBSCRIBE, emits DATA_BATCH frames with micro-batching, and enforces HEARTBEAT timeouts. Metrics and logs are exported for observability and alerting.

```
Clients (TCP)           L4 TCP Load Balancer       TCP Stream Server
Producers/Consumers  →                          →  Modules:
Auth + Heartbeat                                   • protocol
                                                   • auth
                                                   • subscription
                                                   • delivery/batching
                                                   • heartbeat
                                                           ↓
                        Alerting              ←    Metrics & Logs
                     Alertmanager/Grafana         Prometheus/JSON Logs
```

## Component Interaction Flow

1. Client establishes a TCP connection via the load balancer.
2. First frame must be AUTH (username/password in Protobuf).
3. Server validates credentials from environment variables.
4. Client sends SUBSCRIBE (mode: SECOND or MINUTE).
5. Server starts DATA_BATCH emission with micro-batching (default 5ms window).
6. Client sends HEARTBEAT every 15s; server closes connection if no heartbeat within 20s.
7. Metrics/logs continuously exported; alerts fire on SLO/SLA breaches.

## Deployment Steps (Dev → Staging → Prod)

**Prerequisites:** Docker, container registry, TLS certificates (optional), Prometheus & Grafana stack, Alertmanager.

### 1. Build & Image
- Compile with Go 1.22+: `CGO_ENABLED=0 go build -ldflags='-s -w'`.
- Create a minimal image (e.g., distroless).
- Push to registry.

### 2. Configuration
- STREAM_USER / STREAM_PASS (secrets).
- HEARTBEAT_INTERVAL=15s, HEARTBEAT_TIMEOUT=20s.
- BATCH_WINDOW_MS=5, MAX_MSG_SIZE=65536, WRITE_DEADLINE_MS=5000.
- TLS_CERT_PATH / TLS_KEY_PATH if TLS enabled.

### 3. Networking
- Expose TCP port via Service/LoadBalancer (L4).
- Enable TCP_NODELAY; adjust send/recv buffers.
- Health port or readiness probe for orchestration.

### 4. Observability
- Expose Prometheus metrics endpoint.
- Configure Grafana dashboards (active_connections, publish_latency_ms, bytes_sent_total).
- Alert rules: heartbeat_timeouts_total spike, latency p95 > 5ms, connection drops.

### 5. Rollout
- Staged rollout with small canary (e.g., 5%).
- Monitor SLOs; expand to 25% → 50% → 100%.
- Rollback on regression (keep previous image tagged).

## Configuration Examples

**Environment variables:**
```
STREAM_USER=stream_user
STREAM_PASS=stream_pass
HEARTBEAT_INTERVAL=15s
HEARTBEAT_TIMEOUT=20s
BATCH_WINDOW_MS=5
MAX_MSG_SIZE=65536
WRITE_DEADLINE_MS=5000
TLS_CERT_PATH=/etc/tls/cert.pem (optional)
TLS_KEY_PATH=/etc/tls/key.pem (optional)
```

## Operational Procedures

### Health & Readiness
- Monitor metrics; a simple TCP connect check validates accept loop.
- Readiness gate: server must accept connections and export metrics.

### Scaling
- Horizontal scale based on active_connections and CPU.
- Keep LB connection timeout > heartbeat timeout to avoid premature drops.

### Backpressure & Slow Clients
- Enforce write queue limits and WRITE_DEADLINE_MS; drop slow connections.

### Incident Handling
1. Identify symptom (latency spike, drops).
2. Check metrics: publish_latency_ms, heartbeat_timeouts_total, bytes_sent_total.
3. Inspect logs for protocol violations or auth failures.
4. Scale out, or temporarily widen batch window to absorb bursts.
5. If needed, roll back to last known good image.

### Security
- Rotate STREAM_PASS via secret update and rolling restart.
- Enable TLS/mTLS in untrusted networks.
- Rate limit AUTH attempts; monitor ERROR frames.