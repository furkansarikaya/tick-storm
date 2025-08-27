# Metrics Scraping Failure Alert Runbook

## Alert: TickStormMetricsScrapeFailure

**Severity**: Medium  
**Threshold**: Prometheus cannot scrape metrics for > 3 minutes  
**SLA Impact**: Low - affects monitoring visibility

## Alert Description

This alert fires when Prometheus cannot successfully scrape metrics from TickStorm instances, affecting monitoring visibility and alerting capabilities.

## Immediate Actions (0-5 minutes)

1. **Check Metrics Endpoint**
   ```bash
   curl -f http://tickstorm-instance:9090/metrics || echo "Metrics endpoint failed"
   ```

2. **Verify Prometheus Configuration**
   ```bash
   # Check Prometheus targets
   curl http://prometheus:9090/api/v1/targets | jq '.data.activeTargets[] | select(.job=="tickstorm")'
   ```

## Diagnosis (5-10 minutes)

### 1. Endpoint Accessibility
```bash
# Test metrics endpoint directly
curl -v http://tickstorm-instance:9090/metrics

# Check port binding
ss -tuln | grep :9090
```

### 2. Service Configuration
```bash
# Verify metrics server is running
kubectl logs deployment/tickstorm | grep -i metrics

# Check service discovery
kubectl get endpoints tickstorm-metrics
```

## Resolution Steps

### Scenario 1: Metrics Server Not Running
```bash
# Restart TickStorm to reinitialize metrics server
kubectl rollout restart deployment/tickstorm
```

### Scenario 2: Network Connectivity
```bash
# Check service configuration
kubectl get svc tickstorm-metrics -o yaml

# Verify network policies
kubectl get networkpolicies
```

### Scenario 3: Resource Issues
```bash
# Check if metrics endpoint is overwhelmed
curl -w "@curl-format.txt" http://tickstorm-instance:9090/metrics
```

## Verification

```bash
# Verify metrics are accessible
curl -s http://tickstorm-instance:9090/metrics | head -10

# Check Prometheus targets
curl -s http://prometheus:9090/api/v1/targets | jq '.data.activeTargets[] | select(.job=="tickstorm") | .health'
```

## Prevention

1. **Monitoring Health**
   - Set up metrics endpoint health checks
   - Monitor scrape duration and success rates
   - Implement metrics endpoint redundancy

2. **Configuration Management**
   ```yaml
   # Ensure proper service configuration
   apiVersion: v1
   kind: Service
   metadata:
     name: tickstorm-metrics
   spec:
     ports:
     - port: 9090
       name: metrics
   ```

## Escalation

**Escalate if:**
- Multiple instances affected
- Metrics unavailable for > 10 minutes
- Critical alerts may be missed due to monitoring gaps
