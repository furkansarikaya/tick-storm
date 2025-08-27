# High Memory Usage Alert Runbook

## Alert: TickStormMemoryUsageHigh

**Severity**: High  
**Threshold**: Memory usage > 800MB  
**SLA Impact**: Medium - may affect performance and stability

## Alert Description

This alert fires when TickStorm memory usage exceeds 800MB. High memory usage can lead to performance degradation, increased garbage collection overhead, and potential out-of-memory conditions.

## Immediate Actions (0-5 minutes)

1. **Check Current Memory Usage**
   ```bash
   curl -s http://localhost:9090/metrics | grep memory_usage
   ps -p $(pgrep tickstorm) -o pid,vsz,rss,pmem
   ```

2. **Verify System Memory**
   ```bash
   free -h
   cat /proc/meminfo | grep -E "(MemTotal|MemAvailable|MemFree)"
   ```

## Diagnosis (5-15 minutes)

### 1. Memory Breakdown Analysis
```bash
# Go runtime memory stats
curl -s http://localhost:9090/metrics | grep -E "(go_memstats|tick_storm_memory)"

# Process memory details
cat /proc/$(pgrep tickstorm)/status | grep -E "(VmSize|VmRSS|VmData|VmStk)"
```

### 2. Connection and Goroutine Analysis
```promql
# Active connections vs memory
tick_storm_active_connections
tick_storm_memory_usage_bytes

# Goroutine count
tick_storm_goroutines
```

### 3. Garbage Collection Metrics
```bash
# GC statistics
curl -s http://localhost:9090/metrics | grep go_gc
```

## Resolution Steps

### Scenario 1: Memory Leak
```bash
# Force garbage collection (temporary)
kill -USR1 $(pgrep tickstorm)

# Restart service if leak suspected
kubectl rollout restart deployment/tickstorm
```

### Scenario 2: High Connection Load
```bash
# Scale horizontally
kubectl scale deployment tickstorm --replicas=3

# Increase memory limits
kubectl patch deployment tickstorm -p '{"spec":{"template":{"spec":{"containers":[{"name":"tickstorm","resources":{"limits":{"memory":"2Gi"}}}]}}}}'
```

### Scenario 3: GC Tuning
```bash
# Optimize garbage collection
export GOGC=75
export GOMEMLIMIT=1500MiB
kubectl rollout restart deployment/tickstorm
```

## Verification

```promql
# Verify memory usage is decreasing
tick_storm_memory_usage_bytes / (1024*1024*1024) < 0.8
```

## Prevention

1. **Memory Monitoring**
   - Set up memory trend analysis
   - Monitor memory per connection ratio
   - Track GC frequency and duration

2. **Optimization**
   ```bash
   # Connection pooling
   export CONNECTION_POOL_SIZE=1000
   export BUFFER_POOL_SIZE=10000
   
   # Memory limits
   export MAX_MESSAGE_SIZE=1MB
   export MAX_BUFFER_SIZE=64KB
   ```

## Escalation

**Escalate if:**
- Memory usage continues growing after restart
- OOM kills detected
- Performance severely degraded
