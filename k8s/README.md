# Kubernetes Deployment Guide

This directory contains Kubernetes manifests for deploying Tick-Storm TCP server with comprehensive resource constraints and monitoring.

## Files Overview

- `namespace.yaml` - Namespace, ResourceQuota, and LimitRange
- `rbac.yaml` - ServiceAccount, Role, and RoleBinding
- `configmap.yaml` - Configuration data
- `secrets.yaml` - Authentication credentials and TLS certificates
- `deployment.yaml` - Main application deployment with resource limits
- `service.yaml` - LoadBalancer and headless services
- `hpa.yaml` - Horizontal Pod Autoscaler and Pod Disruption Budget

## Resource Configuration

### Container Resource Limits

```yaml
resources:
  limits:
    memory: "2Gi"      # Maximum memory usage
    cpu: "2000m"       # Maximum CPU (2 cores)
    ephemeral-storage: "1Gi"
  requests:
    memory: "1Gi"      # Guaranteed memory
    cpu: "500m"        # Guaranteed CPU (0.5 cores)
    ephemeral-storage: "500Mi"
```

### Application-Level Resource Constraints

Environment variables configure OS-level and Go runtime limits:

```yaml
env:
# OS-level resource limits
- name: ULIMIT_MAX_OPEN_FILES
  value: "65536"
- name: ULIMIT_MAX_MEMORY_SIZE
  value: "1610612736"  # 1.5GB
- name: ULIMIT_MAX_CPU_TIME
  value: "7200"        # 2 hours
- name: ULIMIT_MAX_STACK_SIZE
  value: "8388608"     # 8MB

# Go runtime tuning
- name: GOMAXPROCS
  value: "2"
- name: GOGC
  value: "100"
- name: GOMEMLIMIT
  value: "1400MB"      # Leave headroom
```

### Namespace-Level Resource Quotas

```yaml
spec:
  hard:
    requests.cpu: "10"
    requests.memory: 20Gi
    limits.cpu: "20"
    limits.memory: 40Gi
    pods: "10"
```

## Deployment Instructions

### 1. Create Namespace and RBAC

```bash
kubectl apply -f namespace.yaml
kubectl apply -f rbac.yaml
```

### 2. Configure Secrets and ConfigMaps

```bash
# Update credentials in secrets.yaml before applying
kubectl apply -f secrets.yaml
kubectl apply -f configmap.yaml
```

### 3. Deploy Application

```bash
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml
```

### 4. Enable Auto-scaling (Optional)

```bash
kubectl apply -f hpa.yaml
```

### 5. Verify Deployment

```bash
# Check pod status
kubectl get pods -n tick-storm

# Check resource usage
kubectl top pods -n tick-storm

# Check service endpoints
kubectl get svc -n tick-storm

# View logs
kubectl logs -f deployment/tick-storm -n tick-storm
```

## Monitoring and Observability

### Health Checks

The deployment includes comprehensive health checks:

- **Startup Probe**: Ensures container starts successfully
- **Readiness Probe**: Determines when pod is ready for traffic
- **Liveness Probe**: Detects and restarts unhealthy containers

### Metrics Endpoint

Prometheus metrics are exposed on port 9090:

```bash
kubectl port-forward svc/tick-storm-service 9090:9090 -n tick-storm
curl http://localhost:9090/metrics
```

### Resource Monitoring

Monitor resource usage and breach events:

```bash
# Resource usage
kubectl top pods -n tick-storm --containers

# Resource events
kubectl get events -n tick-storm --sort-by='.lastTimestamp'

# Pod resource limits
kubectl describe pod -l app=tick-storm -n tick-storm
```

## Scaling and Performance

### Horizontal Pod Autoscaler

The HPA automatically scales based on:
- CPU utilization (target: 70%)
- Memory utilization (target: 80%)

```bash
# Check HPA status
kubectl get hpa -n tick-storm

# View scaling events
kubectl describe hpa tick-storm-hpa -n tick-storm
```

### Manual Scaling

```bash
# Scale to specific replica count
kubectl scale deployment tick-storm --replicas=5 -n tick-storm

# Check scaling status
kubectl rollout status deployment/tick-storm -n tick-storm
```

## Security Configuration

### Pod Security Context

```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 65532
  runAsGroup: 65532
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  capabilities:
    drop:
    - ALL
```

### Network Policies (Optional)

Create network policies to restrict traffic:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: tick-storm-netpol
  namespace: tick-storm
spec:
  podSelector:
    matchLabels:
      app: tick-storm
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: monitoring
    ports:
    - protocol: TCP
      port: 9090
  - ports:
    - protocol: TCP
      port: 8080
```

## Troubleshooting

### Resource Constraint Issues

1. **Pod OOMKilled**
   ```bash
   # Check memory limits
   kubectl describe pod -l app=tick-storm -n tick-storm
   
   # Increase memory limits in deployment.yaml
   # Update GOMEMLIMIT environment variable accordingly
   ```

2. **CPU Throttling**
   ```bash
   # Check CPU metrics
   kubectl top pods -n tick-storm
   
   # Adjust CPU limits and GOMAXPROCS
   ```

3. **File Descriptor Limits**
   ```bash
   # Check container logs for FD errors
   kubectl logs -l app=tick-storm -n tick-storm | grep "file descriptor"
   
   # Increase ULIMIT_MAX_OPEN_FILES if needed
   ```

### Connection Issues

1. **Service Not Accessible**
   ```bash
   # Check service endpoints
   kubectl get endpoints -n tick-storm
   
   # Test connectivity
   kubectl run test-pod --image=busybox -it --rm -- nc -zv tick-storm-service.tick-storm.svc.cluster.local 8080
   ```

2. **Load Balancer Issues**
   ```bash
   # Check load balancer status
   kubectl describe svc tick-storm-service -n tick-storm
   
   # Check cloud provider load balancer configuration
   ```

### Performance Issues

1. **High Latency**
   ```bash
   # Check resource usage
   kubectl top pods -n tick-storm
   
   # Review application metrics
   kubectl port-forward svc/tick-storm-service 9090:9090 -n tick-storm
   ```

2. **Connection Rejections**
   ```bash
   # Check application logs for resource breaches
   kubectl logs -l app=tick-storm -n tick-storm | grep "resource breach"
   
   # Review resource constraint configuration
   ```

## Production Considerations

### Resource Planning

1. **Memory Sizing**
   - Container limit: 2Gi
   - Go runtime limit: 1400MB (leaves 600MB headroom)
   - OS-level limit: 1.5GB

2. **CPU Allocation**
   - Container limit: 2 cores
   - GOMAXPROCS: 2 (matches CPU limit)
   - Request: 0.5 cores (guaranteed)

3. **Storage**
   - Ephemeral storage for logs and temporary files
   - Read-only root filesystem for security

### High Availability

1. **Multi-Zone Deployment**
   ```yaml
   affinity:
     podAntiAffinity:
       requiredDuringSchedulingIgnoredDuringExecution:
       - labelSelector:
           matchExpressions:
           - key: app
             operator: In
             values:
             - tick-storm
         topologyKey: topology.kubernetes.io/zone
   ```

2. **Pod Disruption Budget**
   - Minimum 1 pod available during updates
   - Prevents all pods from being terminated simultaneously

### Monitoring Integration

1. **Prometheus Setup**
   ```yaml
   apiVersion: monitoring.coreos.com/v1
   kind: ServiceMonitor
   metadata:
     name: tick-storm
     namespace: tick-storm
   spec:
     selector:
       matchLabels:
         app: tick-storm
     endpoints:
     - port: metrics
       interval: 30s
       path: /metrics
   ```

2. **Grafana Dashboard**
   - Import dashboard for Tick-Storm metrics
   - Monitor resource usage and breach events
   - Set up alerts for critical thresholds

This configuration provides enterprise-grade deployment with comprehensive resource management, security, and observability for the Tick-Storm TCP server.
