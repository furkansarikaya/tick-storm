# TickStorm Alert Runbooks

This directory contains operational runbooks for responding to TickStorm alerts. Each runbook provides step-by-step procedures for diagnosing and resolving specific issues.

## Runbook Index

### Critical Alerts
- [Service Down](./service-down.md) - When TickStorm service is completely unavailable
- [Instance Down](./instance-down.md) - When a specific instance becomes unreachable
- [Heartbeat Timeouts](./heartbeat-timeouts.md) - High rate of client heartbeat timeouts

### Performance Issues
- [High Latency](./high-latency.md) - Publish latency exceeds SLA thresholds
- [Message Processing](./message-processing.md) - Slow message processing performance
- [Write Timeouts](./write-timeouts.md) - High rate of write operation timeouts
- [Low Throughput](./low-throughput.md) - Message throughput below expected levels

### Connection Issues
- [Connection Drops](./connection-drops.md) - High rate of connection drops
- [No Messages](./no-messages.md) - No messages being sent despite active connections
- [Capacity Planning](./capacity-planning.md) - Approaching connection capacity limits

### Authentication Issues
- [Auth Failures](./auth-failures.md) - High authentication failure rates
- [Auth Rate Limiting](./auth-rate-limiting.md) - Excessive authentication rate limiting

### Resource Issues
- [High Memory](./high-memory.md) - Memory usage exceeding thresholds
- [High Goroutines](./high-goroutines.md) - Goroutine count exceeding limits
- [High CPU](./high-cpu.md) - CPU usage exceeding thresholds

### Error Handling
- [Error Rate](./error-rate.md) - Overall error rate exceeding thresholds
- [Protocol Errors](./protocol-errors.md) - Protocol-specific error handling

### Monitoring Issues
- [Metrics Scraping](./metrics-scraping.md) - Prometheus metrics collection failures

## Runbook Structure

Each runbook follows this standard structure:

1. **Alert Description** - What the alert means
2. **Immediate Actions** - First steps to take
3. **Diagnosis** - How to investigate the root cause
4. **Resolution** - Step-by-step fix procedures
5. **Prevention** - How to prevent recurrence
6. **Escalation** - When and how to escalate

## Emergency Contacts

- **On-Call Engineer**: Use PagerDuty escalation
- **Team Lead**: Available via Slack #tickstorm-alerts
- **Infrastructure Team**: #infrastructure-support

## Useful Links

- [Grafana Dashboard](${GRAFANA_URL}/d/tickstorm-overview)
- [Prometheus Alerts](${PROMETHEUS_URL}/alerts)
- [Service Logs](${LOGS_URL}/tickstorm)
- [Deployment Pipeline](${CI_URL}/tickstorm)

## General Troubleshooting Tips

1. **Check Service Health**: Always start with the health endpoint
2. **Review Recent Changes**: Check deployment history and configuration changes
3. **Examine Logs**: Look for error patterns and stack traces
4. **Monitor Metrics**: Use Grafana dashboards for trend analysis
5. **Test Connectivity**: Verify network connectivity and DNS resolution
6. **Resource Usage**: Check CPU, memory, and file descriptor usage
7. **Database Health**: Verify external dependencies if applicable
