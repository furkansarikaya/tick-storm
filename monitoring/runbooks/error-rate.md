# Error Rate Alert Runbook

## Alert: TickStormErrorRate

**Severity**: High  
**Threshold**: Overall error rate > 1%  
**SLA Impact**: High - affects service reliability

## Alert Description

This alert fires when the overall error rate exceeds 1% of total connections. High error rates indicate system instability, resource issues, or external dependency problems that affect service reliability.

## Immediate Actions (0-5 minutes)

1. **Check Error Metrics**
   ```bash
   curl -s http://localhost:9090/metrics | grep errors_total
   ```

2. **Identify Error Types**
   ```promql
   rate(tick_storm_errors_total[5m]) by (error_type)
   sum(rate(tick_storm_errors_total[5m])) / sum(rate(tick_storm_total_connections_total[5m])) * 100
   ```

## Diagnosis (5-15 minutes)

### 1. Error Pattern Analysis
```bash
# Recent error logs
journalctl -u tickstorm --since "10 minutes ago" | grep -i error | tail -20

# Error distribution
curl -s http://localhost:9090/metrics | grep errors_total | sort
```

### 2. System Health Check
```bash
# Resource usage
top -p $(pgrep tickstorm) -n 1
free -h

# Network connectivity
ss -tuln | grep :8080
```

## Resolution Steps

### Scenario 1: Resource Exhaustion
```bash
# Scale resources
kubectl patch deployment tickstorm -p '{"spec":{"template":{"spec":{"containers":[{"name":"tickstorm","resources":{"limits":{"cpu":"2000m","memory":"2Gi"}}}]}}}}'
```

### Scenario 2: Configuration Issues
```bash
# Verify configuration
env | grep -E "(TIMEOUT|LIMIT|BUFFER)"

# Restart with clean state
kubectl rollout restart deployment/tickstorm
```

## Verification

```promql
# Verify error rate is decreasing
sum(rate(tick_storm_errors_total[5m])) / sum(rate(tick_storm_total_connections_total[5m])) * 100 < 1
```

## Prevention

1. **Error Monitoring**
   - Set up error trend analysis
   - Monitor error types and patterns
   - Implement circuit breakers

2. **System Hardening**
   ```bash
   # Error handling improvements
   export MAX_RETRIES=3
   export RETRY_BACKOFF=100ms
   export CIRCUIT_BREAKER_THRESHOLD=10
   ```

## Escalation

**Escalate if:**
- Error rate remains > 1% after 15 minutes
- Multiple error types increasing simultaneously
- Service stability compromised
