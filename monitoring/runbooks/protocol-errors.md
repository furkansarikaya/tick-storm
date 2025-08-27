# Protocol Error Alert Runbook

## Alert: TickStormProtocolErrorRate

**Severity**: Medium  
**Threshold**: Protocol error rate > 0.5%  
**SLA Impact**: Medium - affects message reliability

## Alert Description

This alert fires when the protocol error rate exceeds 0.5% of received messages. Protocol errors indicate malformed messages, version mismatches, or client implementation issues.

## Immediate Actions (0-5 minutes)

1. **Check Protocol Metrics**
   ```bash
   curl -s http://localhost:9090/metrics | grep protocol_errors
   ```

2. **Review Error Distribution**
   ```promql
   rate(tick_storm_protocol_errors_total[5m]) by (error_type)
   rate(tick_storm_messages_recv_total[5m])
   ```

## Diagnosis (5-15 minutes)

### 1. Protocol Error Analysis
```bash
# Recent protocol errors
journalctl -u tickstorm --since "10 minutes ago" | grep -i "protocol\|malformed\|invalid"

# Message statistics
curl -s http://localhost:9090/metrics | grep messages_
```

### 2. Client Pattern Analysis
```bash
# Connection patterns
curl -s http://localhost:9090/metrics | grep connection_duration

# Authentication patterns (may indicate client issues)
curl -s http://localhost:9090/metrics | grep auth_
```

## Resolution Steps

### Scenario 1: Client Version Issues
```bash
# Check for version compatibility
# Review client connection logs for version information
journalctl -u tickstorm --since "30 minutes ago" | grep -i version
```

### Scenario 2: Message Format Issues
```bash
# Enable detailed protocol logging (if available)
export LOG_LEVEL=debug
export PROTOCOL_DEBUG=true
kubectl rollout restart deployment/tickstorm
```

### Scenario 3: Network Corruption
```bash
# Check network interface errors
cat /proc/net/dev | grep -E "(drop|err|fifo)"

# Verify checksums are working
curl -s http://localhost:9090/metrics | grep checksum
```

## Verification

```promql
# Verify protocol error rate is decreasing
rate(tick_storm_protocol_errors_total[5m]) / rate(tick_storm_messages_recv_total[5m]) * 100 < 0.5
```

## Prevention

1. **Protocol Validation**
   - Implement stricter message validation
   - Add protocol version negotiation
   - Enhance error reporting to clients

2. **Client Education**
   - Update client libraries
   - Provide protocol documentation
   - Implement client-side validation

## Escalation

**Escalate if:**
- Protocol error rate remains > 0.5% after 20 minutes
- Multiple client types affected
- New protocol version deployment suspected
