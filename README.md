# TickStorm - High-Performance TCP Stream Server

*Built for performance, designed for scale.*

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![Performance](https://img.shields.io/badge/Latency-<1ms-0066CC?style=flat)](https://github.com/furkansarikaya/tick-storm)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat&logo=docker)](https://www.docker.com/)
[![License](https://img.shields.io/badge/License-MIT-38A169?style=flat)](LICENSE)

**TickStorm** is a high-performance TCP stream server written in Go that delivers time-based tick data using binary framing with Protobuf serialization. Designed for financial data streaming with sub-millisecond latency and support for 100k+ concurrent connections.

## 🚀 Features

### Core Functionality
- **High-Performance TCP Server**: Goroutine-per-connection model with optimized networking
- **Binary Protocol**: Custom framing with Protobuf serialization for minimal overhead
- **Authentication**: Mandatory client authentication with rate limiting
- **Subscription Modes**: SECOND or MINUTE tick delivery modes
- **Micro-Batching**: 5ms batching window to optimize throughput
- **Heartbeat Monitoring**: Client health tracking with configurable timeouts

### Security & Transport
- **TLS 1.3 Support**: Enterprise-grade transport security
- **Mutual TLS (mTLS)**: Client certificate authentication
- **Strong Cryptography**: AES-256-GCM, ChaCha20-Poly1305 cipher suites
- **Certificate Validation**: Full certificate chain and expiration checking

### Performance Optimizations
- **TCP Optimizations**: TCP_NODELAY, optimized buffer sizes
- **Object Pooling**: Frame and message pooling to reduce GC pressure
- **Async Write Queues**: Non-blocking writes with backpressure handling
- **CRC32C Checksums**: Hardware-accelerated integrity validation

### Monitoring & Observability
- **Comprehensive Metrics**: Connection counts, throughput, latency tracking
- **TLS Metrics**: Handshake performance, cipher suite usage
- **Health Checks**: Container-ready health endpoints
- **Structured Logging**: JSON logging with configurable levels

## 📋 Protocol Specification

### Frame Format
```
[Magic(2B)][Ver(1B)][Type(1B)][Len(4B)][Payload][CRC32C(4B)]
```

### Message Types
- `0x01 AUTH`: Client authentication
- `0x02 SUBSCRIBE`: Subscription mode selection
- `0x03 HEARTBEAT`: Keepalive signal
- `0x04 DATA_BATCH`: Batched tick data
- `0x05 ERROR`: Error reporting

## 🛠 Installation

### Prerequisites
- Go 1.22 or higher
- Docker (optional, for containerized deployment)

### Build from Source
```bash
git clone https://github.com/furkansarikaya/tick-storm.git
cd tick-storm
go mod download
go build -o tick-storm ./cmd/server
```

### Docker Build
```bash
# Build minimal Docker image (<20MB)
./scripts/docker-build.sh

# Or use Docker Compose
docker-compose up --build
```

## ⚙️ Configuration

TickStorm is configured via environment variables:

### Server Configuration
```bash
LISTEN_ADDR=0.0.0.0:8080          # Server listen address
MAX_CONNECTIONS=100000             # Maximum concurrent connections
WRITE_DEADLINE_MS=5000            # Write timeout in milliseconds
HEARTBEAT_TIMEOUT_MS=20000        # Heartbeat timeout
HEARTBEAT_INTERVAL_MS=15000       # Expected heartbeat interval
```

### Performance Tuning
```bash
TCP_READ_BUFFER_SIZE=65536        # TCP read buffer size
TCP_WRITE_BUFFER_SIZE=65536       # TCP write buffer size
MAX_WRITE_QUEUE_SIZE=1000         # Async write queue size
BATCH_WINDOW_MS=5                 # Micro-batching window
```

### Authentication
```bash
AUTH_USERNAME=admin               # Authentication username
AUTH_PASSWORD=secure123           # Authentication password
```

### TLS Configuration
```bash
TLS_ENABLED=true                  # Enable TLS
TLS_CERT_FILE=/path/to/cert.pem   # Server certificate
TLS_KEY_FILE=/path/to/key.pem     # Server private key
TLS_CLIENT_AUTH=require_verify    # Client certificate mode
TLS_CA_FILE=/path/to/ca.pem       # CA certificate for client validation
```

## 🚀 Quick Start

### Basic Server
```bash
# Start server with default configuration
./tick-storm

# Start with custom configuration
LISTEN_ADDR=0.0.0.0:9090 MAX_CONNECTIONS=50000 ./tick-storm
```

### Docker Deployment
```bash
# Using Docker Compose (recommended)
docker-compose up -d

# Direct Docker run
docker run -p 8080:8080 \
  -e LISTEN_ADDR=0.0.0.0:8080 \
  -e AUTH_USERNAME=admin \
  -e AUTH_PASSWORD=secure123 \
  tick-storm:latest
```

### Client Connection Flow
1. **Connect**: Establish TCP connection to server
2. **Authenticate**: Send AUTH frame with credentials
3. **Subscribe**: Send SUBSCRIBE frame with mode (SECOND/MINUTE)
4. **Heartbeat**: Send HEARTBEAT frames every 15 seconds
5. **Receive Data**: Process incoming DATA_BATCH frames

## 📊 Performance Targets

- **Latency**: p50 < 1ms, p95 < 5ms
- **Throughput**: 100k+ concurrent connections per instance
- **Memory**: < 1GB per instance at peak load
- **CPU**: < 70% utilization at peak throughput

## 🔧 Development

### Running Tests
```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific test suite
go test ./internal/server -v
```

### Building
```bash
# Development build
go build -o tick-storm ./cmd/server

# Production build (static binary)
CGO_ENABLED=0 GOOS=linux go build -ldflags='-w -s' -o tick-storm ./cmd/server
```

## 📈 Monitoring

### Health Check
```bash
# Container health check
./tick-storm -health-check

# HTTP health endpoint (if enabled)
curl http://localhost:8080/health
```

### Metrics
Server exposes comprehensive metrics including:
- Active connections count
- Message throughput rates
- Write queue performance
- TLS handshake metrics
- Authentication success/failure rates

## 🐳 Container Deployment

### Kubernetes
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tick-storm
spec:
  replicas: 3
  selector:
    matchLabels:
      app: tick-storm
  template:
    metadata:
      labels:
        app: tick-storm
    spec:
      containers:
      - name: tick-storm
        image: tick-storm:latest
        ports:
        - containerPort: 8080
        env:
        - name: LISTEN_ADDR
          value: "0.0.0.0:8080"
        - name: MAX_CONNECTIONS
          value: "50000"
        resources:
          limits:
            memory: "1Gi"
            cpu: "2"
          requests:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          exec:
            command: ["/tick-storm", "-health-check"]
          initialDelaySeconds: 10
          periodSeconds: 30
```

## 🔒 Security

### TLS/mTLS Configuration
TickStorm supports enterprise-grade TLS security:
- TLS 1.3 enforcement
- Strong cipher suites (AES-256-GCM, ChaCha20-Poly1305)
- Client certificate authentication
- Certificate rotation support
- OCSP validation (placeholder)

### Security Best Practices
- Use strong authentication credentials
- Enable TLS in production environments
- Implement proper certificate management
- Monitor authentication failure rates
- Use non-root containers

## 📚 Documentation

- [Architecture Overview](docs/ARCHITECTURE.md)
- [Protocol Specification](docs/PROTOCOL.md)
- [Version Migration Guide](docs/VERSION_MIGRATION.md)
- [Deployment Guide](docs/DEPLOYMENT.md)

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🏗 Architecture

TickStorm follows clean architecture principles with clear separation of concerns:

```
├── cmd/server/          # Application entry point
├── internal/
│   ├── auth/           # Authentication & rate limiting
│   ├── protocol/       # Protocol implementation & versioning
│   └── server/         # TCP server, connection handling, TLS
├── api/proto/          # Protobuf definitions
├── docs/              # Documentation
└── scripts/           # Build & deployment scripts
```

## 🎯 Roadmap

- [ ] Redis integration for session storage
- [ ] Prometheus metrics export
- [ ] gRPC management API
- [ ] Load balancer health checks
- [ ] Certificate auto-renewal
- [ ] Multi-region deployment support

---

**TickStorm** - Built for performance, designed for scale. 🚀
