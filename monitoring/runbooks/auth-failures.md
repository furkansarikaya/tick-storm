# Authentication Failure Alert Runbook

## Alert: TickStormAuthFailureRate

**Severity**: High  
**Threshold**: Authentication failure rate > 5%  
**SLA Impact**: Medium - affects user access

## Alert Description

This alert fires when the authentication failure rate exceeds 5% of total authentication attempts. High authentication failure rates may indicate credential issues, brute force attacks, or system problems.

## Immediate Actions (0-5 minutes)

1. **Check Authentication Metrics**
   ```bash
   curl -s http://localhost:9090/metrics | grep auth_
   ```

2. **Review Recent Authentication Patterns**
   ```promql
   rate(tick_storm_auth_failures_total[5m]) by (reason)
   rate(tick_storm_auth_success_total[5m])
   ```

## Diagnosis (5-15 minutes)

### 1. Identify Failure Reasons
```promql
# Authentication failures by reason
rate(tick_storm_auth_failures_total[5m]) by (reason)

# Rate limiting activity
rate(tick_storm_auth_rate_limited_total[5m])
```

### 2. Check for Attack Patterns
```bash
# Look for suspicious IP patterns in logs
journalctl -u tickstorm --since "10 minutes ago" | grep -i "auth.*fail" | awk '{print $NF}' | sort | uniq -c | sort -nr
```

### 3. Verify Credential Configuration
```bash
# Check environment variables
env | grep AUTH
```

## Resolution Steps

### Scenario 1: Credential Issues
```bash
# Verify authentication configuration
echo $AUTH_USERNAME
echo $AUTH_PASSWORD | wc -c  # Check length without exposing

# Update credentials if needed
kubectl create secret generic tickstorm-auth --from-literal=username=newuser --from-literal=password=newpass --dry-run=client -o yaml | kubectl apply -f -
```

### Scenario 2: Brute Force Attack
```bash
# Implement IP blocking
# Add suspicious IPs to blocklist
export IP_BLOCKLIST="192.168.1.100,10.0.0.50"

# Restart service to apply new blocklist
kubectl rollout restart deployment/tickstorm
```

### Scenario 3: Rate Limiting Issues
```bash
# Adjust rate limiting parameters
export AUTH_MAX_ATTEMPTS=15
export AUTH_RATE_LIMIT_WINDOW=300s

# Restart to apply changes
kubectl rollout restart deployment/tickstorm
```

## Verification

```promql
# Verify auth failure rate is decreasing
rate(tick_storm_auth_failures_total[5m]) / (rate(tick_storm_auth_success_total[5m]) + rate(tick_storm_auth_failures_total[5m])) * 100 < 5
```

## Prevention

1. **Enhanced Monitoring**
   - Monitor authentication patterns by IP
   - Set up geolocation-based alerts
   - Track credential rotation schedules

2. **Security Hardening**
   ```bash
   # Implement stronger rate limiting
   AUTH_MAX_ATTEMPTS=10
   AUTH_RATE_LIMIT_WINDOW=600s
   
   # Enable IP allowlisting for known clients
   IP_ALLOWLIST="trusted-network-range"
   ```

## Escalation

**Escalate if:**
- Auth failure rate remains > 5% after 10 minutes
- Evidence of coordinated attack
- Legitimate users reporting access issues

**Escalation Path:**
1. Notify security team via Slack #security-alerts
2. Consider emergency credential rotation
3. Engage incident response team if attack suspected
