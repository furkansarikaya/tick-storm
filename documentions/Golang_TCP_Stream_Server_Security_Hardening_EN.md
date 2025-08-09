# SECURITY HARDENING GUIDELINES
## Golang TCP Stream Server (Binary Framing + Protobuf)

**Version:** 1.0  
**Date:** 2025-08-08  

## 1) Purpose

These guidelines provide mandatory and recommended security measures for operating the Golang TCP Stream Server in production environments.

## 2) Transport Security

- Enable TLS 1.3 for all external connections.
- For sensitive environments, enable mutual TLS (mTLS) with client certificates.
- Use strong ciphers and disable weak/legacy protocols.

## 3) Authentication & Authorization

- Require AUTH frame as first frame after TCP connect.
- Store STREAM_USER and STREAM_PASS securely in environment variables or secret managers.
- Rotate credentials periodically (e.g., every 90 days).
- Rate limit failed authentication attempts to mitigate brute-force attacks.

## 4) Data Integrity

- Validate CRC32C checksum for all incoming frames.
- Reject frames exceeding MAX_MSG_SIZE.
- Enforce correct Magic bytes and protocol version.

## 5) Input Validation

- Validate Protobuf message fields against schema.
- Reject unknown message types (Type field).
- Enforce required fields for each message type.

## 6) Network & Infrastructure

- Restrict server listening IP to trusted networks or firewall rules.
- Use a dedicated port for the TCP service.
- Apply OS-level limits (ulimit) to prevent resource exhaustion.
- Enable TCP keepalive for dead connection detection.

## 7) Logging & Monitoring

- Log authentication failures, heartbeat timeouts, and protocol violations.
- Monitor security-related metrics (failed_auth_total, invalid_frame_total).
- Set up alerts for unusual spikes in failed authentication or invalid messages.

## 8) Incident Response

**On suspected breach:**
1. Rotate credentials immediately.
2. Revoke affected client certificates (if mTLS enabled).
3. Review logs for malicious activity.
4. Patch and redeploy.

## 9) Secure Development Practices

- Run static code analysis (golangci-lint, gosec).
- Avoid committing secrets to source control.
- Perform security reviews before major releases.

Following these security hardening guidelines reduces the risk of unauthorized access, data corruption, and service disruption.