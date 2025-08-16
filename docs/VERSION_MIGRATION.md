# Protocol Version Migration Strategy

## Overview

This document outlines the strategy for managing protocol version migrations in the Tick-Storm TCP stream server. The versioning system is designed to ensure backward compatibility, smooth client transitions, and maintainable protocol evolution.

## Version Management

### Current Version Support
- **Current Protocol Version**: `0x01` (v1.0)
- **Minimum Supported Version**: `0x01`
- **Maximum Supported Version**: `0x01`

### Version Lifecycle States

1. **Active**: Currently supported and recommended for new clients
2. **Deprecated**: Still supported but clients should migrate to newer versions
3. **End of Life (EOL)**: No longer supported, connections will be rejected

## Version Negotiation Process

### Client Connection Flow

1. **Initial Connection**: Client connects and sends first frame with its preferred version
2. **Version Validation**: Server validates the client version using `ValidateVersion()`
3. **Compatibility Check**: Server checks compatibility using `IsVersionCompatible()`
4. **Negotiation**: If direct compatibility fails, server attempts to find compatible version
5. **Response**: Server either accepts the version or sends error response

### Negotiation Algorithm

```go
func GetVersionNegotiationResponse(clientVersion uint8) (uint8, error) {
    // 1. Check if client version is supported
    if !IsVersionSupported(clientVersion) {
        return 0, fmt.Errorf("no compatible version found")
    }
    
    // 2. Check direct compatibility
    if IsVersionCompatible(CurrentProtocolVersion, clientVersion) {
        return clientVersion, nil
    }
    
    // 3. Find compatible version from matrix
    // Implementation details in internal/protocol/version.go
}
```

## Compatibility Matrix

The compatibility matrix defines which versions can communicate with each other:

```go
var DefaultCompatibilityMatrix = &VersionCompatibilityMatrix{
    ServerToClient: map[uint8][]uint8{
        0x01: {0x01}, // v1.0 server supports v1.0 clients
    },
    ClientToServer: map[uint8][]uint8{
        0x01: {0x01}, // v1.0 client supports v1.0 servers
    },
}
```

## Version Features

Each version defines a set of supported features:

### Version 1.0 (0x01) Features
- ✅ Authentication
- ✅ Subscription (SECOND/MINUTE modes)
- ✅ Heartbeat mechanism
- ✅ Data batch delivery
- ✅ Error reporting
- ✅ CRC32C checksum validation
- ✅ Input validation
- ✅ Rate limiting
- ✅ Asynchronous writes
- ✅ Object pooling
- ✅ TCP optimizations
- ❌ Compression (planned for future)
- ❌ TLS support (planned for future)

## Migration Strategies

### Adding New Versions

1. **Define Version Metadata**
   ```go
   0x02: {
       Name:        "v1.1",
       ReleaseDate: time.Date(2024, 12, 1, 0, 0, 0, 0, time.UTC),
       Features: VersionFeatures{
           // Enhanced features
           Compression: true,
           // ... other features
       },
   }
   ```

2. **Update Compatibility Matrix**
   ```go
   ServerToClient: map[uint8][]uint8{
       0x01: {0x01},
       0x02: {0x01, 0x02}, // v1.1 server supports both v1.0 and v1.1 clients
   }
   ```

3. **Implement Version-Specific Handlers**
   - Add new message types if needed
   - Implement feature-specific logic
   - Ensure backward compatibility

### Deprecating Old Versions

1. **Mark as Deprecated**
   ```go
   0x01: {
       Name:        "v1.0",
       Deprecated:  true,
       EOL:         &time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
   }
   ```

2. **Client Migration Period**
   - Send deprecation warnings in error responses
   - Provide migration documentation
   - Monitor usage metrics

3. **End of Life Process**
   - Remove from supported versions
   - Update compatibility matrix
   - Reject connections with EOL versions

## Error Handling

### Version-Specific Error Responses

The server provides detailed error information based on client version capabilities:

```go
func CreateVersionSpecificErrorResponse(version uint8, code pb.ErrorCode, message string) (*pb.ErrorResponse, error) {
    capabilities, err := GetVersionCapabilities(version)
    if err != nil {
        return nil, err
    }
    
    errorResp := &pb.ErrorResponse{
        Code:        code,
        Message:     message,
        TimestampMs: GetCurrentTimestamp(),
    }
    
    if capabilities.InputValidation {
        errorResp.Details = fmt.Sprintf("Version 0x%02X error: %s", version, message)
    }
    
    return errorResp, nil
}
```

### Common Error Scenarios

1. **Unsupported Version**: `ERROR_CODE_UNSUPPORTED_VERSION`
2. **Deprecated Version**: Warning in error details
3. **EOL Version**: Connection rejected with specific error
4. **Feature Not Available**: `ERROR_CODE_FEATURE_NOT_SUPPORTED`

## Monitoring and Metrics

### Version Usage Tracking

The system tracks version usage for migration planning:

```go
type VersionMetrics struct {
    VersionCounts       map[uint8]int64
    DeprecatedUsage     int64
    UnsupportedAttempts int64
}
```

### Key Metrics

- **Active Connections by Version**: Monitor version distribution
- **Deprecated Version Usage**: Track clients needing migration
- **Unsupported Attempts**: Identify problematic client versions
- **Feature Usage**: Understand which features are actively used

## Testing Strategy

### Version Compatibility Tests

1. **Cross-Version Communication**
   - Test all supported version combinations
   - Verify feature availability per version
   - Validate error responses

2. **Migration Scenarios**
   - Client upgrade/downgrade scenarios
   - Server version rollback compatibility
   - Feature degradation handling

3. **Edge Cases**
   - Unsupported version handling
   - Malformed version headers
   - Version negotiation failures

## Implementation Guidelines

### Adding New Features

1. **Feature Flag Approach**
   ```go
   if capabilities.NewFeature {
       // Implement new feature logic
   } else {
       // Fallback to compatible behavior
   }
   ```

2. **Message Type Extensions**
   - Add new message types for new features
   - Maintain backward compatibility for existing types
   - Use feature flags to control availability

3. **Protocol Extensions**
   - Extend frame format carefully
   - Ensure older versions can ignore new fields
   - Document breaking changes clearly

### Best Practices

1. **Gradual Rollout**: Deploy new versions incrementally
2. **Monitoring**: Track version adoption and issues
3. **Documentation**: Keep migration guides updated
4. **Testing**: Comprehensive cross-version testing
5. **Communication**: Notify clients of deprecations early

## Future Considerations

### Planned Features (v1.1+)

- **Compression**: Reduce bandwidth usage
- **TLS Support**: Enhanced security
- **Multi-tenancy**: Namespace isolation
- **Advanced Metrics**: Enhanced monitoring
- **Streaming Compression**: Real-time data compression

### Long-term Strategy

- Maintain 2-3 active versions simultaneously
- 6-month deprecation period minimum
- Annual major version releases
- Quarterly minor version releases for features
- Immediate patch releases for security issues

## Conclusion

This migration strategy ensures smooth protocol evolution while maintaining client compatibility. The version-aware architecture allows for gradual feature rollout and controlled deprecation of older versions.

For implementation details, see:
- `internal/protocol/version.go` - Version management
- `internal/server/version_handler.go` - Version-aware connection handling
- `internal/protocol/frame.go` - Frame-level version validation
