# Resource Constraints and Monitoring

This document describes the resource constraint management and monitoring system in Tick-Storm TCP server, including OS-level limits, graceful degradation, and monitoring capabilities.

## Overview

The resource constraint system provides comprehensive protection against resource exhaustion through:

- **OS-level resource limits** (ulimit configuration)
- **Real-time resource monitoring** with alerting
- **Graceful degradation** when limits are exceeded
- **Automatic connection rejection** during resource breaches
- **Comprehensive metrics** and logging

## Components

### 1. ResourceConstraints

Manages OS-level resource limits using ulimit syscalls.

**Features:**
- File descriptor limits
- Memory limits
- CPU time limits
- Stack size limits
- Core dump size limits
- Go runtime parameter tuning

### 2. ResourceMonitor

Provides real-time monitoring of resource usage with configurable thresholds.

**Monitored Resources:**
- Memory usage (MB and percentage)
- File descriptor usage
- Goroutine count
- Active connections
- CPU usage (planned)

### 3. ResourceBreachHandler

Implements graceful degradation when resource limits are exceeded.

**Actions:**
- Reject new connections with proper error messages
- Trigger garbage collection for memory pressure
- Log breach events with detailed context
- Provide breach statistics and metrics

## Configuration

### Environment Variables

#### OS-Level Resource Limits

```bash
# File descriptor limits
export ULIMIT_MAX_OPEN_FILES=65536

# Memory limits (in bytes)
export ULIMIT_MAX_MEMORY_SIZE=1073741824  # 1GB

# CPU time limit (in seconds)
export ULIMIT_MAX_CPU_TIME=3600  # 1 hour

# Stack size limit (in bytes)
export ULIMIT_MAX_STACK_SIZE=8388608  # 8MB

# Core dump size limit (in bytes)
export ULIMIT_MAX_CORE_SIZE=0  # Disable core dumps
```

#### Go Runtime Parameters

```bash
# Maximum number of OS threads
export GOMAXPROCS=8

# Garbage collection target percentage
export GOGC=100

# Memory limit for Go runtime (Go 1.19+)
export GOMEMLIMIT=1GB
```

#### Resource Monitoring Thresholds

```bash
# Resource limits (set programmatically)
# MaxMemoryMB: 1024 (1GB)
# MaxFileDescriptors: 65536
# MaxGoroutines: 50000
# MaxConnections: 100000

# Alert thresholds
# WarningThreshold: 0.8 (80%)
# CriticalThreshold: 0.9 (90%)
```

## Usage Examples

### Basic Configuration

```go
// Initialize resource management in server
limits := ResourceLimits{
    MaxMemoryMB:       1024,  // 1GB
    MaxFileDescriptors: 65536, // 64K FDs
    MaxGoroutines:     50000,  // 50K goroutines
    MaxConnections:    100000, // 100K connections
    WarningThreshold:  0.8,    // 80% warning
    CriticalThreshold: 0.9,    // 90% critical
}

resourceMonitor := NewResourceMonitor(limits)
resourceConstraints := NewResourceConstraints()
breachHandler := NewResourceBreachHandler(logger, resourceMonitor)
```

### Applying Resource Constraints

```go
// Apply OS-level resource limits
if err := resourceConstraints.ApplyResourceLimits(); err != nil {
    log.Fatalf("Failed to apply resource limits: %v", err)
}

// Set Go runtime limits
resourceConstraints.SetGoRuntimeLimits()

// Log current limits
resourceConstraints.LogResourceLimits()
```

### Starting Monitoring

```go
// Start resource monitoring
resourceMonitor.Start()

// Start breach monitoring
go breachHandler.StartMonitoring(ctx)
```

## Graceful Degradation

When resource limits are exceeded, the system implements graceful degradation:

### Memory Breach (>90% usage)
- Enable connection rejection
- Trigger garbage collection
- Log memory pressure events
- Clear breach when usage drops below 80%

### File Descriptor Breach (>90% usage)
- Reject new connections
- Log FD exhaustion warnings
- Monitor FD usage recovery

### Goroutine Breach (>50,000 goroutines)
- Reject new connections
- Log goroutine count alerts
- Monitor goroutine cleanup

### Connection Breach (>95,000 connections)
- Reject new connections immediately
- Send proper error responses
- Log connection limit events

## Error Responses

When connections are rejected due to resource constraints:

```
503 Service Unavailable: server memory limit exceeded
503 Service Unavailable: server file descriptor limit exceeded
503 Service Unavailable: server connection limit exceeded
503 Service Unavailable: server goroutine limit exceeded
```

## Monitoring and Metrics

### Server Statistics

The server exposes resource metrics via `GetStats()`:

```json
{
  "resource_memory_breach": false,
  "resource_fd_breach": false,
  "resource_goroutine_breach": false,
  "resource_connection_breach": false,
  "resource_rejecting_connections": false,
  "resource_connections_rejected": 0,
  "resource_degradation_events": 0
}
```

### Resource Usage

Current resource usage is available through the monitor:

```go
usage := resourceMonitor.GetCurrentUsage()
fmt.Printf("Memory: %.1f%%, FDs: %.1f%%, Goroutines: %d, Connections: %d",
    usage.MemoryUsagePercent,
    usage.FDUsagePercent,
    usage.GoroutineCount,
    usage.ActiveConnections)
```

### Alerts and Logging

Resource alerts are logged with structured data:

```json
{
  "level": "ERROR",
  "msg": "memory usage breach detected - enabling graceful degradation",
  "memory_usage_percent": 92.5,
  "action": "rejecting_new_connections",
  "component": "resource_breach_handler"
}
```

## Best Practices

### Production Deployment

1. **Set Conservative Limits**
   ```bash
   export ULIMIT_MAX_OPEN_FILES=65536
   export ULIMIT_MAX_MEMORY_SIZE=2147483648  # 2GB
   export GOMEMLIMIT=1800MB  # Leave headroom
   ```

2. **Monitor Resource Usage**
   - Set up alerts for warning thresholds (80%)
   - Monitor breach events and degradation
   - Track connection rejection rates

3. **Tune for Workload**
   - Adjust limits based on expected load
   - Monitor actual resource usage patterns
   - Set appropriate alert thresholds

### Container Environments

When running in containers, ensure container limits align with application limits:

```yaml
# Docker Compose
services:
  tick-storm:
    deploy:
      resources:
        limits:
          memory: 2G
          cpus: '2.0'
        reservations:
          memory: 1G
          cpus: '1.0'
```

```yaml
# Kubernetes
resources:
  limits:
    memory: "2Gi"
    cpu: "2000m"
  requests:
    memory: "1Gi"
    cpu: "1000m"
```

### Monitoring Integration

Integrate with external monitoring systems:

```go
// Custom alert handler
type PrometheusAlertHandler struct {
    registry *prometheus.Registry
}

func (h *PrometheusAlertHandler) HandleResourceAlert(alert ResourceAlert) {
    // Send metrics to Prometheus
    resourceUsageGauge.WithLabelValues(alert.Type).Set(alert.Usage)
    resourceAlertsCounter.WithLabelValues(alert.Type, alert.Level.String()).Inc()
}

// Register with resource monitor
resourceMonitor.AddAlertHandler(&PrometheusAlertHandler{})
```

## Troubleshooting

### High Memory Usage

1. Check for memory leaks in connection handlers
2. Verify garbage collection is running effectively
3. Monitor goroutine count for runaway goroutines
4. Adjust `GOGC` and `GOMEMLIMIT` parameters

### File Descriptor Exhaustion

1. Verify proper connection cleanup
2. Check for leaked file handles
3. Increase `ULIMIT_MAX_OPEN_FILES` if needed
4. Monitor connection lifecycle

### Connection Rejections

1. Check resource breach logs
2. Verify resource usage patterns
3. Adjust resource limits if appropriate
4. Implement connection pooling on client side

### Performance Impact

Resource monitoring has minimal performance impact:
- Monitoring runs every 5 seconds
- Atomic operations for counters
- Efficient resource usage calculations
- Configurable alert thresholds

## Security Considerations

- Resource limits prevent denial-of-service attacks
- Graceful degradation maintains service availability
- Proper error responses don't leak system information
- Monitoring helps detect resource-based attacks

## Future Enhancements

- CPU usage monitoring and limits
- Network bandwidth monitoring
- Disk I/O monitoring
- Dynamic limit adjustment based on load
- Integration with container orchestration systems
- Advanced alerting with external systems
