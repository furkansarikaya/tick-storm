package protocol

import (
	"fmt"
	"time"
)

// GetCurrentTimestamp returns the current timestamp in milliseconds
func GetCurrentTimestamp() int64 {
	return time.Now().UnixMilli()
}

// Protocol version constants
const (
	// Current protocol version
	CurrentProtocolVersion = 0x01
	
	// Minimum supported version for backward compatibility
	MinSupportedVersion = 0x01
	
	// Maximum supported version
	MaxSupportedVersion = 0x01
)

// Version represents a protocol version with its capabilities
type Version struct {
	Number      uint8
	Name        string
	ReleaseDate time.Time
	Features    VersionFeatures
	Deprecated  bool
	EOL         *time.Time // End of Life date
}

// VersionFeatures defines what features are available in each version
type VersionFeatures struct {
	// Core features
	Authentication    bool
	Subscription      bool
	Heartbeat        bool
	DataBatch        bool
	ErrorReporting   bool
	
	// Advanced features
	CRC32Checksum    bool
	InputValidation  bool
	RateLimiting     bool
	Compression      bool
	TLS              bool
	
	// Performance features
	AsyncWrites      bool
	ObjectPooling    bool
	TCPOptimizations bool
}

// SupportedVersions defines all supported protocol versions
var SupportedVersions = map[uint8]*Version{
	0x01: {
		Number:      0x01,
		Name:        "v1.0",
		ReleaseDate: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Features: VersionFeatures{
			Authentication:    true,
			Subscription:      true,
			Heartbeat:        true,
			DataBatch:        true,
			ErrorReporting:   true,
			CRC32Checksum:    true,
			InputValidation:  true,
			RateLimiting:     true,
			Compression:      false, // Not implemented yet
			TLS:              false, // Not implemented yet
			AsyncWrites:      true,
			ObjectPooling:    true,
			TCPOptimizations: true,
		},
		Deprecated: false,
		EOL:        nil,
	},
}

// VersionCompatibilityMatrix defines which versions can communicate
type VersionCompatibilityMatrix struct {
	// Server version -> list of compatible client versions
	ServerToClient map[uint8][]uint8
	// Client version -> list of compatible server versions
	ClientToServer map[uint8][]uint8
}

// DefaultCompatibilityMatrix defines the default compatibility rules
var DefaultCompatibilityMatrix = &VersionCompatibilityMatrix{
	ServerToClient: map[uint8][]uint8{
		0x01: {0x01}, // v1.0 server supports v1.0 clients
	},
	ClientToServer: map[uint8][]uint8{
		0x01: {0x01}, // v1.0 client supports v1.0 servers
	},
}

// IsVersionSupported checks if a protocol version is supported
func IsVersionSupported(version uint8) bool {
	_, exists := SupportedVersions[version]
	return exists
}

// IsVersionCompatible checks if client and server versions are compatible
func IsVersionCompatible(serverVersion, clientVersion uint8) bool {
	if compatibleClients, exists := DefaultCompatibilityMatrix.ServerToClient[serverVersion]; exists {
		for _, compatibleVersion := range compatibleClients {
			if compatibleVersion == clientVersion {
				return true
			}
		}
	}
	return false
}

// GetVersion returns version information for a given version number
func GetVersion(version uint8) (*Version, error) {
	if v, exists := SupportedVersions[version]; exists {
		return v, nil
	}
	return nil, fmt.Errorf("unsupported version: 0x%02X", version)
}

// GetCurrentVersion returns the current protocol version information
func GetCurrentVersion() *Version {
	return SupportedVersions[CurrentProtocolVersion]
}

// ValidateVersion validates a protocol version and returns detailed error information
func ValidateVersion(version uint8) error {
	// First check if version exists in supported versions (handles deprecated/EOL)
	versionInfo, exists := SupportedVersions[version]
	if exists {
		// Check if version is deprecated
		if versionInfo.Deprecated {
			if versionInfo.EOL != nil && time.Now().After(*versionInfo.EOL) {
				return fmt.Errorf("version 0x%02X (%s) has reached end of life on %s", 
					version, versionInfo.Name, versionInfo.EOL.Format("2006-01-02"))
			}
			// Version is deprecated but still supported
			return fmt.Errorf("version 0x%02X (%s) is deprecated, please upgrade", 
				version, versionInfo.Name)
		}
		return nil // Valid version
	}
	
	// Version not in supported versions, check range for better error messages
	if version < MinSupportedVersion {
		return fmt.Errorf("version 0x%02X is too old (minimum supported: 0x%02X)", 
			version, MinSupportedVersion)
	}
	
	if version > MaxSupportedVersion {
		return fmt.Errorf("version 0x%02X is too new (maximum supported: 0x%02X)", 
			version, MaxSupportedVersion)
	}
	
	return fmt.Errorf("version 0x%02X is not supported", version)
}

// GetVersionFeatures returns the features available for a specific version
func GetVersionFeatures(version uint8) (*VersionFeatures, error) {
	versionInfo, err := GetVersion(version)
	if err != nil {
		return nil, err
	}
	return &versionInfo.Features, nil
}

// HasFeature checks if a specific feature is available in a version
func HasFeature(version uint8, feature string) bool {
	versionInfo, err := GetVersion(version)
	if err != nil {
		return false
	}
	
	features := versionInfo.Features
	switch feature {
	case "authentication":
		return features.Authentication
	case "subscription":
		return features.Subscription
	case "heartbeat":
		return features.Heartbeat
	case "data_batch":
		return features.DataBatch
	case "error_reporting":
		return features.ErrorReporting
	case "crc32_checksum":
		return features.CRC32Checksum
	case "input_validation":
		return features.InputValidation
	case "rate_limiting":
		return features.RateLimiting
	case "compression":
		return features.Compression
	case "tls":
		return features.TLS
	case "async_writes":
		return features.AsyncWrites
	case "object_pooling":
		return features.ObjectPooling
	case "tcp_optimizations":
		return features.TCPOptimizations
	default:
		return false
	}
}

// GetVersionNegotiationResponse determines the best version for communication
func GetVersionNegotiationResponse(clientVersion uint8) (uint8, error) {
	// First check if the client version is supported at all
	if !IsVersionSupported(clientVersion) {
		return 0, fmt.Errorf("no compatible version found for client version 0x%02X", clientVersion)
	}
	
	// Check if client version is directly compatible with server
	if IsVersionCompatible(CurrentProtocolVersion, clientVersion) {
		return clientVersion, nil
	}
	
	// Try to find a compatible version from the compatibility matrix
	if compatibleClients, exists := DefaultCompatibilityMatrix.ServerToClient[CurrentProtocolVersion]; exists {
		for _, compatibleVersion := range compatibleClients {
			if IsVersionSupported(compatibleVersion) && compatibleVersion == clientVersion {
				return compatibleVersion, nil
			}
		}
	}
	
	return 0, fmt.Errorf("no compatible version found for client version 0x%02X", clientVersion)
}

// VersionMetrics tracks version usage statistics
type VersionMetrics struct {
	VersionCounts    map[uint8]int64
	DeprecatedUsage  int64
	UnsupportedAttempts int64
}

// NewVersionMetrics creates a new version metrics tracker
func NewVersionMetrics() *VersionMetrics {
	return &VersionMetrics{
		VersionCounts: make(map[uint8]int64),
	}
}

// RecordVersionUsage records usage of a specific version
func (vm *VersionMetrics) RecordVersionUsage(version uint8) {
	vm.VersionCounts[version]++
	
	if versionInfo, exists := SupportedVersions[version]; exists && versionInfo.Deprecated {
		vm.DeprecatedUsage++
	}
}

// RecordUnsupportedVersion records an attempt to use an unsupported version
func (vm *VersionMetrics) RecordUnsupportedVersion() {
	vm.UnsupportedAttempts++
}

// GetStats returns version usage statistics
func (vm *VersionMetrics) GetStats() map[string]interface{} {
	stats := make(map[string]interface{})
	stats["version_counts"] = vm.VersionCounts
	stats["deprecated_usage"] = vm.DeprecatedUsage
	stats["unsupported_attempts"] = vm.UnsupportedAttempts
	
	// Calculate percentages
	total := int64(0)
	for _, count := range vm.VersionCounts {
		total += count
	}
	
	if total > 0 {
		percentages := make(map[uint8]float64)
		for version, count := range vm.VersionCounts {
			percentages[version] = float64(count) / float64(total) * 100.0
		}
		stats["version_percentages"] = percentages
	}
	
	return stats
}
