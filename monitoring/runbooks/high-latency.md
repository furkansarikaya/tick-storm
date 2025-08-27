# High Latency Alert Runbook

## Alert: TickStormPublishLatencyHigh

**Severity**: High  
**Threshold**: 95th percentile publish latency > 5ms  
**SLA Impact**: High - affects performance SLA

## Alert Description

This alert fires when the 95th percentile publish latency exceeds 5ms. This indicates that message publishing operations are taking longer than expected, potentially impacting client experience and violating performance SLAs.

## Immediate Actions (0-5 minutes)

1. **Check Current Latency**
   ```bash
   curl -s http://localhost:9090/metrics | grep publish_latency
   ```

2. **Verify System Load**
   ```bash
   top -p $(pgrep tickstorm)
   iostat -x 1 5
   ```

3. **Check Active Connections**
   ```promql
   tick_storm_active_connections
   rate(tick_storm_messages_sent_total[1m])
   ```

## Diagnosis (5-15 minutes)

### 1. Resource Bottlenecks
```bash
# CPU usage
top -p $(pgrep tickstorm) -n 1 | grep tickstorm

# Memory usage
ps -p $(pgrep tickstorm) -o pid,vsz,rss,pmem

# Disk I/O
iotop -p $(pgrep tickstorm) -n 3
```

### 2. Network Congestion
```bash
# Network interface statistics
cat /proc/net/dev
ss -tuln | grep :8080

# Check for packet drops
netstat -i
```

### 3. Application Metrics
```promql
# Message processing duration
histogram_quantile(0.95, rate(tick_storm_message_processing_duration_seconds_bucket[5m]))

# Write latency
histogram_quantile(0.95, rate(tick_storm_write_latency_seconds_bucket[5m]))

# Goroutine count
tick_storm_goroutines
```

## Resolution Steps

### Scenario 1: High CPU Usage
```bash
# Check goroutine count
curl -s http://localhost:9090/metrics | grep goroutines

# Scale up resources
kubectl patch deployment tickstorm -p '{"spec":{"template":{"spec":{"containers":[{"name":"tickstorm","resources":{"limits":{"cpu":"2000m"}}}]}}}}'
```

### Scenario 2: Memory Pressure
```bash
# Check memory usage
curl -s http://localhost:9090/metrics | grep memory_usage

# Increase memory limits
kubectl patch deployment tickstorm -p '{"spec":{"template":{"spec":{"containers":[{"name":"tickstorm","resources":{"limits":{"memory":"2Gi"}}}]}}}}'
```

### Scenario 3: Network Bottleneck
```bash
# Check network buffer sizes
sysctl net.core.rmem_max
sysctl net.core.wmem_max

# Optimize TCP settings
echo 'net.core.rmem_max = 134217728' >> /etc/sysctl.conf
echo 'net.core.wmem_max = 134217728' >> /etc/sysctl.conf
sysctl -p
```

## Verification

```promql
# Verify latency improvement
histogram_quantile(0.95, rate(tick_storm_publish_latency_seconds_bucket[5m])) * 1000 < 5
```

## Prevention

1. **Performance Monitoring**
   - Set up latency trend analysis
   - Monitor resource utilization patterns
   - Implement predictive scaling

2. **Optimization**
   ```bash
   # Tune batch processing
   export BATCH_WINDOW=3ms
   export MAX_BATCH_SIZE=200
   
   # Optimize buffer sizes
   export TCP_WRITE_BUFFER_SIZE=131072
   ```

## Escalation

**Escalate if:**
- Latency remains > 5ms after 15 minutes
- Multiple performance metrics are degraded
- Client complaints increase

**Escalation Path:**
1. Notify performance team via Slack
2. Consider emergency scaling
3. Engage infrastructure team if needed
