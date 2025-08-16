package protocol

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionConstants(t *testing.T) {
	assert.Equal(t, uint8(0x01), uint8(CurrentProtocolVersion))
	assert.Equal(t, uint8(0x01), uint8(MinSupportedVersion))
	assert.Equal(t, uint8(0x01), uint8(MaxSupportedVersion))
}

func TestIsVersionSupported(t *testing.T) {
	tests := []struct {
		name     string
		version  uint8
		expected bool
	}{
		{"current version", 0x01, true},
		{"unsupported version", 0x02, false},
		{"zero version", 0x00, false},
		{"high version", 0xFF, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsVersionSupported(tt.version)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsVersionCompatible(t *testing.T) {
	tests := []struct {
		name          string
		serverVersion uint8
		clientVersion uint8
		expected      bool
	}{
		{"same version", 0x01, 0x01, true},
		{"unsupported client", 0x01, 0x02, false},
		{"unsupported server", 0x02, 0x01, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsVersionCompatible(tt.serverVersion, tt.clientVersion)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetVersion(t *testing.T) {
	t.Run("valid version", func(t *testing.T) {
		version, err := GetVersion(0x01)
		require.NoError(t, err)
		assert.Equal(t, uint8(0x01), version.Number)
		assert.Equal(t, "v1.0", version.Name)
		assert.False(t, version.Deprecated)
	})

	t.Run("invalid version", func(t *testing.T) {
		version, err := GetVersion(0x99)
		assert.Error(t, err)
		assert.Nil(t, version)
		assert.Contains(t, err.Error(), "unsupported version: 0x99")
	})
}

func TestGetCurrentVersion(t *testing.T) {
	version := GetCurrentVersion()
	require.NotNil(t, version)
	assert.Equal(t, CurrentProtocolVersion, version.Number)
}

func TestValidateVersion(t *testing.T) {
	tests := []struct {
		name        string
		version     uint8
		expectError bool
		errorMsg    string
	}{
		{"valid current version", 0x01, false, ""},
		{"too old version", 0x00, true, "too old"},
		{"too new version", 0x02, true, "too new"},
		{"unsupported version", 0xFF, true, "too new"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateVersion(tt.version)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetVersionFeatures(t *testing.T) {
	t.Run("valid version", func(t *testing.T) {
		features, err := GetVersionFeatures(0x01)
		require.NoError(t, err)
		
		// Check core features
		assert.True(t, features.Authentication)
		assert.True(t, features.Subscription)
		assert.True(t, features.Heartbeat)
		assert.True(t, features.DataBatch)
		assert.True(t, features.ErrorReporting)
		
		// Check advanced features
		assert.True(t, features.CRC32Checksum)
		assert.True(t, features.InputValidation)
		assert.True(t, features.RateLimiting)
		assert.False(t, features.Compression) // Not implemented yet
		assert.False(t, features.TLS)         // Not implemented yet
		
		// Check performance features
		assert.True(t, features.AsyncWrites)
		assert.True(t, features.ObjectPooling)
		assert.True(t, features.TCPOptimizations)
	})

	t.Run("invalid version", func(t *testing.T) {
		features, err := GetVersionFeatures(0x99)
		assert.Error(t, err)
		assert.Nil(t, features)
	})
}

func TestHasFeature(t *testing.T) {
	tests := []struct {
		name     string
		version  uint8
		feature  string
		expected bool
	}{
		{"authentication v1", 0x01, "authentication", true},
		{"compression v1", 0x01, "compression", false},
		{"tls v1", 0x01, "tls", false},
		{"async_writes v1", 0x01, "async_writes", true},
		{"unknown feature", 0x01, "unknown", false},
		{"invalid version", 0x99, "authentication", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasFeature(tt.version, tt.feature)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetVersionNegotiationResponse(t *testing.T) {
	tests := []struct {
		name          string
		clientVersion uint8
		expectedVer   uint8
		expectError   bool
	}{
		{"compatible version", 0x01, 0x01, false},
		{"unsupported version", 0x02, 0, true},
		{"too old version", 0x00, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, err := GetVersionNegotiationResponse(tt.clientVersion)
			if tt.expectError {
				assert.Error(t, err)
				assert.Equal(t, uint8(0), version)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedVer, version)
			}
		})
	}
}

func TestVersionMetrics(t *testing.T) {
	metrics := NewVersionMetrics()

	t.Run("initial state", func(t *testing.T) {
		stats := metrics.GetStats()
		assert.Equal(t, int64(0), stats["deprecated_usage"])
		assert.Equal(t, int64(0), stats["unsupported_attempts"])
		assert.Empty(t, stats["version_counts"])
	})

	t.Run("record version usage", func(t *testing.T) {
		metrics.RecordVersionUsage(0x01)
		metrics.RecordVersionUsage(0x01)
		metrics.RecordVersionUsage(0x01)

		stats := metrics.GetStats()
		versionCounts := stats["version_counts"].(map[uint8]int64)
		assert.Equal(t, int64(3), versionCounts[0x01])
		
		// Check percentages
		percentages := stats["version_percentages"].(map[uint8]float64)
		assert.Equal(t, 100.0, percentages[0x01])
	})

	t.Run("record unsupported version", func(t *testing.T) {
		metrics.RecordUnsupportedVersion()
		metrics.RecordUnsupportedVersion()

		stats := metrics.GetStats()
		assert.Equal(t, int64(2), stats["unsupported_attempts"])
	})
}

func TestVersionCompatibilityMatrix(t *testing.T) {
	matrix := DefaultCompatibilityMatrix

	t.Run("server to client compatibility", func(t *testing.T) {
		compatibleClients, exists := matrix.ServerToClient[0x01]
		assert.True(t, exists)
		assert.Contains(t, compatibleClients, uint8(0x01))
	})

	t.Run("client to server compatibility", func(t *testing.T) {
		compatibleServers, exists := matrix.ClientToServer[0x01]
		assert.True(t, exists)
		assert.Contains(t, compatibleServers, uint8(0x01))
	})
}

func TestVersionDeprecation(t *testing.T) {
	// Test with a hypothetical deprecated version
	deprecatedVersion := &Version{
		Number:      0x00,
		Name:        "v0.9",
		ReleaseDate: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		Features:    VersionFeatures{Authentication: true},
		Deprecated:  true,
		EOL:         nil, // Not yet EOL
	}

	// Temporarily add to supported versions for testing
	SupportedVersions[0x00] = deprecatedVersion
	defer delete(SupportedVersions, 0x00)

	err := ValidateVersion(0x00)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "deprecated")
}

func TestVersionEOL(t *testing.T) {
	// Test with a version that has reached EOL
	eolDate := time.Now().Add(-24 * time.Hour) // Yesterday
	eolVersion := &Version{
		Number:      0x00,
		Name:        "v0.8",
		ReleaseDate: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
		Features:    VersionFeatures{Authentication: true},
		Deprecated:  true,
		EOL:         &eolDate,
	}

	// Temporarily add to supported versions for testing
	SupportedVersions[0x00] = eolVersion
	defer delete(SupportedVersions, 0x00)

	err := ValidateVersion(0x00)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "end of life")
}

func BenchmarkVersionValidation(b *testing.B) {
	b.Run("ValidateVersion", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ValidateVersion(0x01)
		}
	})

	b.Run("IsVersionSupported", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			IsVersionSupported(0x01)
		}
	})

	b.Run("HasFeature", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			HasFeature(0x01, "authentication")
		}
	})
}

func BenchmarkVersionMetrics(b *testing.B) {
	metrics := NewVersionMetrics()
	
	b.Run("RecordVersionUsage", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			metrics.RecordVersionUsage(0x01)
		}
	})

	b.Run("GetStats", func(b *testing.B) {
		// Pre-populate some data
		for i := 0; i < 1000; i++ {
			metrics.RecordVersionUsage(0x01)
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			metrics.GetStats()
		}
	})
}
