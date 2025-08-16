# TickStorm - High-Performance TCP Stream Server

*Built for performance, designed for scale.*

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![Performance](https://img.shields.io/badge/Latency-<1ms-0066CC?style=flat)](/)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat&logo=docker)](/)
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
- **TLS 1.3 Support**: Optional encrypted transport with mutual TLS
- **Client Certificate Validation**: mTLS authentication support
- **Rate Limiting**: 10 authentication attempts per minute per IP
- **Input Validation**: Comprehensive frame and message validation

### Performance & Scale
- **Sub-millisecond Latency**: p50 < 1ms, p95 < 5ms response times
- **High Concurrency**: Support for 100k+ simultaneous connections
- **Memory Efficient**: < 1GB memory usage per 100k connections
- **Optimized Networking**: TCP_NODELAY, buffer pooling, write batching

### Monitoring & Operations
- **Health Checks**: Built-in health endpoints for container orchestration
- **Metrics**: Prometheus-compatible metrics export
- **Structured Logging**: JSON logging with configurable levels
- **Graceful Shutdown**: Clean connection termination and resource cleanup

## 📋 Quick Start

### Prerequisites
- Go 1.22 or later
- Docker (optional, for containerized deployment)

### Installation

#### From Source
```bash
git clone https://github.com/furkansarikaya/tick-storm.git
cd tick-storm
go build -o tickstorm ./cmd/server
./tickstorm
```

#### Using Docker
```bash
docker run -p 8080:8080 tickstorm:latest
```

#### Using Docker Compose
```bash
docker-compose up -d
```

## 🔧 Configuration

### Environment Variables

#### Server Configuration
| Variable | Default | Description |
|----------|---------|-------------|
| `LISTEN_ADDR` | `0.0.0.0:8080` | Server listen address |
| `MAX_CONNECTIONS` | `100000` | Maximum concurrent connections |
| `WRITE_DEADLINE_MS` | `5000` | Write timeout in milliseconds |
| `HEARTBEAT_TIMEOUT_MS` | `20000` | Client heartbeat timeout |
| `HEARTBEAT_INTERVAL_MS` | `15000` | Expected heartbeat interval |

#### Authentication
| Variable | Default | Description |
|----------|---------|-------------|
| `AUTH_USERNAME` | `admin` | Authentication username |
| `AUTH_PASSWORD` | `password` | Authentication password |

#### TLS Configuration
| Variable | Default | Description |
|----------|---------|-------------|
| `TLS_ENABLED` | `false` | Enable TLS transport |
| `TLS_CERT_FILE` | `""` | TLS certificate file path |
| `TLS_KEY_FILE` | `""` | TLS private key file path |
| `TLS_CLIENT_AUTH` | `NoClientCert` | Client certificate requirement |

#### Performance Tuning
| Variable | Default | Description |
|----------|---------|-------------|
| `TCP_READ_BUFFER_SIZE` | `65536` | TCP read buffer size |
| `TCP_WRITE_BUFFER_SIZE` | `65536` | TCP write buffer size |
| `MAX_WRITE_QUEUE_SIZE` | `1000` | Maximum write queue size |
| `BATCH_WINDOW_MS` | `5` | Micro-batching window |

## 🐳 Docker Deployment

### Building the Image
```bash
# Build the Docker image
docker build -t tickstorm:latest .

# Or use the build script
./scripts/docker-build.sh
```

### Running with Docker
```bash
# Basic run
docker run -p 8080:8080 tickstorm:latest

# With environment variables
docker run -p 8080:8080 \
  -e AUTH_USERNAME=myuser \
  -e AUTH_PASSWORD=mypass \
  tickstorm:latest

# With TLS support
docker run -p 8080:8080 \
  -v /path/to/certs:/certs \
  -e TLS_ENABLED=true \
  -e TLS_CERT_FILE=/certs/server.crt \
  -e TLS_KEY_FILE=/certs/server.key \
  tickstorm:latest
```

### Docker Compose
```yaml
version: '3.8'
services:
  tickstorm:
    image: tickstorm:latest
    ports:
      - "8080:8080"
    environment:
      - AUTH_USERNAME=admin
      - AUTH_PASSWORD=secure_password
      - MAX_CONNECTIONS=50000
    restart: unless-stopped
    healthcheck:
      test: ["/tickstorm", "-health-check"]
      interval: 30s
      timeout: 10s
      retries: 3
```

## ☸️ Kubernetes Deployment

### Basic Deployment
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tickstorm
spec:
  replicas: 3
  selector:
    matchLabels:
      app: tickstorm
  template:
    metadata:
      labels:
        app: tickstorm
    spec:
      containers:
      - name: tickstorm
        image: tickstorm:latest
        ports:
        - containerPort: 8080
        env:
        - name: AUTH_USERNAME
          valueFrom:
            secretKeyRef:
              name: tickstorm-auth
              key: username
        - name: AUTH_PASSWORD
          valueFrom:
            secretKeyRef:
              name: tickstorm-auth
              key: password
        resources:
          limits:
            memory: "1Gi"
            cpu: "2"
          requests:
            memory: "512Mi"
            cpu: "1"
        livenessProbe:
          exec:
            command: ["/tickstorm", "-health-check"]
          initialDelaySeconds: 30
          periodSeconds: 30
        readinessProbe:
          exec:
            command: ["/tickstorm", "-health-check"]
          initialDelaySeconds: 5
          periodSeconds: 10
```

## 🔒 Security Best Practices

### TLS Configuration
- Use TLS 1.3 for all production deployments
- Implement mutual TLS (mTLS) for client authentication
- Regularly rotate certificates
- Use strong cipher suites and key lengths

### Container Security
- Run containers as non-root user
- Use distroless base images
- Enable read-only filesystem
- Implement proper resource limits

### Network Security
- Use network policies to restrict traffic
- Implement rate limiting at load balancer level
- Monitor for suspicious connection patterns
- Use secure credential storage (Kubernetes secrets, etc.)

## 📊 Monitoring

### Health Checks
```bash
# Container health check
./tickstorm -health-check

# Manual health check
curl http://localhost:8080/health
```

### Metrics
TickStorm exposes Prometheus-compatible metrics:
- `tickstorm_active_connections`: Current active connections
- `tickstorm_messages_sent_total`: Total messages sent
- `tickstorm_bytes_sent_total`: Total bytes transmitted
- `tickstorm_heartbeat_timeouts_total`: Heartbeat timeouts
- `tickstorm_tls_handshakes_total`: TLS handshake metrics

### Logging
Structured JSON logging with configurable levels:
```bash
# Set log level
export LOG_LEVEL=debug
./tickstorm
```

## 🧪 Testing

### Unit Tests
```bash
go test ./...
```

### Integration Tests
```bash
go test -tags=integration ./...
```

### Performance Tests
```bash
go test -bench=. ./internal/server/
```

### Load Testing
```bash
# Build test client
go build -o test-client ./cmd/test-client

# Run load test
./test-client -connections=1000 -duration=60s
```

## 📚 Documentation

- [Architecture Design](docs/ARCHITECTURE.md)
- [Protocol Specification](docs/PROTOCOL.md)
- [Version Migration Guide](docs/VERSION_MIGRATION.md)
- [Brand Guidelines](docs/BRAND_GUIDELINES.md)
- [Contributing Guide](CONTRIBUTING.md)

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Development Setup
```bash
# Clone repository
git clone https://github.com/furkansarikaya/tick-storm.git
cd tick-storm

# Install dependencies
go mod download

# Run tests
make test

# Build binary
make build
```

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🏆 Performance Benchmarks

| Metric | Value |
|--------|-------|
| Latency (p50) | < 1ms |
| Latency (p95) | < 5ms |
| Latency (p99) | < 10ms |
| Throughput | 1M+ messages/sec |
| Concurrent Connections | 100k+ |
| Memory Usage | < 1GB per 100k connections |
| CPU Usage | < 70% at peak load |

## 🔧 Troubleshooting

### Common Issues

#### Connection Refused
```bash
# Check if server is running
netstat -tlnp | grep 8080

# Check Docker container status
docker ps
docker logs tickstorm-server
```

#### High Memory Usage
```bash
# Check connection count
curl http://localhost:8080/stats

# Monitor memory usage
docker stats tickstorm-server
```

#### TLS Handshake Failures
```bash
# Verify certificate validity
openssl x509 -in server.crt -text -noout

# Test TLS connection
openssl s_client -connect localhost:8080
```

---

**TickStorm** - Built for performance, designed for scale.
