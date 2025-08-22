# Network Security Guide

This guide explains how to securely deploy Tick‑Storm by controlling listener binding and filtering client IPs at the server level, alongside recommended firewall configurations.

## Overview

- IP filtering is enforced at connection accept time in `internal/server/server.go` `acceptLoop()`.
- Blocklist takes precedence over allowlist.
- If allowlist is empty, all IPs are allowed unless blocklisted.
- Both IPv4 and IPv6 addresses and CIDR ranges are supported.

## Configuration

Environment variables (prefer container/orchestrator secrets/config maps for management):

- LISTEN_ADDR: Full listen address (e.g., 127.0.0.1:8080 or [::1]:8080). Takes precedence over LISTEN_HOST/PORT.
- LISTEN_HOST: Listen host only (e.g., 0.0.0.0, 127.0.0.1, ::, or a specific interface IP).
- LISTEN_PORT: Listen port only (e.g., 8080). Defaults to 8080 if not specified.
- IP_ALLOWLIST: Comma-separated list of IPs or CIDRs allowed (e.g., 10.0.0.0/8,192.168.0.0/16,203.0.113.10).
- IP_BLOCKLIST: Comma-separated list of IPs or CIDRs blocked (e.g., 198.51.100.0/24,203.0.113.200).

Notes:
- Blocklist wins over allowlist.
- Empty allowlist means allow all unless blocked.
- IPv6 is fully supported (e.g., 2001:db8::/32, 2001:db8::1).

## Examples

### Docker Compose

```yaml
services:
  tick-storm:
    image: tickstorm:latest
    ports:
      - "8080:8080"
    environment:
      - LISTEN_ADDR=0.0.0.0:8080
      # Alternative split form
      # - LISTEN_HOST=0.0.0.0
      # - LISTEN_PORT=8080
      - IP_ALLOWLIST=10.0.0.0/8,192.168.0.0/16
      - IP_BLOCKLIST=198.51.100.0/24
```

### Kubernetes (excerpt)

```yaml
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
      - name: tickstorm
        image: tickstorm:latest
        env:
        - name: LISTEN_ADDR
          value: ":8080"
        - name: IP_ALLOWLIST
          value: "10.0.0.0/8,192.168.0.0/16"
        - name: IP_BLOCKLIST
          value: "198.51.100.0/24"
```

## Firewall Hardening

Server‑side IP filtering should complement — not replace — firewall rules.

### Linux (iptables)

```bash
# Allow RFC1918 ranges (example) and block an abuse subnet
sudo iptables -A INPUT -p tcp --dport 8080 -s 10.0.0.0/8 -j ACCEPT
sudo iptables -A INPUT -p tcp --dport 8080 -s 192.168.0.0/16 -j ACCEPT
sudo iptables -A INPUT -p tcp --dport 8080 -s 198.51.100.0/24 -j DROP
# Default deny, then allow loopback (adjust policy to your baseline)
sudo iptables -A INPUT -i lo -j ACCEPT
sudo iptables -A INPUT -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT
sudo iptables -A INPUT -p tcp --dport 8080 -j DROP
```

### Linux (nftables)

```bash
sudo nft add table inet filter
sudo nft add chain inet filter input { type filter hook input priority 0; policy drop; }
sudo nft add rule inet filter input ct state established,related accept
sudo nft add rule inet filter input iif lo accept
sudo nft add rule inet filter input tcp dport 8080 ip saddr 10.0.0.0/8 accept
sudo nft add rule inet filter input tcp dport 8080 ip saddr 192.168.0.0/16 accept
sudo nft add rule inet filter input tcp dport 8080 ip saddr 198.51.100.0/24 drop
```

### Ubuntu (ufw)

```bash
sudo ufw allow from 10.0.0.0/8 to any port 8080 proto tcp
sudo ufw allow from 192.168.0.0/16 to any port 8080 proto tcp
sudo ufw deny from 198.51.100.0/24 to any port 8080 proto tcp
```

### Cloud Load Balancers / Security Groups

- Prefer security groups/load balancer ACLs to limit source ranges.
- For AWS: restrict ALB/NLB listener source CIDRs; set EC2 SG inbound rules to specific CIDRs only.
- For GCP/Azure: configure firewall rules to only allow expected ranges.

### Kubernetes NetworkPolicy (optional)

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: tickstorm-allow-ingress
spec:
  podSelector:
    matchLabels:
      app: tickstorm
  policyTypes: [Ingress]
  ingress:
  - from:
    - ipBlock:
        cidr: 10.0.0.0/8
        except: ["10.0.5.0/24"]
    ports:
    - protocol: TCP
      port: 8080
```

## Authentication & Rate Limiting

Tick‑Storm enforces authentication on the very first frame. Per‑IP rate limiting (port is ignored) is applied to mitigate brute‑force and credential‑stuffing attempts.

Environment variables:

- STREAM_USER: Expected username for AUTH.
- STREAM_PASS: Expected password for AUTH.
- AUTH_MAX_ATTEMPTS: Max allowed authentication attempts per IP within the window. Default: 3.
- AUTH_RATE_LIMIT_WINDOW: Window duration for counting attempts (e.g., "1m", "30s"). Default: 1m.

Behavior:

- First frame must be AUTH. Otherwise the server responds with error code `ERROR_CODE_AUTH_REQUIRED` and closes the connection.
- Rate limiting is enforced per IP address (source port is ignored). Exceeding `AUTH_MAX_ATTEMPTS` within `AUTH_RATE_LIMIT_WINDOW` temporarily blocks further attempts from that IP.
- On invalid credentials, limiter penalties apply to slow down repeated failures. A successful authentication resets limiter state for that IP.

Client error codes related to authentication:

- ERROR_CODE_INVALID_AUTH: Invalid credentials.
- ERROR_CODE_AUTH_REQUIRED: First frame was not AUTH.
- ERROR_CODE_ALREADY_AUTHENTICATED: Duplicate authentication attempt on the same connection.
- ERROR_CODE_RATE_LIMITED: Too many authentication attempts from the same IP.

## Monitoring and Visibility

- The server counts IP‑filtered rejections via `GlobalMetrics.IncrementIPRejectedConnections()` and surfaces it through the metrics snapshot in `internal/server/metrics.go`.
- Authentication metrics are exposed via the server stats (`internal/server/server.go` `GetStats()`):
  - auth_success: total successful authentications
  - auth_failures: total authentication failures
  - auth_rate_limited: total attempts rejected due to rate limiting
- Add log sampling for rejected connections in front‑end proxies/load balancers.

## DDoS Protection

Tick-Storm includes built-in DDoS protection mechanisms to defend against various attack vectors:

### Connection Rate Limiting
- **Per-IP Connection Limits**: Maximum 100 concurrent connections per IP address
- **Connection Rate Control**: Maximum 10 new connections per second per IP
- **Burst Protection**: Prevents rapid connection flooding from single sources

### Port Scanning Detection
- **Threshold-Based Detection**: Triggers when >20 ports accessed from single IP within 5 minutes
- **Consecutive Scan Detection**: Identifies rapid sequential port access attempts
- **Automatic Blocking**: Port scanning IPs are automatically blocked from new connections

### Metrics and Monitoring
DDoS protection metrics are available via server stats with `ddos_` prefix:
- `ddos_blocked_connections`: Total connections blocked due to DDoS protection
- `ddos_rate_limited_connections`: Connections rejected due to rate limiting
- `ddos_port_scan_attempts`: Total port scanning attempts detected
- `ddos_active_tracked_ips`: Number of IPs currently being tracked
- `ddos_suspicious_ips`: Number of IPs flagged for suspicious activity

## Network Monitoring and Alerting

The server includes comprehensive network monitoring with configurable alerting:

### Alert Types
- **High Connection Rate**: Alerts when connection rate exceeds 1000/second
- **High Failure Rate**: Critical alert when >50% of connections fail
- **Port Scanning Activity**: Critical alert for >100 port scan attempts/minute

### Alert Handlers
- **Structured Logging**: All alerts logged with detailed metadata
- **Extensible System**: Custom alert handlers can be added
- **Cooldown Protection**: 5-minute cooldown between similar alerts

### Configuration
Environment variables for monitoring thresholds:
- `MONITOR_MAX_CONN_PER_SEC`: Maximum connections per second threshold
- `MONITOR_MAX_FAILURE_RATE`: Maximum connection failure rate (0.0-1.0)
- `MONITOR_MAX_PORT_SCANS_PER_MIN`: Maximum port scans per minute

## Best Practices

- Bind to loopback (`127.0.0.1` or `[::1]`) for local dev.
- Use `LISTEN_ADDR` to target a specific interface instead of `0.0.0.0` when possible.
- Maintain allowlist as primary control; use blocklist for tactical denies.
- Keep allow/block lists minimal and aggregated (use CIDR).
- Always deploy complementary firewall policies.
- Monitor DDoS protection metrics regularly for attack patterns.
- Configure network monitoring alerts for your environment.
- Use geographic IP blocking for additional protection if needed.

## Troubleshooting

- Confirm listener with `ss -tlnp | grep 8080` and that it binds to the expected address.
- Verify effective allow/block configuration via unit tests and small integration checks.
- Inspect server logs and metrics snapshot for rejected connection counts.
- Check DDoS protection metrics if connections are being unexpectedly blocked.
- Review network monitoring alerts for security incidents.
- Verify port scanning detection isn't triggering false positives.
