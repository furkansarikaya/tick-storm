# L4 TCP Load Balancer Configuration for Tick-Storm

This document provides configuration examples and best practices for deploying Tick-Storm behind Layer 4 (L4) TCP load balancers to achieve horizontal scaling and high availability.

## Overview

Tick-Storm is designed to be stateless and horizontally scalable. Each instance maintains its own connections and does not share state with other instances. This makes it ideal for deployment behind L4 TCP load balancers.

## Load Balancer Requirements

### Essential Features
- **Layer 4 (TCP) Load Balancing**: Required for TCP protocol support
- **Session Persistence**: Not required (stateless design)
- **Health Checks**: TCP health checks on port 8081
- **Connection Draining**: Graceful shutdown support during deployments

### Recommended Features
- **Connection Pooling**: Improve performance for high connection volumes
- **SSL Termination**: Optional, can be handled at load balancer or server level
- **Rate Limiting**: Additional protection against DDoS attacks
- **Monitoring**: Connection metrics and health status

## Configuration Examples

### HAProxy Configuration

```haproxy
global
    daemon
    maxconn 100000
    log stdout local0

defaults
    mode tcp
    timeout connect 5000ms
    timeout client 50000ms
    timeout server 50000ms
    option tcplog

# Health check backend for readiness probes
backend tick-storm-health
    mode http
    balance roundrobin
    option httpchk GET /ready
    http-check expect status 200
    server tick-storm-1 10.0.1.10:8081 check
    server tick-storm-2 10.0.1.11:8081 check
    server tick-storm-3 10.0.1.12:8081 check

# Main TCP backend for client connections
backend tick-storm-tcp
    mode tcp
    balance roundrobin
    option tcp-check
    tcp-check connect port 8081
    tcp-check send "GET /ping HTTP/1.0\r\n\r\n"
    tcp-check expect string "pong"
    server tick-storm-1 10.0.1.10:8080 check port 8081
    server tick-storm-2 10.0.1.11:8080 check port 8081
    server tick-storm-3 10.0.1.12:8080 check port 8081

# Frontend for client connections
frontend tick-storm-frontend
    bind *:8080
    mode tcp
    default_backend tick-storm-tcp

# Health check frontend
frontend health-frontend
    bind *:8081
    mode http
    default_backend tick-storm-health
```

### NGINX Stream Configuration

```nginx
# /etc/nginx/nginx.conf
events {
    worker_connections 10000;
}

stream {
    upstream tick_storm_backend {
        least_conn;
        server 10.0.1.10:8080 max_fails=3 fail_timeout=30s;
        server 10.0.1.11:8080 max_fails=3 fail_timeout=30s;
        server 10.0.1.12:8080 max_fails=3 fail_timeout=30s;
    }

    # Health check configuration (requires nginx-plus or custom module)
    upstream tick_storm_health {
        server 10.0.1.10:8081;
        server 10.0.1.11:8081;
        server 10.0.1.12:8081;
    }

    server {
        listen 8080;
        proxy_pass tick_storm_backend;
        proxy_timeout 1s;
        proxy_responses 1;
        proxy_connect_timeout 1s;
        
        # Enable session persistence if needed (not recommended for stateless design)
        # ip_hash;
    }
}

http {
    upstream tick_storm_health_http {
        server 10.0.1.10:8081;
        server 10.0.1.11:8081;
        server 10.0.1.12:8081;
    }

    server {
        listen 8081;
        location /health {
            proxy_pass http://tick_storm_health_http;
            proxy_set_header Host $host;
        }
        
        location /ready {
            proxy_pass http://tick_storm_health_http;
            proxy_set_header Host $host;
        }
    }
}
```

### AWS Application Load Balancer (ALB) + Network Load Balancer (NLB)

```yaml
# ALB for health checks (HTTP)
apiVersion: v1
kind: Service
metadata:
  name: tick-storm-health
  annotations:
    service.beta.kubernetes.io/aws-load-balancer-type: "alb"
    service.beta.kubernetes.io/aws-load-balancer-scheme: "internal"
    alb.ingress.kubernetes.io/healthcheck-path: "/ready"
    alb.ingress.kubernetes.io/healthcheck-port: "8081"
spec:
  type: LoadBalancer
  ports:
  - port: 8081
    targetPort: 8081
    protocol: TCP
  selector:
    app: tick-storm

---
# NLB for TCP traffic
apiVersion: v1
kind: Service
metadata:
  name: tick-storm-tcp
  annotations:
    service.beta.kubernetes.io/aws-load-balancer-type: "nlb"
    service.beta.kubernetes.io/aws-load-balancer-scheme: "internet-facing"
    service.beta.kubernetes.io/aws-load-balancer-cross-zone-load-balancing-enabled: "true"
spec:
  type: LoadBalancer
  ports:
  - port: 8080
    targetPort: 8080
    protocol: TCP
  selector:
    app: tick-storm
```

### Google Cloud Load Balancer

```yaml
# Backend service configuration
apiVersion: compute/v1
kind: BackendService
metadata:
  name: tick-storm-backend
spec:
  protocol: TCP
  loadBalancingScheme: EXTERNAL
  healthChecks:
  - healthCheck: tick-storm-health-check
  backends:
  - group: instance-group-us-central1-a
    balancingMode: CONNECTION
  - group: instance-group-us-central1-b
    balancingMode: CONNECTION

---
# Health check configuration
apiVersion: compute/v1
kind: HealthCheck
metadata:
  name: tick-storm-health-check
spec:
  type: TCP
  tcpHealthCheck:
    port: 8081
    request: "GET /ping HTTP/1.0\r\n\r\n"
    response: "pong"
  checkIntervalSec: 10
  timeoutSec: 5
  healthyThreshold: 2
  unhealthyThreshold: 3
```

## Health Check Configuration

### Health Check Endpoints

Tick-Storm provides multiple health check endpoints on port 8081:

- **`/health`**: Comprehensive health status with detailed metrics
- **`/ready`**: Readiness probe for load balancer integration
- **`/ping`**: Simple liveness check (returns "pong")
- **`/healthz`**: Kubernetes-style health check

### Health Check Examples

```bash
# Simple ping check
curl http://tick-storm:8081/ping
# Response: pong

# Readiness check
curl http://tick-storm:8081/ready
# Response: {"status": "ready", "timestamp": "2024-01-01T00:00:00Z"}

# Detailed health status
curl http://tick-storm:8081/health
# Response: Full health status JSON with metrics
```

### TCP Health Check Script

```bash
#!/bin/bash
# tcp-health-check.sh
HOST=${1:-localhost}
PORT=${2:-8081}

# Simple TCP connection test
timeout 5 bash -c "</dev/tcp/$HOST/$PORT" 2>/dev/null
if [ $? -eq 0 ]; then
    echo "Health check passed"
    exit 0
else
    echo "Health check failed"
    exit 1
fi
```

## Load Balancing Algorithms

### Recommended Algorithms

1. **Round Robin**: Default choice for stateless applications
2. **Least Connections**: Better for varying connection durations
3. **Weighted Round Robin**: For heterogeneous server capacities

### Not Recommended

- **IP Hash/Session Persistence**: Not needed for stateless design
- **Least Response Time**: May not be accurate for persistent connections

## Connection Management

### Connection Limits

Configure load balancer limits based on Tick-Storm capacity:

```yaml
# Per-instance limits
max_connections_per_instance: 100000
connection_timeout: 300s
idle_timeout: 60s

# Load balancer limits
max_total_connections: 500000
connection_rate_limit: 1000/s
```

### Connection Draining

Enable connection draining for graceful deployments:

```yaml
# HAProxy example
server tick-storm-1 10.0.1.10:8080 check disabled
# Wait for connections to drain before stopping instance
```

## Monitoring and Metrics

### Load Balancer Metrics

Monitor these key metrics:

- **Connection Count**: Active connections per instance
- **Connection Rate**: New connections per second
- **Health Check Status**: Instance availability
- **Response Time**: Health check latency
- **Error Rate**: Failed health checks

### Tick-Storm Instance Metrics

Each instance exposes metrics via `/health` endpoint:

```json
{
  "status": "healthy",
  "instance_id": "abc123",
  "active_connections": 50000,
  "total_connections": 1000000,
  "uptime_seconds": 3600,
  "memory_usage_mb": 512,
  "cpu_usage_percent": 45
}
```

## Security Considerations

### Network Security

- **Firewall Rules**: Restrict access to health check ports
- **TLS Termination**: Configure at load balancer or server level
- **Rate Limiting**: Implement at load balancer level
- **DDoS Protection**: Use cloud provider DDoS protection

### Access Control

```yaml
# Example security group rules
ingress_rules:
  - protocol: tcp
    port: 8080
    source: load_balancer_security_group
  - protocol: tcp
    port: 8081
    source: load_balancer_security_group
```

## Deployment Strategies

### Blue-Green Deployment

1. Deploy new version to green environment
2. Update load balancer health checks
3. Gradually shift traffic from blue to green
4. Monitor metrics and rollback if needed

### Rolling Updates

1. Remove one instance from load balancer
2. Update instance with new version
3. Add instance back to load balancer
4. Repeat for all instances

### Canary Deployment

1. Deploy new version to subset of instances
2. Configure weighted routing (e.g., 90% old, 10% new)
3. Monitor metrics and gradually increase new version traffic
4. Complete rollout or rollback based on results

## Troubleshooting

### Common Issues

1. **Health Check Failures**
   - Verify health check endpoint accessibility
   - Check instance resource usage
   - Review application logs

2. **Connection Imbalance**
   - Verify load balancing algorithm
   - Check instance capacity and performance
   - Review connection draining configuration

3. **High Latency**
   - Monitor network connectivity
   - Check load balancer configuration
   - Review instance performance metrics

### Debug Commands

```bash
# Check load balancer status
curl -s http://load-balancer:8081/health | jq .

# Test direct instance connection
telnet tick-storm-instance:8080

# Monitor connection distribution
watch -n 1 'curl -s http://load-balancer:8081/health | jq .active_connections'
```

## Best Practices

### Configuration

1. **Use TCP Health Checks**: More reliable than HTTP for TCP services
2. **Set Appropriate Timeouts**: Balance responsiveness with stability
3. **Enable Connection Draining**: Prevent connection loss during deployments
4. **Monitor All Metrics**: Track both load balancer and instance metrics

### Scaling

1. **Auto-scaling**: Use HPA based on connection count and CPU usage
2. **Capacity Planning**: Monitor peak usage and plan for growth
3. **Geographic Distribution**: Deploy across multiple regions for HA
4. **Load Testing**: Regularly test capacity and failover scenarios

### Operations

1. **Graceful Shutdowns**: Always use proper shutdown procedures
2. **Rolling Updates**: Never update all instances simultaneously
3. **Monitoring Alerts**: Set up alerts for health check failures
4. **Documentation**: Keep load balancer configuration documented

## Performance Tuning

### Load Balancer Tuning

```yaml
# HAProxy performance tuning
global:
  maxconn: 100000
  nbproc: 4
  cpu-map: auto

defaults:
  timeout connect: 1s
  timeout client: 30s
  timeout server: 30s
  option tcp-nodelay
```

### Network Optimization

- **TCP_NODELAY**: Disable Nagle algorithm for low latency
- **Connection Pooling**: Reuse connections when possible
- **Buffer Sizes**: Tune based on message sizes
- **Keep-Alive**: Configure appropriate timeouts

This configuration guide provides a comprehensive foundation for deploying Tick-Storm behind L4 TCP load balancers with high availability, scalability, and performance.
