// Package server implements OS-level resource constraints and ulimit management.
package server

import (
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strconv"
	"syscall"
)

// ResourceConstraints manages OS-level resource limits
type ResourceConstraints struct {
	logger *slog.Logger
}

// UlimitConfig holds ulimit configuration values
type UlimitConfig struct {
	// File descriptor limits
	MaxOpenFiles     uint64 // RLIMIT_NOFILE
	MaxOpenFilesSoft uint64 // Soft limit for open files
	
	// Memory limits
	MaxMemorySize     uint64 // RLIMIT_AS (virtual memory)
	MaxDataSize       uint64 // RLIMIT_DATA
	MaxStackSize      uint64 // RLIMIT_STACK
	
	// Process limits
	MaxProcesses      uint64 // RLIMIT_NPROC
	MaxCoreSize       uint64 // RLIMIT_CORE
	
	// CPU limits
	MaxCPUTime        uint64 // RLIMIT_CPU (seconds)
	
	// Network limits
	MaxLockedMemory   uint64 // RLIMIT_MEMLOCK
}

// NewResourceConstraints creates a new resource constraints manager
func NewResourceConstraints() *ResourceConstraints {
	return &ResourceConstraints{
		logger: slog.Default().With("component", "resource_constraints"),
	}
}

// LoadConfigFromEnv loads ulimit configuration from environment variables
func (rc *ResourceConstraints) LoadConfigFromEnv() *UlimitConfig {
	config := &UlimitConfig{
		MaxOpenFiles:     65536,  // Default: 64K file descriptors
		MaxOpenFilesSoft: 32768,  // Default: 32K soft limit
		MaxMemorySize:    0,      // 0 = unlimited
		MaxDataSize:      0,      // 0 = unlimited
		MaxStackSize:     8388608, // Default: 8MB stack
		MaxProcesses:     32768,  // Default: 32K processes
		MaxCoreSize:      0,      // Default: no core dumps
		MaxCPUTime:       0,      // 0 = unlimited CPU time
		MaxLockedMemory:  65536,  // Default: 64KB locked memory
	}
	
	// Load from environment variables
	if val := os.Getenv("ULIMIT_MAX_OPEN_FILES"); val != "" {
		if parsed, err := strconv.ParseUint(val, 10, 64); err == nil {
			config.MaxOpenFiles = parsed
		}
	}
	
	if val := os.Getenv("ULIMIT_MAX_OPEN_FILES_SOFT"); val != "" {
		if parsed, err := strconv.ParseUint(val, 10, 64); err == nil {
			config.MaxOpenFilesSoft = parsed
		}
	}
	
	if val := os.Getenv("ULIMIT_MAX_MEMORY_SIZE"); val != "" {
		if parsed, err := strconv.ParseUint(val, 10, 64); err == nil {
			config.MaxMemorySize = parsed
		}
	}
	
	if val := os.Getenv("ULIMIT_MAX_DATA_SIZE"); val != "" {
		if parsed, err := strconv.ParseUint(val, 10, 64); err == nil {
			config.MaxDataSize = parsed
		}
	}
	
	if val := os.Getenv("ULIMIT_MAX_STACK_SIZE"); val != "" {
		if parsed, err := strconv.ParseUint(val, 10, 64); err == nil {
			config.MaxStackSize = parsed
		}
	}
	
	if val := os.Getenv("ULIMIT_MAX_PROCESSES"); val != "" {
		if parsed, err := strconv.ParseUint(val, 10, 64); err == nil {
			config.MaxProcesses = parsed
		}
	}
	
	if val := os.Getenv("ULIMIT_MAX_CORE_SIZE"); val != "" {
		if parsed, err := strconv.ParseUint(val, 10, 64); err == nil {
			config.MaxCoreSize = parsed
		}
	}
	
	if val := os.Getenv("ULIMIT_MAX_CPU_TIME"); val != "" {
		if parsed, err := strconv.ParseUint(val, 10, 64); err == nil {
			config.MaxCPUTime = parsed
		}
	}
	
	if val := os.Getenv("ULIMIT_MAX_LOCKED_MEMORY"); val != "" {
		if parsed, err := strconv.ParseUint(val, 10, 64); err == nil {
			config.MaxLockedMemory = parsed
		}
	}
	
	return config
}

// ApplyResourceLimits applies OS-level resource limits using syscalls
func (rc *ResourceConstraints) ApplyResourceLimits(config *UlimitConfig) error {
	rc.logger.Info("applying OS-level resource limits",
		"max_open_files", config.MaxOpenFiles,
		"max_memory_size", config.MaxMemorySize,
		"max_processes", config.MaxProcesses,
	)
	
	// Set file descriptor limits
	if err := rc.setRlimit(syscall.RLIMIT_NOFILE, config.MaxOpenFilesSoft, config.MaxOpenFiles); err != nil {
		return fmt.Errorf("failed to set file descriptor limit: %w", err)
	}
	
	// Set virtual memory limit
	if config.MaxMemorySize > 0 {
		if err := rc.setRlimit(syscall.RLIMIT_AS, config.MaxMemorySize, config.MaxMemorySize); err != nil {
			rc.logger.Warn("failed to set virtual memory limit", "error", err)
		}
	}
	
	// Set data segment size limit
	if config.MaxDataSize > 0 {
		if err := rc.setRlimit(syscall.RLIMIT_DATA, config.MaxDataSize, config.MaxDataSize); err != nil {
			rc.logger.Warn("failed to set data segment limit", "error", err)
		}
	}
	
	// Set stack size limit
	if config.MaxStackSize > 0 {
		if err := rc.setRlimit(syscall.RLIMIT_STACK, config.MaxStackSize, config.MaxStackSize); err != nil {
			rc.logger.Warn("failed to set stack size limit", "error", err)
		}
	}
	
	// Set process limit (not available on all platforms)
	// if config.MaxProcesses > 0 {
	//     if err := rc.setRlimit(syscall.RLIMIT_NPROC, config.MaxProcesses, config.MaxProcesses); err != nil {
	//         rc.logger.Warn("failed to set process limit", "error", err)
	//     }
	// }
	
	// Set core dump size limit
	if err := rc.setRlimit(syscall.RLIMIT_CORE, config.MaxCoreSize, config.MaxCoreSize); err != nil {
		rc.logger.Warn("failed to set core dump limit", "error", err)
	}
	
	// Set CPU time limit
	if config.MaxCPUTime > 0 {
		if err := rc.setRlimit(syscall.RLIMIT_CPU, config.MaxCPUTime, config.MaxCPUTime); err != nil {
			rc.logger.Warn("failed to set CPU time limit", "error", err)
		}
	}
	
	// Set locked memory limit (not available on all platforms)
	// if config.MaxLockedMemory > 0 {
	//     if err := rc.setRlimit(syscall.RLIMIT_MEMLOCK, config.MaxLockedMemory, config.MaxLockedMemory); err != nil {
	//         rc.logger.Warn("failed to set locked memory limit", "error", err)
	//     }
	// }
	
	rc.logger.Info("resource limits applied successfully")
	return nil
}

// setRlimit sets a resource limit using syscall
func (rc *ResourceConstraints) setRlimit(resource int, soft, hard uint64) error {
	rLimit := syscall.Rlimit{
		Cur: soft,
		Max: hard,
	}
	
	return syscall.Setrlimit(resource, &rLimit)
}

// GetCurrentLimits returns current OS-level resource limits
func (rc *ResourceConstraints) GetCurrentLimits() (map[string]syscall.Rlimit, error) {
	limits := make(map[string]syscall.Rlimit)
	
	resources := map[string]int{
		"RLIMIT_NOFILE": syscall.RLIMIT_NOFILE,
		"RLIMIT_AS":     syscall.RLIMIT_AS,
		"RLIMIT_DATA":   syscall.RLIMIT_DATA,
		"RLIMIT_STACK":  syscall.RLIMIT_STACK,
		"RLIMIT_CORE":   syscall.RLIMIT_CORE,
		"RLIMIT_CPU":    syscall.RLIMIT_CPU,
		// Note: RLIMIT_NPROC and RLIMIT_MEMLOCK not available on all platforms
	}
	
	for name, resource := range resources {
		var rLimit syscall.Rlimit
		if err := syscall.Getrlimit(resource, &rLimit); err != nil {
			return nil, fmt.Errorf("failed to get %s: %w", name, err)
		}
		limits[name] = rLimit
	}
	
	return limits, nil
}

// LogCurrentLimits logs current resource limits for debugging
func (rc *ResourceConstraints) LogCurrentLimits() {
	limits, err := rc.GetCurrentLimits()
	if err != nil {
		rc.logger.Error("failed to get current limits", "error", err)
		return
	}
	
	for name, limit := range limits {
		rc.logger.Info("current resource limit",
			"resource", name,
			"soft_limit", limit.Cur,
			"hard_limit", limit.Max,
		)
	}
}

// SetGoRuntimeLimits configures Go runtime limits
func (rc *ResourceConstraints) SetGoRuntimeLimits() {
	// Set GOMAXPROCS based on available CPUs
	if maxProcs := os.Getenv("GOMAXPROCS"); maxProcs == "" {
		numCPU := runtime.NumCPU()
		runtime.GOMAXPROCS(numCPU)
		rc.logger.Info("set GOMAXPROCS", "value", numCPU)
	}
	
	// Configure Go runtime parameters
	if gcPercent := os.Getenv("GOGC"); gcPercent != "" {
		if percent, err := strconv.Atoi(gcPercent); err == nil {
			// Note: runtime.SetGCPercent removed in newer Go versions
			// Use GOGC environment variable instead
			slog.Info("GC percentage configured via GOGC", "percent", percent)
		}
	}
	
	// Set memory limit for Go runtime (Go 1.19+)
	if memLimit := os.Getenv("GOMEMLIMIT"); memLimit != "" {
		rc.logger.Info("Go memory limit set via GOMEMLIMIT", "value", memLimit)
	}
}

// ValidateResourceLimits validates that resource limits are reasonable
func (rc *ResourceConstraints) ValidateResourceLimits(config *UlimitConfig) error {
	// Validate file descriptor limits
	if config.MaxOpenFilesSoft > config.MaxOpenFiles {
		return fmt.Errorf("soft file descriptor limit (%d) cannot exceed hard limit (%d)",
			config.MaxOpenFilesSoft, config.MaxOpenFiles)
	}
	
	// Validate minimum file descriptor requirements
	minFDs := uint64(1024)
	if config.MaxOpenFiles < minFDs {
		return fmt.Errorf("file descriptor limit (%d) is too low, minimum required: %d",
			config.MaxOpenFiles, minFDs)
	}
	
	// Validate stack size
	minStack := uint64(1024 * 1024) // 1MB minimum
	if config.MaxStackSize > 0 && config.MaxStackSize < minStack {
		return fmt.Errorf("stack size limit (%d) is too low, minimum required: %d",
			config.MaxStackSize, minStack)
	}
	
	// Validate process limits
	if config.MaxProcesses > 0 && config.MaxProcesses < 100 {
		return fmt.Errorf("process limit (%d) is too low, minimum required: 100",
			config.MaxProcesses)
	}
	
	return nil
}

// GetResourceUsageStats returns current resource usage statistics
func (rc *ResourceConstraints) GetResourceUsageStats() map[string]interface{} {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	stats := map[string]interface{}{
		"goroutines":           runtime.NumGoroutine(),
		"memory_alloc_mb":      m.Alloc / 1024 / 1024,
		"memory_sys_mb":        m.Sys / 1024 / 1024,
		"memory_heap_mb":       m.HeapAlloc / 1024 / 1024,
		"memory_stack_mb":      m.StackInuse / 1024 / 1024,
		"gc_cycles":            m.NumGC,
		"gc_pause_total_ns":    m.PauseTotalNs,
		"gc_pause_last_ns":     m.PauseNs[(m.NumGC+255)%256],
		"num_cpu":              runtime.NumCPU(),
		"gomaxprocs":           runtime.GOMAXPROCS(0),
	}
	
	// Add OS-level stats if available
	if limits, err := rc.GetCurrentLimits(); err == nil {
		for name, limit := range limits {
			stats[fmt.Sprintf("limit_%s_soft", name)] = limit.Cur
			stats[fmt.Sprintf("limit_%s_hard", name)] = limit.Max
		}
	}
	
	return stats
}

// CheckResourceHealth performs health checks on resource usage
func (rc *ResourceConstraints) CheckResourceHealth() []string {
	var issues []string
	
	// Check goroutine count
	numGoroutines := runtime.NumGoroutine()
	if numGoroutines > 10000 {
		issues = append(issues, fmt.Sprintf("High goroutine count: %d", numGoroutines))
	}
	
	// Check memory usage
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	heapMB := m.HeapAlloc / 1024 / 1024
	if heapMB > 1024 { // More than 1GB heap
		issues = append(issues, fmt.Sprintf("High heap memory usage: %d MB", heapMB))
	}
	
	// Check GC pressure
	if m.NumGC > 0 {
		avgPause := m.PauseTotalNs / uint64(m.NumGC)
		if avgPause > 10*1000*1000 { // More than 10ms average GC pause
			issues = append(issues, fmt.Sprintf("High GC pause time: %d ns average", avgPause))
		}
	}
	
	return issues
}
