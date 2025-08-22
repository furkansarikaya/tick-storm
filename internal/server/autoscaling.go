// Package server implements auto-scaling integration support for Kubernetes HPA and cloud providers.
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
	"time"
)

const (
	contentTypeJSON = "application/json"
	contentTypeText = "text/plain"
)

// AutoScalingConfig contains configuration for auto-scaling integration
type AutoScalingConfig struct {
	Enabled                bool    `json:"enabled"`
	MetricsPort           int     `json:"metrics_port"`
	ScaleUpThreshold      float64 `json:"scale_up_threshold"`
	ScaleDownThreshold    float64 `json:"scale_down_threshold"`
	ConnectionsPerInstance int     `json:"connections_per_instance"`
	CPUTargetPercent      int     `json:"cpu_target_percent"`
	MemoryTargetPercent   int     `json:"memory_target_percent"`
}

// AutoScalingMetrics contains metrics for auto-scaling decisions
type AutoScalingMetrics struct {
	InstanceID              string  `json:"instance_id"`
	ActiveConnections       int32   `json:"active_connections"`
	ConnectionUtilization   float64 `json:"connection_utilization"`
	CPUUtilization         float64 `json:"cpu_utilization"`
	MemoryUtilization      float64 `json:"memory_utilization"`
	RequestRate            float64 `json:"request_rate"`
	ErrorRate              float64 `json:"error_rate"`
	RecommendedReplicas    int     `json:"recommended_replicas"`
	ScaleAction            string  `json:"scale_action"`
	Timestamp              string  `json:"timestamp"`
}

// initAutoScaling initializes auto-scaling support
func (s *Server) initAutoScaling() {
	config := s.getAutoScalingConfig()
	if !config.Enabled {
		return
	}

	s.logger.Info("initializing auto-scaling support", "config", config)
	
	// Start metrics server for HPA custom metrics
	go s.startAutoScalingMetricsServer(config.MetricsPort)
}

// getAutoScalingConfig loads auto-scaling configuration from environment
func (s *Server) getAutoScalingConfig() AutoScalingConfig {
	config := AutoScalingConfig{
		Enabled:                getEnvBool("AUTOSCALING_ENABLED", false),
		MetricsPort:           getEnvInt("AUTOSCALING_METRICS_PORT", 9090),
		ScaleUpThreshold:      getEnvFloat("AUTOSCALING_SCALE_UP_THRESHOLD", 0.8),
		ScaleDownThreshold:    getEnvFloat("AUTOSCALING_SCALE_DOWN_THRESHOLD", 0.3),
		ConnectionsPerInstance: getEnvInt("AUTOSCALING_CONNECTIONS_PER_INSTANCE", 80000),
		CPUTargetPercent:      getEnvInt("AUTOSCALING_CPU_TARGET", 70),
		MemoryTargetPercent:   getEnvInt("AUTOSCALING_MEMORY_TARGET", 80),
	}
	
	return config
}

// startAutoScalingMetricsServer starts HTTP server for auto-scaling metrics
func (s *Server) startAutoScalingMetricsServer(port int) {
	mux := http.NewServeMux()
	
	// Prometheus-style metrics endpoint
	mux.HandleFunc("/metrics", s.handlePrometheusMetrics)
	
	// Custom metrics for HPA
	mux.HandleFunc("/autoscaling/metrics", s.handleAutoScalingMetrics)
	
	// Scale recommendations endpoint
	mux.HandleFunc("/autoscaling/recommendations", s.handleScaleRecommendations)
	
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}
	
	s.logger.Info("starting auto-scaling metrics server", "port", port)
	
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		s.logger.Error("auto-scaling metrics server failed", "error", err)
	}
}

// handlePrometheusMetrics serves Prometheus-compatible metrics
func (s *Server) handlePrometheusMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", contentTypeJSON)
	
	instanceMetrics := s.GetInstanceMetrics()
	
	fmt.Fprintf(w, "# HELP tick_storm_active_connections Current number of active connections\n")
	fmt.Fprintf(w, "# TYPE tick_storm_active_connections gauge\n")
	fmt.Fprintf(w, "tick_storm_active_connections{instance_id=\"%s\"} %d\n", 
		s.instanceID, atomic.LoadInt32(&s.activeConns))
	
	fmt.Fprintf(w, "# HELP tick_storm_total_connections Total number of connections handled\n")
	fmt.Fprintf(w, "# TYPE tick_storm_total_connections counter\n")
	fmt.Fprintf(w, "tick_storm_total_connections{instance_id=\"%s\"} %d\n", 
		s.instanceID, atomic.LoadUint64(&s.totalConns))
	
	fmt.Fprintf(w, "# HELP tick_storm_memory_usage_bytes Memory usage in bytes\n")
	fmt.Fprintf(w, "# TYPE tick_storm_memory_usage_bytes gauge\n")
	fmt.Fprintf(w, "tick_storm_memory_usage_bytes{instance_id=\"%s\"} %d\n", 
		s.instanceID, int64(instanceMetrics["memory_alloc_mb"].(uint64)*1024*1024))
	
	fmt.Fprintf(w, "# HELP tick_storm_goroutines Number of goroutines\n")
	fmt.Fprintf(w, "# TYPE tick_storm_goroutines gauge\n")
	fmt.Fprintf(w, "tick_storm_goroutines{instance_id=\"%s\"} %d\n", 
		s.instanceID, instanceMetrics["goroutines"])
	
	fmt.Fprintf(w, "# HELP tick_storm_uptime_seconds Server uptime in seconds\n")
	fmt.Fprintf(w, "# TYPE tick_storm_uptime_seconds gauge\n")
	fmt.Fprintf(w, "tick_storm_uptime_seconds{instance_id=\"%s\"} %.2f\n", 
		s.instanceID, instanceMetrics["uptime_seconds"])
}

// handleAutoScalingMetrics serves custom metrics for HPA
func (s *Server) handleAutoScalingMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", contentTypeJSON)
	
	metrics := s.calculateAutoScalingMetrics()
	
	if err := json.NewEncoder(w).Encode(metrics); err != nil {
		http.Error(w, "Failed to encode metrics", http.StatusInternalServerError)
		return
	}
}

// handleScaleRecommendations provides scaling recommendations
func (s *Server) handleScaleRecommendations(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", contentTypeJSON)
	
	config := s.getAutoScalingConfig()
	metrics := s.calculateAutoScalingMetrics()
	
	recommendation := s.calculateScaleRecommendation(config, metrics)
	
	if err := json.NewEncoder(w).Encode(recommendation); err != nil {
		http.Error(w, "Failed to encode recommendation", http.StatusInternalServerError)
		return
	}
}

// calculateAutoScalingMetrics calculates current metrics for auto-scaling
func (s *Server) calculateAutoScalingMetrics() AutoScalingMetrics {
	config := s.getAutoScalingConfig()
	activeConns := atomic.LoadInt32(&s.activeConns)
	maxConns := config.ConnectionsPerInstance
	connectionUtilization := float64(activeConns) / float64(maxConns)

	instanceMetrics := s.GetInstanceMetrics()
	memoryUtil := float64(instanceMetrics["memory_alloc_mb"].(uint64)) / 1024.0 // Assume 1GB limit

	// Calculate request rate (connections per second over last minute)
	requestRate := s.calculateRequestRate()

	// Calculate error rate
	errorRate := s.calculateErrorRate()
	
	return AutoScalingMetrics{
		InstanceID:            s.instanceID,
		ActiveConnections:     activeConns,
		ConnectionUtilization: connectionUtilization,
		CPUUtilization:       0.0, // Would need OS-level monitoring
		MemoryUtilization:    memoryUtil,
		RequestRate:          requestRate,
		ErrorRate:            errorRate,
		Timestamp:            time.Now().Format(time.RFC3339),
	}
}

// calculateScaleRecommendation determines if scaling is needed
func (s *Server) calculateScaleRecommendation(config AutoScalingConfig, metrics AutoScalingMetrics) map[string]interface{} {
	recommendation := map[string]interface{}{
		"instance_id": s.instanceID,
		"timestamp":   time.Now().Format(time.RFC3339),
		"metrics":     metrics,
	}
	
	// Determine scale action based on thresholds
	if metrics.ConnectionUtilization > config.ScaleUpThreshold {
		recommendation["action"] = "scale_up"
		recommendation["reason"] = "connection_utilization_high"
		recommendation["recommended_replicas"] = s.calculateTargetReplicas(metrics, config, "up")
	} else if metrics.ConnectionUtilization < config.ScaleDownThreshold {
		recommendation["action"] = "scale_down"
		recommendation["reason"] = "connection_utilization_low"
		recommendation["recommended_replicas"] = s.calculateTargetReplicas(metrics, config, "down")
	} else {
		recommendation["action"] = "no_action"
		recommendation["reason"] = "utilization_within_target"
		recommendation["recommended_replicas"] = 1 // Current instance
	}
	
	return recommendation
}

// calculateTargetReplicas calculates target number of replicas
func (s *Server) calculateTargetReplicas(metrics AutoScalingMetrics, config AutoScalingConfig, direction string) int {
	currentConnections := float64(metrics.ActiveConnections)
	connectionsPerInstance := float64(config.ConnectionsPerInstance)
	
	// Calculate required replicas based on current load
	requiredReplicas := int(currentConnections / (connectionsPerInstance * 0.7)) // Target 70% utilization
	
	if requiredReplicas < 1 {
		requiredReplicas = 1
	}
	
	// Apply scaling constraints
	if direction == "up" {
		return requiredReplicas + 1 // Add buffer for scale up
	}
	
	return requiredReplicas
}

// calculateRequestRate calculates recent request rate
func (s *Server) calculateRequestRate() float64 {
	// This would need historical data tracking
	// For now, return a placeholder based on current connections
	totalConns := atomic.LoadUint64(&s.totalConns)
	return float64(totalConns) / time.Since(s.startTime).Seconds()
}

// calculateErrorRate calculates the current error rate
func (s *Server) calculateErrorRate() float64 {
	totalAuth := atomic.LoadUint64(&s.authSuccess) + atomic.LoadUint64(&s.authFailures) + atomic.LoadUint64(&s.authRateLimited)
	if totalAuth == 0 {
		return 0.0
	}
	
	errors := atomic.LoadUint64(&s.authFailures) + atomic.LoadUint64(&s.authRateLimited)
	return float64(errors) / float64(totalAuth)
}

// GetAutoScalingStatus returns current auto-scaling status
func (s *Server) getServerStats() map[string]interface{} {
	config := s.getAutoScalingConfig()
	metrics := s.calculateAutoScalingMetrics()
	recommendation := s.calculateScaleRecommendation(config, metrics)
	
	return map[string]interface{}{
		"enabled":        config.Enabled,
		"config":         config,
		"current_metrics": metrics,
		"recommendation": recommendation,
	}
}

// Helper functions for environment variable parsing
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseFloat(value, 64); err == nil {
			return parsed
		}
	}
	return defaultValue
}
