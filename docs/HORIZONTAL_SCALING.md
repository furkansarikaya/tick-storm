# Horizontal Scaling Guide for Tick-Storm

This document provides comprehensive guidance for horizontally scaling Tick-Storm TCP server instances to handle high-volume tick data streaming with load balancing, auto-scaling, and high availability.

## Overview

Tick-Storm is designed as a stateless, horizontally scalable TCP server that can be deployed behind Layer 4 (L4) TCP load balancers. Each instance maintains independent connections and does not share state, making it ideal for horizontal scaling architectures.

## Architecture Principles

### Stateless Design
- **No Shared State**: Each instance operates independently
- **Connection Isolation**: Connections are bound to specific instances
- **Independent Authentication**: Each instance validates credentials locally
- **Autonomous Operation**: Instances can start/stop without affecting others

### Load Distribution
- **Connection-Based**: Load balancers distribute connections across instances
- **Round-Robin**: Default algorithm for even distribution
- **Health-Based**: Unhealthy instances are removed from rotation
- **Capacity-Aware**: Instances report capacity for intelligent routing

## Deployment Strategies

### Kubernetes Deployment

#### Basic Deployment Configuration

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tick-storm
  namespace: tick-storm
spec:
  replicas: 3
  selector:
    matchLabels:
      app: tick-storm
  template:
    metadata:
      labels:
        app: tick-storm
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "9090"
        prometheus.io/path: "/metrics"
    spec:
      containers:
      - name: tick-storm
        image: tick-storm:latest
        ports:
        - containerPort: 8080
          name: tcp-server
        - containerPort: 8081
          name: health
        - containerPort: 9090
          name: metrics
        env:
        - name: INSTANCE_ID
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: AUTOSCALING_ENABLED
          value: "true"
        - name: AUTOSCALING_METRICS_PORT
          value: "9090"
        resources:
          requests:
            memory: "512Mi"
            cpu: "500m"
          limits:
            memory: "1Gi"
            cpu: "1000m"
        livenessProbe:
          httpGet:
            path: /ping
            port: 8081
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8081
          initialDelaySeconds: 5
          periodSeconds: 5
```

#### Horizontal Pod Autoscaler (HPA)

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: tick-storm-hpa
  namespace: tick-storm
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: tick-storm
  minReplicas: 2
  maxReplicas: 20
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
  - type: Pods
    pods:
      metric:
        name: tick_storm_active_connections
      target:
        type: AverageValue
        averageValue: "80000"
  behavior:
    scaleUp:
      stabilizationWindowSeconds: 60
      policies:
      - type: Percent
        value: 50
        periodSeconds: 60
    scaleDown:
      stabilizationWindowSeconds: 300
      policies:
      - type: Percent
        value: 10
        periodSeconds: 60
```

### Docker Swarm Deployment

```yaml
version: '3.8'
services:
  tick-storm:
    image: tick-storm:latest
    deploy:
      replicas: 3
      update_config:
        parallelism: 1
        delay: 30s
        order: start-first
      restart_policy:
        condition: on-failure
        delay: 5s
        max_attempts: 3
      resources:
        limits:
          memory: 1G
          cpus: '1.0'
        reservations:
          memory: 512M
          cpus: '0.5'
    ports:
      - "8080:8080"
      - "8081:8081"
    environment:
      - AUTOSCALING_ENABLED=true
      - AUTOSCALING_CONNECTIONS_PER_INSTANCE=80000
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8081/ping"]
      interval: 30s
      timeout: 10s
      retries: 3
    networks:
      - tick-storm-network

networks:
  tick-storm-network:
    driver: overlay
```

## Auto-Scaling Configuration

### Environment Variables

```bash
# Auto-scaling configuration
AUTOSCALING_ENABLED=true
AUTOSCALING_METRICS_PORT=9090
AUTOSCALING_SCALE_UP_THRESHOLD=0.8
AUTOSCALING_SCALE_DOWN_THRESHOLD=0.3
AUTOSCALING_CONNECTIONS_PER_INSTANCE=80000
AUTOSCALING_CPU_TARGET=70
AUTOSCALING_MEMORY_TARGET=80

# Instance identification
INSTANCE_ID=auto-generated-or-custom
APP_VERSION=1.0.0

# Resource limits
ULIMIT_MAX_OPEN_FILES=100000
ULIMIT_MAX_MEMORY_SIZE=1073741824  # 1GB
GOMAXPROCS=4
GOMEMLIMIT=1GiB
```

### Custom Metrics for HPA

Tick-Storm exposes custom metrics for advanced auto-scaling:

```bash
# Connection utilization metric
curl http://tick-storm:9090/metrics | grep tick_storm_active_connections

# Auto-scaling recommendations
curl http://tick-storm:9090/autoscaling/recommendations
```

### Prometheus Integration

```yaml
# prometheus-config.yaml
global:
  scrape_interval: 15s

scrape_configs:
- job_name: 'tick-storm'
  static_configs:
  - targets: ['tick-storm:9090']
  metrics_path: /metrics
  scrape_interval: 10s

- job_name: 'tick-storm-autoscaling'
  static_configs:
  - targets: ['tick-storm:9090']
  metrics_path: /autoscaling/metrics
  scrape_interval: 30s
```

## Load Balancer Integration

### Service Configuration

```yaml
apiVersion: v1
kind: Service
metadata:
  name: tick-storm-lb
  annotations:
    service.beta.kubernetes.io/aws-load-balancer-type: "nlb"
    service.beta.kubernetes.io/aws-load-balancer-cross-zone-load-balancing-enabled: "true"
spec:
  type: LoadBalancer
  ports:
  - port: 8080
    targetPort: 8080
    protocol: TCP
    name: tcp-server
  selector:
    app: tick-storm
  sessionAffinity: None  # Stateless design doesn't need session affinity
```

### Health Check Configuration

```yaml
apiVersion: v1
kind: Service
metadata:
  name: tick-storm-health
spec:
  type: ClusterIP
  ports:
  - port: 8081
    targetPort: 8081
    protocol: TCP
    name: health
  selector:
    app: tick-storm
```

## Scaling Strategies

### Vertical vs Horizontal Scaling

#### Horizontal Scaling (Recommended)
- **Pros**: Better fault tolerance, easier to manage, cost-effective
- **Cons**: More complex networking, load balancer dependency
- **Use Case**: High connection volumes, geographic distribution

#### Vertical Scaling
- **Pros**: Simpler architecture, no load balancer needed
- **Cons**: Single point of failure, resource limits, higher costs
- **Use Case**: Moderate loads, development environments

### Scaling Triggers

#### Connection-Based Scaling
```yaml
# Scale up when average connections > 80,000 per instance
# Scale down when average connections < 30,000 per instance
metrics:
- type: Pods
  pods:
    metric:
      name: tick_storm_active_connections
    target:
      type: AverageValue
      averageValue: "80000"
```

#### Resource-Based Scaling
```yaml
# Scale based on CPU and memory utilization
metrics:
- type: Resource
  resource:
    name: cpu
    target:
      type: Utilization
      averageUtilization: 70
- type: Resource
  resource:
    name: memory
    target:
      type: Utilization
      averageUtilization: 80
```

#### Custom Metrics Scaling
```yaml
# Scale based on error rate or response time
- type: External
  external:
    metric:
      name: tick_storm_error_rate
    target:
      type: Value
      value: "0.05"  # 5% error rate threshold
```

## Instance Management

### Instance Identification

Each instance generates a unique ID for tracking and debugging:

```go
// Environment-based ID (recommended for containers)
INSTANCE_ID=tick-storm-deployment-abc123-xyz789

// Auto-generated ID (fallback)
instanceID := generateInstanceID() // Returns hex-encoded random ID
```

### Instance Metrics

```json
{
  "instance_id": "tick-storm-abc123",
  "uptime_seconds": 3600,
  "active_connections": 75000,
  "total_connections": 1500000,
  "memory_usage_mb": 768,
  "cpu_usage_percent": 65,
  "version": "1.0.0",
  "platform": "linux/amd64"
}
```

### Health Status Reporting

```json
{
  "status": "healthy",
  "instance_id": "tick-storm-abc123",
  "checks": {
    "server": "healthy",
    "authentication": "healthy",
    "resources": "healthy",
    "connections": "healthy"
  },
  "metrics": {
    "active_connections": 75000,
    "connection_utilization": 0.75,
    "memory_utilization": 0.75,
    "breach_status": "none"
  }
}
```

## Deployment Patterns

### Blue-Green Deployment

1. **Deploy Green Environment**
   ```bash
   kubectl apply -f tick-storm-green-deployment.yaml
   kubectl wait --for=condition=ready pod -l app=tick-storm,version=green
   ```

2. **Switch Traffic**
   ```bash
   kubectl patch service tick-storm-lb -p '{"spec":{"selector":{"version":"green"}}}'
   ```

3. **Monitor and Rollback if Needed**
   ```bash
   kubectl patch service tick-storm-lb -p '{"spec":{"selector":{"version":"blue"}}}'
   ```

### Rolling Updates

```yaml
spec:
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 1
      maxSurge: 1
  template:
    spec:
      terminationGracePeriodSeconds: 60  # Allow graceful shutdown
```

### Canary Deployment

```yaml
# Canary deployment with 10% traffic
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: tick-storm-rollout
spec:
  replicas: 10
  strategy:
    canary:
      steps:
      - setWeight: 10
      - pause: {duration: 5m}
      - setWeight: 50
      - pause: {duration: 10m}
      - setWeight: 100
  selector:
    matchLabels:
      app: tick-storm
```

## Monitoring and Observability

### Key Metrics to Monitor

#### Instance-Level Metrics
- **Active Connections**: Current connection count per instance
- **Connection Rate**: New connections per second
- **Memory Usage**: Heap and system memory consumption
- **CPU Usage**: Processor utilization percentage
- **Error Rate**: Authentication and protocol errors

#### Cluster-Level Metrics
- **Total Instances**: Number of running instances
- **Total Connections**: Aggregate connection count
- **Load Distribution**: Connection distribution across instances
- **Scaling Events**: Auto-scaling up/down events
- **Health Status**: Overall cluster health

### Alerting Rules

```yaml
# Prometheus alerting rules
groups:
- name: tick-storm-scaling
  rules:
  - alert: HighConnectionUtilization
    expr: tick_storm_active_connections / 80000 > 0.9
    for: 2m
    labels:
      severity: warning
    annotations:
      summary: "High connection utilization on {{ $labels.instance }}"
      
  - alert: ScalingEventFailed
    expr: increase(kube_hpa_status_condition{condition="ScalingLimited"}[5m]) > 0
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "Auto-scaling event failed for tick-storm"
```

### Grafana Dashboard

```json
{
  "dashboard": {
    "title": "Tick-Storm Horizontal Scaling",
    "panels": [
      {
        "title": "Active Connections per Instance",
        "type": "graph",
        "targets": [
          {
            "expr": "tick_storm_active_connections",
            "legendFormat": "{{ instance_id }}"
          }
        ]
      },
      {
        "title": "Auto-scaling Events",
        "type": "graph",
        "targets": [
          {
            "expr": "kube_hpa_status_current_replicas{hpa=\"tick-storm-hpa\"}",
            "legendFormat": "Current Replicas"
          }
        ]
      }
    ]
  }
}
```

## Performance Optimization

### Connection Distribution

#### Load Balancer Configuration
```yaml
# Even distribution across instances
spec:
  sessionAffinity: None
  sessionAffinityConfig: null
  
# Connection-based load balancing
metadata:
  annotations:
    service.beta.kubernetes.io/aws-load-balancer-backend-protocol: "tcp"
    service.beta.kubernetes.io/aws-load-balancer-connection-draining-enabled: "true"
    service.beta.kubernetes.io/aws-load-balancer-connection-draining-timeout: "60"
```

#### Instance Capacity Planning
```bash
# Calculate optimal instance count
target_connections=500000
connections_per_instance=80000
min_instances=$((target_connections / connections_per_instance))
recommended_instances=$((min_instances + 2))  # Add buffer
```

### Resource Optimization

#### Memory Management
```yaml
env:
- name: GOMEMLIMIT
  value: "768MiB"  # 75% of container limit
- name: GOGC
  value: "100"     # Default GC target
resources:
  limits:
    memory: "1Gi"
  requests:
    memory: "512Mi"
```

#### CPU Optimization
```yaml
env:
- name: GOMAXPROCS
  valueFrom:
    resourceFieldRef:
      resource: limits.cpu
resources:
  limits:
    cpu: "1000m"
  requests:
    cpu: "500m"
```

## Troubleshooting

### Common Scaling Issues

#### Slow Scale-Up
```bash
# Check HPA status
kubectl describe hpa tick-storm-hpa

# Check resource availability
kubectl top nodes
kubectl describe nodes

# Check pod scheduling
kubectl get events --sort-by=.metadata.creationTimestamp
```

#### Uneven Load Distribution
```bash
# Check connection distribution
kubectl exec -it tick-storm-pod -- curl localhost:8081/health | jq .active_connections

# Verify load balancer configuration
kubectl describe service tick-storm-lb

# Check instance health
kubectl get pods -l app=tick-storm -o wide
```

#### Resource Constraints
```bash
# Check resource usage
kubectl top pods -l app=tick-storm

# Check resource limits
kubectl describe pods -l app=tick-storm | grep -A 5 "Limits:"

# Check cluster capacity
kubectl describe nodes | grep -A 5 "Allocated resources:"
```

### Debug Commands

```bash
# Instance health check
curl http://tick-storm-instance:8081/health | jq .

# Auto-scaling metrics
curl http://tick-storm-instance:9090/autoscaling/metrics | jq .

# Scale recommendations
curl http://tick-storm-instance:9090/autoscaling/recommendations | jq .

# Connection distribution
kubectl get pods -l app=tick-storm -o json | jq '.items[] | {name: .metadata.name, connections: .status.containerStatuses[0].ready}'
```

## Best Practices

### Deployment
1. **Always use readiness probes** for proper load balancer integration
2. **Set appropriate resource limits** to prevent resource starvation
3. **Use rolling updates** with proper termination grace periods
4. **Monitor scaling events** and adjust thresholds based on traffic patterns

### Scaling
1. **Start with conservative thresholds** and adjust based on observed behavior
2. **Use multiple metrics** for scaling decisions (CPU, memory, connections)
3. **Set minimum replicas** to handle baseline load
4. **Configure scale-down delays** to prevent thrashing

### Operations
1. **Monitor connection distribution** across instances
2. **Set up alerting** for scaling events and failures
3. **Regular load testing** to validate scaling behavior
4. **Document scaling policies** and thresholds for operational teams

This horizontal scaling guide provides a comprehensive foundation for deploying and managing Tick-Storm at scale with high availability, performance, and operational excellence.
