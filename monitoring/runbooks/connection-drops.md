# Connection Drop Alert Runbook

## Alert: TickStormConnectionDropRate

**Severity**: High  
**Threshold**: Connection drop rate > 0.5%  
**SLA Impact**: High - affects service availability

## Alert Description

This alert fires when the connection drop rate exceeds 0.5% of total connections. Connection drops indicate network issues, resource constraints, or client-side problems that cause established connections to be terminated unexpectedly.

## Immediate Actions (0-5 minutes)

1. **Check Connection Metrics**
   ```bash
   curl -s http://localhost:9090/metrics | grep connection_
   ```

2. **Verify Service Health**
   ```bash
   curl http://localhost:8081/health
   ```

3. **Check Active Connections**
   ```promql
   tick_storm_active_connections
   rate(tick_storm_connection_errors_total{error_type="connection_dropped"}[5m])
   ```

## Diagnosis (5-15 minutes)

### 1. Network Issues
```bash
# Check network interface errors
cat /proc/net/dev | grep -E "(drop|err)"

# TCP connection statistics
ss -s
netstat -s | grep -i drop
```

### 2. Resource Constraints
```bash
# File descriptor usage
lsof -p $(pgrep tickstorm) | wc -l
ulimit -n

# Memory pressure
free -h
cat /proc/meminfo | grep -i available
```

### 3. Connection Patterns
```promql
# Connection duration before drops
histogram_quantile(0.50, rate(tick_storm_connection_duration_seconds_bucket[5m]))

# Error types
rate(tick_storm_connection_errors_total[5m]) by (error_type)
```

## Resolution Steps

### Scenario 1: File Descriptor Exhaustion
```bash
# Increase file descriptor limits
echo "tickstorm soft nofile 65536" >> /etc/security/limits.conf
echo "tickstorm hard nofile 65536" >> /etc/security/limits.conf

# For systemd service
mkdir -p /etc/systemd/system/tickstorm.service.d/
echo -e "[Service]\nLimitNOFILE=65536" > /etc/systemd/system/tickstorm.service.d/limits.conf
systemctl daemon-reload
systemctl restart tickstorm
```

### Scenario 2: Memory Pressure
```bash
# Scale up memory
kubectl patch deployment tickstorm -p '{"spec":{"template":{"spec":{"containers":[{"name":"tickstorm","resources":{"limits":{"memory":"4Gi"}}}]}}}}'

# Enable memory optimization
export GOGC=80
kubectl rollout restart deployment/tickstorm
```

### Scenario 3: Network Configuration
```bash
# Optimize TCP settings
sysctl -w net.core.somaxconn=65535
sysctl -w net.ipv4.tcp_max_syn_backlog=65535
sysctl -w net.core.netdev_max_backlog=5000

# Make permanent
echo "net.core.somaxconn = 65535" >> /etc/sysctl.conf
echo "net.ipv4.tcp_max_syn_backlog = 65535" >> /etc/sysctl.conf
sysctl -p
```

## Verification

```promql
# Verify connection drop rate is decreasing
rate(tick_storm_connection_errors_total{error_type="connection_dropped"}[5m]) / rate(tick_storm_total_connections_total[5m]) * 100 < 0.5
```

## Prevention

1. **Capacity Planning**
   - Monitor connection growth trends
   - Set up predictive scaling
   - Regular load testing

2. **System Tuning**
   ```bash
   # Optimize connection handling
   export MAX_CONNECTIONS=100000
   export CONNECTION_TIMEOUT=300s
   export KEEPALIVE_INTERVAL=30s
   ```

## Escalation

**Escalate if:**
- Connection drop rate remains > 0.5% after 15 minutes
- Service becomes unstable
- Multiple instances affected simultaneously
