# Instance Down Alert Runbook

## Alert: TickStormInstanceDown

**Severity**: Critical  
**Threshold**: Instance unreachable for > 1 minute  
**SLA Impact**: Critical - instance unavailability

## Alert Description

This alert fires when a specific TickStorm instance becomes unreachable by Prometheus for more than 1 minute, indicating the instance is down or network connectivity issues.

## Immediate Actions (0-2 minutes)

1. **Verify Instance Status**
   ```bash
   kubectl get pods -l app=tickstorm -o wide
   kubectl describe pod tickstorm-instance-name
   ```

2. **Check Node Health**
   ```bash
   kubectl get nodes
   kubectl describe node node-name
   ```

## Diagnosis (2-5 minutes)

### 1. Pod Status Analysis
```bash
# Pod events and logs
kubectl describe pod tickstorm-instance-name
kubectl logs tickstorm-instance-name --tail=100
```

### 2. Resource Constraints
```bash
# Node resource usage
kubectl top nodes
kubectl top pods -l app=tickstorm
```

## Resolution Steps

### Scenario 1: Pod Restart Required
```bash
# Delete problematic pod (will be recreated)
kubectl delete pod tickstorm-instance-name
```

### Scenario 2: Node Issues
```bash
# Drain and reschedule if node problems
kubectl drain node-name --ignore-daemonsets --delete-emptydir-data
```

### Scenario 3: Resource Exhaustion
```bash
# Scale deployment
kubectl scale deployment tickstorm --replicas=3
```

## Verification

```bash
# Verify instance is healthy
kubectl get pods -l app=tickstorm
curl http://new-instance:8081/health
```

## Escalation

**Escalate if:**
- Multiple instances down simultaneously
- Node-level issues detected
- Auto-recovery fails
