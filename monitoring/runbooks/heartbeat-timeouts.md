# Heartbeat Timeout Alert Runbook

## Alert: TickStormHeartbeatTimeoutRate

**Severity**: Critical  
**Threshold**: Heartbeat timeout rate > 1%  
**SLA Impact**: High - affects connection reliability

## Alert Description

This alert fires when the rate of heartbeat timeouts exceeds 1% of total heartbeat messages. Heartbeat timeouts indicate that clients are not responding to heartbeat requests within the configured timeout period (default: 20 seconds).

## Immediate Actions (0-5 minutes)

1. **Check Service Health**
   ```bash
   curl http://tickstorm-instance:8081/health
   curl http://tickstorm-instance:9090/metrics | grep heartbeat
   ```

2. **Verify Alert Scope**
   - Check if multiple instances are affected
   - Determine if specific client patterns are involved

3. **Review Current Metrics**
   ```promql
   # Current heartbeat timeout rate
   rate(tick_storm_heartbeat_timeouts_total[5m]) / rate(tick_storm_heartbeats_recv_total[5m]) * 100
   
   # Active connections
   tick_storm_active_connections
   
   # Recent connection patterns
   rate(tick_storm_total_connections_total[5m])
   ```

## Diagnosis (5-15 minutes)

### 1. Network Connectivity Issues
```bash
# Check network latency to common client locations
ping -c 10 client-network-range

# Check for packet loss
netstat -i
ss -tuln | grep :8080
```

### 2. Server Resource Constraints
```bash
# Check CPU and memory usage
top -p $(pgrep tickstorm)
free -h

# Check file descriptor usage
lsof -p $(pgrep tickstorm) | wc -l
ulimit -n

# Check goroutine count
curl -s http://localhost:9090/metrics | grep tick_storm_goroutines
```

### 3. Client Behavior Analysis
```bash
# Check connection duration patterns
curl -s http://localhost:9090/metrics | grep connection_duration

# Look for authentication issues
curl -s http://localhost:9090/metrics | grep auth_failures
```

### 4. Log Analysis
```bash
# Check for heartbeat-related errors
journalctl -u tickstorm -f --since "10 minutes ago" | grep -i heartbeat

# Look for connection errors
journalctl -u tickstorm -f --since "10 minutes ago" | grep -i "connection\|timeout"
```

## Resolution Steps

### Scenario 1: Network Issues
1. **Verify Load Balancer Health**
   ```bash
   # Check load balancer status
   curl -I http://load-balancer/health
   
   # Verify backend health checks
   # (Load balancer specific commands)
   ```

2. **Check Firewall Rules**
   ```bash
   # Verify port 8080 is accessible
   telnet tickstorm-instance 8080
   
   # Check iptables rules
   sudo iptables -L -n | grep 8080
   ```

### Scenario 2: Resource Exhaustion
1. **Scale Resources**
   ```bash
   # Increase memory limits (Kubernetes)
   kubectl patch deployment tickstorm -p '{"spec":{"template":{"spec":{"containers":[{"name":"tickstorm","resources":{"limits":{"memory":"2Gi"}}}]}}}}'
   
   # Scale replicas
   kubectl scale deployment tickstorm --replicas=3
   ```

2. **Optimize Configuration**
   ```bash
   # Reduce heartbeat timeout if appropriate
   export HEARTBEAT_TIMEOUT=15s
   
   # Increase connection limits
   export MAX_CONNECTIONS=150000
   ```

### Scenario 3: Application Issues
1. **Restart Service**
   ```bash
   # Graceful restart
   kubectl rollout restart deployment/tickstorm
   
   # Or systemd restart
   sudo systemctl restart tickstorm
   ```

2. **Check Configuration**
   ```bash
   # Verify heartbeat configuration
   env | grep HEARTBEAT
   
   # Check connection timeouts
   env | grep TIMEOUT
   ```

## Verification (15-20 minutes)

1. **Monitor Alert Status**
   ```promql
   # Verify heartbeat timeout rate is decreasing
   rate(tick_storm_heartbeat_timeouts_total[5m]) / rate(tick_storm_heartbeats_recv_total[5m]) * 100 < 1
   ```

2. **Check Service Metrics**
   ```bash
   # Verify active connections are stable
   curl -s http://localhost:9090/metrics | grep tick_storm_active_connections
   
   # Check for new connection errors
   curl -s http://localhost:9090/metrics | grep connection_errors
   ```

3. **Test Client Connectivity**
   ```bash
   # Test connection from client network
   telnet tickstorm-instance 8080
   
   # Verify heartbeat responses
   # (Use test client if available)
   ```

## Prevention

1. **Monitoring Improvements**
   - Set up network latency monitoring
   - Add client-side heartbeat metrics
   - Monitor connection duration patterns

2. **Configuration Tuning**
   ```bash
   # Optimize heartbeat intervals
   HEARTBEAT_INTERVAL=10s
   HEARTBEAT_TIMEOUT=15s
   
   # Tune TCP keepalive
   TCP_KEEPALIVE_TIME=30
   TCP_KEEPALIVE_INTVL=5
   TCP_KEEPALIVE_PROBES=3
   ```

3. **Capacity Planning**
   - Monitor connection growth trends
   - Plan for peak usage periods
   - Set up auto-scaling rules

## Escalation

**Escalate if:**
- Heartbeat timeout rate remains > 1% after 20 minutes
- Multiple instances are affected simultaneously
- Service becomes completely unresponsive
- Client impact is widespread

**Escalation Path:**
1. Page on-call infrastructure team
2. Notify team lead via Slack #tickstorm-alerts
3. Consider emergency maintenance window

## Related Alerts

- `TickStormConnectionDropRate` - May indicate related connectivity issues
- `TickStormServiceDown` - Could be consequence of widespread timeouts
- `TickStormMemoryUsageHigh` - Resource constraints causing timeouts

## Useful Queries

```promql
# Heartbeat timeout rate by instance
rate(tick_storm_heartbeat_timeouts_total[5m]) by (instance)

# Connection patterns
rate(tick_storm_total_connections_total[5m]) by (instance)

# Resource usage correlation
tick_storm_memory_usage_bytes / (1024*1024*1024) > 0.5
```
