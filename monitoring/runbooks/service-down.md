# Service Down Alert Runbook

## Alert: TickStormServiceDown

**Severity**: Critical  
**Threshold**: No active connections and no new connections for 1 minute  
**SLA Impact**: Critical - complete service unavailability

## Alert Description

This alert fires when TickStorm has no active connections and no new connections for 1 minute, indicating complete service unavailability.

## Immediate Actions (0-2 minutes)

1. **Verify Service Status**
   ```bash
   curl -f http://tickstorm-instance:8081/health || echo "Health check failed"
   kubectl get pods -l app=tickstorm
   ```

2. **Check Process Status**
   ```bash
   systemctl status tickstorm
   ps aux | grep tickstorm
   ```

## Diagnosis (2-5 minutes)

### 1. Container/Process Health
```bash
# Kubernetes deployment status
kubectl describe deployment tickstorm
kubectl logs deployment/tickstorm --tail=50

# System service status
journalctl -u tickstorm --since "5 minutes ago" --no-pager
```

### 2. Network Connectivity
```bash
# Port accessibility
telnet tickstorm-instance 8080
ss -tuln | grep :8080

# Load balancer health
curl -I http://load-balancer/health
```

## Resolution Steps

### Scenario 1: Process Crashed
```bash
# Restart service
kubectl rollout restart deployment/tickstorm
# OR
systemctl restart tickstorm
```

### Scenario 2: Network Issues
```bash
# Check firewall
sudo iptables -L -n | grep 8080

# Verify DNS resolution
nslookup tickstorm-instance
```

### Scenario 3: Resource Exhaustion
```bash
# Check system resources
df -h
free -h
uptime

# Scale if needed
kubectl scale deployment tickstorm --replicas=2
```

## Verification

```bash
# Verify service is accepting connections
curl http://tickstorm-instance:8081/health
telnet tickstorm-instance 8080
```

## Escalation

**Escalate immediately:**
- Page on-call engineer
- Notify incident commander
- Activate emergency procedures
