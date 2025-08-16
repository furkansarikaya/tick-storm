# Secret Management Documentation

## Overview

TickStorm implements a comprehensive secret management system designed to securely handle credentials, API keys, certificates, and other sensitive data. The system supports multiple storage backends, automatic rotation, audit logging, and emergency procedures.

## Architecture

### Core Components

1. **SecretManager Interface**: Unified interface for secret operations
2. **Storage Backends**: Environment variables, HashiCorp Vault, and extensible providers
3. **Rotation Scheduler**: Automated periodic secret rotation
4. **Audit Logger**: Comprehensive access logging and monitoring
5. **Validation System**: Secret format and security validation
6. **Caching Layer**: Performance optimization with TTL-based caching

### Secret Types

The system manages various types of secrets:

- **Authentication Credentials**: Username/password pairs
- **API Keys**: Service authentication tokens
- **TLS Certificates**: SSL/TLS certificate files and keys
- **Database Connections**: Database URLs and credentials
- **Encryption Keys**: Symmetric and asymmetric encryption keys

## Configuration

### Environment Manager

The default environment-based secret manager loads secrets from environment variables:

```go
manager := NewEnvSecretManager(&SecretManagerConfig{
    ValidationEnabled: true,
    FallbackToEnv:    true,
    DefaultTTL:       24 * time.Hour,
})
```

### Vault Manager

For production environments, use HashiCorp Vault:

```go
vaultConfig := &VaultConfig{
    Address:   "https://vault.example.com:8200",
    Token:     "hvs.CAESIJ...",
    Mount:     "secret",
    Namespace: "production",
    Timeout:   30 * time.Second,
}

manager, err := NewVaultSecretManager(vaultConfig)
```

## Secret Operations

### Retrieving Secrets

```go
ctx := context.Background()

// Get complete secret with metadata
secret, err := manager.GetSecret(ctx, "AUTH_PASSWORD")
if err != nil {
    log.Fatal(err)
}

// Get just the secret value
value, err := manager.GetSecretValue(ctx, "API_KEY")
if err != nil {
    log.Fatal(err)
}
```

### Storing Secrets

```go
metadata := &SecretMetadata{
    Description: "Database connection password",
    Tags:        []string{"database", "production"},
    Attributes: map[string]string{
        "environment": "prod",
        "service":     "tickstorm",
    },
    TTL: &[]time.Duration{7 * 24 * time.Hour}[0], // 7 days
}

err := manager.SetSecret(ctx, "DB_PASSWORD", "secure_password_123", metadata)
```

### Manual Rotation

```go
// Rotate a specific secret
newSecret, err := manager.RotateSecret(ctx, "API_KEY")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Secret rotated: v%d -> v%d\n", 
    newSecret.Version-1, newSecret.Version)
```

## Automatic Rotation

### Rotation Scheduler

Configure automatic rotation for critical secrets:

```go
config := &RotationConfig{
    DefaultInterval:   24 * time.Hour,
    CheckInterval:     time.Hour,
    MaxRetries:        3,
    RetryDelay:        5 * time.Minute,
    EmergencyInterval: 15 * time.Minute,
    Schedules: map[string]time.Duration{
        "AUTH_PASSWORD": 7 * 24 * time.Hour,   // Weekly
        "API_KEY":       30 * 24 * time.Hour,  // Monthly
        "JWT_SECRET":    90 * 24 * time.Hour,  // Quarterly
    },
}

scheduler := NewRotationScheduler(manager, config)
err := scheduler.Start(ctx)
```

### Rotation Hooks

Implement custom logic before and after rotation:

```go
type CustomRotationHook struct {
    notifier *NotificationService
}

func (h *CustomRotationHook) BeforeRotation(ctx context.Context, key string) error {
    return h.notifier.SendAlert("Starting rotation for " + key)
}

func (h *CustomRotationHook) AfterRotation(ctx context.Context, key string, success bool, err error) error {
    if success {
        return h.notifier.SendSuccess("Rotation completed for " + key)
    }
    return h.notifier.SendError("Rotation failed for " + key + ": " + err.Error())
}

scheduler.AddHook(&CustomRotationHook{notifier: notificationService})
```

## Emergency Procedures

### Emergency Rotation

Trigger immediate rotation for compromised secrets:

```go
emergencyManager := NewEmergencyRotationManager(scheduler)

result, err := emergencyManager.TriggerEmergencyRotation(
    ctx, 
    "COMPROMISED_API_KEY",
    "Security incident - key exposed in logs",
    "high",
)

if err != nil {
    log.Fatal("Emergency rotation failed:", err)
}

fmt.Printf("Emergency rotation completed in %v\n", result.Duration)
```

### Incident Response

1. **Immediate Actions**:
   - Trigger emergency rotation for affected secrets
   - Disable compromised credentials
   - Review audit logs for unauthorized access

2. **Investigation**:
   - Analyze access patterns
   - Identify potential data exposure
   - Document incident timeline

3. **Recovery**:
   - Verify new credentials are working
   - Update dependent services
   - Monitor for continued issues

## Audit and Monitoring

### Audit Logging

All secret operations are automatically logged:

```go
// Audit logs include:
type SecretAuditLog struct {
    Timestamp time.Time                 `json:"timestamp"`
    Operation string                    `json:"operation"`
    SecretKey string                    `json:"secret_key"`
    Source    string                    `json:"source"`
    Success   bool                      `json:"success"`
    Error     string                    `json:"error,omitempty"`
    Metadata  map[string]interface{}    `json:"metadata,omitempty"`
}
```

### Monitoring Metrics

Key metrics to monitor:

- **Secret Access Frequency**: Unusual access patterns
- **Rotation Success Rate**: Failed rotations requiring attention
- **Cache Hit Rate**: Performance optimization opportunities
- **Validation Failures**: Potential security issues
- **Emergency Rotations**: Security incident indicators

### Alerting

Set up alerts for:

- Failed secret rotations (3+ consecutive failures)
- Emergency rotation triggers
- Unusual access patterns (>100 requests/minute per key)
- Validation failures (>10% failure rate)
- Cache misses (>50% miss rate)

## Security Best Practices

### Secret Generation

- **Passwords**: Minimum 24 characters, mixed case, numbers, symbols
- **API Keys**: 32+ character random strings
- **Encryption Keys**: Use cryptographically secure random generation
- **Certificates**: Follow organizational PKI policies

### Storage Security

- **Environment Variables**: Use for development only
- **Vault**: Recommended for production environments
- **Encryption**: All secrets encrypted at rest and in transit
- **Access Control**: Principle of least privilege

### Rotation Policies

- **Critical Secrets**: Weekly rotation (auth credentials)
- **API Keys**: Monthly rotation
- **Encryption Keys**: Quarterly rotation
- **Certificates**: Based on expiration dates

### Validation Rules

- **Length**: Minimum 12 characters for passwords
- **Complexity**: Mixed character types required
- **Entropy**: Minimum entropy requirements
- **Blacklists**: Common passwords and patterns blocked

## Integration Examples

### Server Integration

```go
// Initialize secret manager in server startup
func initSecretManager() SecretManager {
    if os.Getenv("VAULT_ADDR") != "" {
        // Production: Use Vault
        config := &VaultConfig{
            Address: os.Getenv("VAULT_ADDR"),
            Token:   os.Getenv("VAULT_TOKEN"),
            Mount:   "secret",
        }
        manager, err := NewVaultSecretManager(config)
        if err != nil {
            log.Fatal("Failed to initialize Vault manager:", err)
        }
        return manager
    }
    
    // Development: Use environment variables
    return NewEnvSecretManager(nil)
}

// Use in authentication
func authenticateClient(manager SecretManager, username, password string) error {
    expectedPassword, err := manager.GetSecretValue(ctx, "AUTH_PASSWORD")
    if err != nil {
        return fmt.Errorf("failed to get auth password: %w", err)
    }
    
    if password != expectedPassword {
        return fmt.Errorf("invalid credentials")
    }
    
    return nil
}
```

### TLS Certificate Management

```go
// Load TLS certificates from secret manager
func loadTLSConfig(manager SecretManager) (*tls.Config, error) {
    certPEM, err := manager.GetSecretValue(ctx, "TLS_CERT_FILE")
    if err != nil {
        return nil, err
    }
    
    keyPEM, err := manager.GetSecretValue(ctx, "TLS_KEY_FILE")
    if err != nil {
        return nil, err
    }
    
    cert, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
    if err != nil {
        return nil, err
    }
    
    return &tls.Config{
        Certificates: []tls.Certificate{cert},
        MinVersion:   tls.VersionTLS13,
    }, nil
}
```

## Troubleshooting

### Common Issues

1. **Secret Not Found**:
   - Verify secret key spelling
   - Check environment variables or Vault path
   - Ensure proper permissions

2. **Rotation Failures**:
   - Check network connectivity to Vault
   - Verify token permissions
   - Review rotation logs for errors

3. **Validation Errors**:
   - Ensure secrets meet complexity requirements
   - Check for invalid characters
   - Verify secret length requirements

4. **Performance Issues**:
   - Monitor cache hit rates
   - Adjust cache TTL settings
   - Consider connection pooling

### Debug Commands

```bash
# Check secret manager status
curl -s http://localhost:8080/health/secrets | jq

# View rotation schedules
curl -s http://localhost:8080/admin/secrets/schedules | jq

# Trigger manual rotation
curl -X POST http://localhost:8080/admin/secrets/rotate/API_KEY

# View audit logs
curl -s http://localhost:8080/admin/secrets/audit | jq
```

## Compliance and Governance

### Regulatory Requirements

- **SOC 2**: Audit logging and access controls
- **PCI DSS**: Encryption and key management
- **GDPR**: Data protection and retention policies
- **HIPAA**: Access logging and encryption

### Documentation Requirements

- Secret inventory and classification
- Rotation schedules and procedures
- Incident response procedures
- Access control policies

### Regular Reviews

- **Monthly**: Review rotation schedules and failures
- **Quarterly**: Audit access patterns and permissions
- **Annually**: Review and update security policies

## API Reference

### SecretManager Interface

```go
type SecretManager interface {
    GetSecret(ctx context.Context, key string) (*Secret, error)
    SetSecret(ctx context.Context, key string, value string, metadata *SecretMetadata) error
    RotateSecret(ctx context.Context, key string) (*Secret, error)
    ListSecrets(ctx context.Context) ([]string, error)
    DeleteSecret(ctx context.Context, key string) error
    ValidateSecret(ctx context.Context, key string) error
    Close() error
}
```

### Configuration Structures

```go
type SecretManagerConfig struct {
    ValidationEnabled bool          `json:"validation_enabled"`
    FallbackToEnv    bool          `json:"fallback_to_env"`
    DefaultTTL       time.Duration `json:"default_ttl"`
}

type Secret struct {
    Key       string            `json:"key"`
    Value     string            `json:"value"`
    Version   int               `json:"version"`
    CreatedAt time.Time         `json:"created_at"`
    UpdatedAt time.Time         `json:"updated_at"`
    ExpiresAt *time.Time        `json:"expires_at,omitempty"`
    Metadata  *SecretMetadata   `json:"metadata,omitempty"`
}
```

## Conclusion

The TickStorm secret management system provides enterprise-grade security for sensitive data with automated rotation, comprehensive auditing, and flexible storage backends. Follow the documented procedures and best practices to maintain a secure and compliant environment.

For additional support or questions, refer to the troubleshooting section or contact the security team.
