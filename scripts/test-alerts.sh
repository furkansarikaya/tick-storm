#!/bin/bash

# TickStorm Alert Testing Script
# This script tests various alert conditions to verify alerting functionality

set -e

TICKSTORM_HOST=${TICKSTORM_HOST:-"localhost:8080"}
METRICS_HOST=${METRICS_HOST:-"localhost:9090"}
PROMETHEUS_HOST=${PROMETHEUS_HOST:-"localhost:9090"}

echo "=== TickStorm Alert Testing Script ==="
echo "TickStorm Host: $TICKSTORM_HOST"
echo "Metrics Host: $METRICS_HOST"
echo "Prometheus Host: $PROMETHEUS_HOST"
echo ""

# Function to check if service is running
check_service() {
    local host=$1
    local service_name=$2
    
    echo "Checking $service_name connectivity..."
    if curl -f -s "http://$host/health" > /dev/null 2>&1 || curl -f -s "http://$host/metrics" > /dev/null 2>&1; then
        echo "✅ $service_name is accessible"
        return 0
    else
        echo "❌ $service_name is not accessible"
        return 1
    fi
}

# Function to get current metric value
get_metric() {
    local metric_name=$1
    curl -s "http://$METRICS_HOST/metrics" | grep "^$metric_name" | head -1 | awk '{print $2}' || echo "0"
}

# Function to test alert condition
test_alert_condition() {
    local alert_name=$1
    local condition=$2
    local expected_result=$3
    
    echo "Testing alert: $alert_name"
    echo "Condition: $condition"
    
    # Query Prometheus for alert status
    local query_url="http://$PROMETHEUS_HOST/api/v1/query?query=ALERTS{alertname=\"$alert_name\"}"
    local alert_status=$(curl -s "$query_url" | jq -r '.data.result[0].metric.alertstate // "inactive"' 2>/dev/null || echo "inactive")
    
    echo "Current status: $alert_status"
    echo "Expected: $expected_result"
    
    if [[ "$alert_status" == "$expected_result" ]]; then
        echo "✅ Alert test passed"
    else
        echo "⚠️  Alert test result differs from expected"
    fi
    echo ""
}

# Function to simulate load for testing
simulate_connections() {
    local count=${1:-10}
    echo "Simulating $count concurrent connections..."
    
    for i in $(seq 1 $count); do
        (
            timeout 5s telnet $TICKSTORM_HOST > /dev/null 2>&1 || true
        ) &
    done
    
    wait
    echo "Connection simulation completed"
}

# Main testing sequence
main() {
    echo "=== Pre-test Service Checks ==="
    check_service "$TICKSTORM_HOST" "TickStorm"
    check_service "$METRICS_HOST" "Metrics"
    check_service "$PROMETHEUS_HOST" "Prometheus"
    echo ""
    
    echo "=== Current Metrics Snapshot ==="
    echo "Active Connections: $(get_metric 'tick_storm_active_connections')"
    echo "Total Connections: $(get_metric 'tick_storm_total_connections_total')"
    echo "Auth Successes: $(get_metric 'tick_storm_auth_success_total')"
    echo "Auth Failures: $(get_metric 'tick_storm_auth_failures_total')"
    echo "Memory Usage: $(get_metric 'tick_storm_memory_usage_bytes')"
    echo "Goroutines: $(get_metric 'tick_storm_goroutines')"
    echo ""
    
    echo "=== Alert Condition Tests ==="
    
    # Test heartbeat timeout alert
    test_alert_condition "TickStormHeartbeatTimeoutRate" \
        "rate(tick_storm_heartbeat_timeouts_total[5m]) / rate(tick_storm_heartbeats_recv_total[5m]) * 100 > 1" \
        "inactive"
    
    # Test high latency alert
    test_alert_condition "TickStormPublishLatencyHigh" \
        "histogram_quantile(0.95, rate(tick_storm_publish_latency_seconds_bucket[5m])) * 1000 > 5" \
        "inactive"
    
    # Test auth failure rate alert
    test_alert_condition "TickStormAuthFailureRate" \
        "rate(tick_storm_auth_failures_total[5m]) / (rate(tick_storm_auth_success_total[5m]) + rate(tick_storm_auth_failures_total[5m])) * 100 > 5" \
        "inactive"
    
    # Test memory usage alert
    test_alert_condition "TickStormMemoryUsageHigh" \
        "tick_storm_memory_usage_bytes / (1024 * 1024 * 1024) > 0.8" \
        "inactive"
    
    # Test connection drop rate alert
    test_alert_condition "TickStormConnectionDropRate" \
        "rate(tick_storm_connection_errors_total{error_type=\"connection_dropped\"}[5m]) / rate(tick_storm_total_connections_total[5m]) * 100 > 0.5" \
        "inactive"
    
    # Test service down alert
    test_alert_condition "TickStormServiceDown" \
        "tick_storm_active_connections == 0 and rate(tick_storm_total_connections_total[5m]) == 0" \
        "inactive"
    
    echo "=== Load Testing for Alert Triggers ==="
    echo "Generating load to test alert responsiveness..."
    simulate_connections 50
    
    echo "Waiting 30 seconds for metrics to update..."
    sleep 30
    
    echo "=== Post-Load Metrics ==="
    echo "Active Connections: $(get_metric 'tick_storm_active_connections')"
    echo "Total Connections: $(get_metric 'tick_storm_total_connections_total')"
    echo ""
    
    echo "=== Alert Rule Validation ==="
    echo "Checking if Prometheus has loaded alert rules..."
    
    local rules_url="http://$PROMETHEUS_HOST/api/v1/rules"
    local rule_count=$(curl -s "$rules_url" | jq '.data.groups[] | select(.name | contains("tickstorm")) | .rules | length' 2>/dev/null | paste -sd+ | bc 2>/dev/null || echo "0")
    
    echo "Loaded TickStorm alert rules: $rule_count"
    
    if [[ "$rule_count" -gt 0 ]]; then
        echo "✅ Alert rules are loaded in Prometheus"
    else
        echo "❌ No TickStorm alert rules found in Prometheus"
        echo "Please verify prometheus-rules.yml is loaded correctly"
    fi
    
    echo ""
    echo "=== Alertmanager Configuration Test ==="
    local alertmanager_url="http://localhost:9093/api/v1/status"
    if curl -f -s "$alertmanager_url" > /dev/null 2>&1; then
        echo "✅ Alertmanager is accessible"
        
        # Check configuration
        local config_url="http://localhost:9093/api/v1/status"
        local config_status=$(curl -s "$config_url" | jq -r '.status' 2>/dev/null || echo "unknown")
        echo "Alertmanager status: $config_status"
    else
        echo "❌ Alertmanager is not accessible"
        echo "Please ensure Alertmanager is running on port 9093"
    fi
    
    echo ""
    echo "=== Test Summary ==="
    echo "Alert testing completed. Review the results above."
    echo "For active alerts, check: http://$PROMETHEUS_HOST/alerts"
    echo "For alert routing, check: http://localhost:9093"
    echo ""
    echo "To manually trigger alerts for testing:"
    echo "1. Stop TickStorm service temporarily (ServiceDown alert)"
    echo "2. Generate high load (various performance alerts)"
    echo "3. Use invalid credentials (AuthFailure alert)"
    echo "4. Simulate network issues (ConnectionDrop alert)"
}

# Run main function
main "$@"
