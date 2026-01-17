package v2

import (
	"testing"
	"time"
)

func TestHealthStatus_States(t *testing.T) {
	healthy := Healthy("all good")
	if !healthy.IsHealthy() {
		t.Error("expected healthy status to be healthy")
	}
	if healthy.IsDegraded() || healthy.IsUnhealthy() {
		t.Error("expected healthy status to not be degraded or unhealthy")
	}

	degraded := Degraded("partially working")
	if !degraded.IsDegraded() {
		t.Error("expected degraded status to be degraded")
	}
	if degraded.IsHealthy() || degraded.IsUnhealthy() {
		t.Error("expected degraded status to not be healthy or unhealthy")
	}

	unhealthy := Unhealthy("not working")
	if !unhealthy.IsUnhealthy() {
		t.Error("expected unhealthy status to be unhealthy")
	}
	if unhealthy.IsHealthy() || unhealthy.IsDegraded() {
		t.Error("expected unhealthy status to not be healthy or degraded")
	}
}

func TestHealthStatus_Builder(t *testing.T) {
	status := NewHealthStatus().
		WithMessage("all systems operational").
		WithDetail("uptime", 3600).
		WithDetail("connections", 10)

	if status.Message != "all systems operational" {
		t.Errorf("expected message 'all systems operational', got %s", status.Message)
	}
	if status.Details["uptime"] != 3600 {
		t.Errorf("expected uptime 3600, got %v", status.Details["uptime"])
	}
	if status.Details["connections"] != 10 {
		t.Errorf("expected connections 10, got %v", status.Details["connections"])
	}
}

func TestHealthStatus_WithCheck(t *testing.T) {
	status := NewHealthStatus().
		WithCheck(HealthCheck{
			Name:    "database",
			State:   HealthStateHealthy,
			Message: "connected",
		}).
		WithCheck(HealthCheck{
			Name:    "cache",
			State:   HealthStateDegraded,
			Message: "slow responses",
		})

	// Overall state should be degraded (worst of checks)
	if !status.IsDegraded() {
		t.Error("expected status to be degraded after adding degraded check")
	}

	if len(status.Checks) != 2 {
		t.Errorf("expected 2 checks, got %d", len(status.Checks))
	}
}

func TestHealthStatus_UnhealthyCheckOverrides(t *testing.T) {
	status := NewHealthStatus().
		WithCheck(HealthCheck{
			Name:  "service1",
			State: HealthStateDegraded,
		}).
		WithCheck(HealthCheck{
			Name:  "service2",
			State: HealthStateUnhealthy,
		})

	if !status.IsUnhealthy() {
		t.Error("expected status to be unhealthy after adding unhealthy check")
	}
}

func TestMetricsReport_Builder(t *testing.T) {
	report := NewMetricsReport().
		WithCustomMetric("custom_counter", 100).
		WithCustomMetric("custom_gauge", 3.14)

	if report.Custom["custom_counter"] != 100 {
		t.Errorf("expected custom_counter 100, got %v", report.Custom["custom_counter"])
	}
	if report.Custom["custom_gauge"] != 3.14 {
		t.Errorf("expected custom_gauge 3.14, got %v", report.Custom["custom_gauge"])
	}
}

func TestMetricsCollector_RecordRequest(t *testing.T) {
	collector := NewMetricsCollector()

	// Record some requests
	collector.RecordRequest(true, 10.0)  // allowed
	collector.RecordRequest(true, 20.0)  // allowed
	collector.RecordRequest(false, 5.0)  // blocked
	collector.RecordError()

	report := collector.Report()

	if report.RequestsTotal != 4 {
		t.Errorf("expected RequestsTotal 4, got %d", report.RequestsTotal)
	}
	if report.RequestsAllowed != 2 {
		t.Errorf("expected RequestsAllowed 2, got %d", report.RequestsAllowed)
	}
	if report.RequestsBlocked != 1 {
		t.Errorf("expected RequestsBlocked 1, got %d", report.RequestsBlocked)
	}
	if report.RequestsErrored != 1 {
		t.Errorf("expected RequestsErrored 1, got %d", report.RequestsErrored)
	}
}

func TestMetricsCollector_ActiveRequests(t *testing.T) {
	collector := NewMetricsCollector()

	collector.IncrementActive()
	collector.IncrementActive()
	collector.IncrementActive()

	report := collector.Report()
	if report.RequestsActive != 3 {
		t.Errorf("expected RequestsActive 3, got %d", report.RequestsActive)
	}

	collector.DecrementActive()
	collector.DecrementActive()

	report = collector.Report()
	if report.RequestsActive != 1 {
		t.Errorf("expected RequestsActive 1, got %d", report.RequestsActive)
	}

	// Decrement below zero should not go negative
	collector.DecrementActive()
	collector.DecrementActive()

	report = collector.Report()
	if report.RequestsActive != 0 {
		t.Errorf("expected RequestsActive 0, got %d", report.RequestsActive)
	}
}

func TestMetricsCollector_Latencies(t *testing.T) {
	collector := NewMetricsCollector()

	// Add latency samples
	latencies := []float64{10, 20, 30, 40, 50}
	for _, l := range latencies {
		collector.RecordRequest(true, l)
	}

	report := collector.Report()

	// Average should be 30
	if report.AverageLatencyMs != 30 {
		t.Errorf("expected AverageLatencyMs 30, got %f", report.AverageLatencyMs)
	}

	// P50 should be around 30 (middle value)
	if report.P50LatencyMs != 30 {
		t.Errorf("expected P50LatencyMs 30, got %f", report.P50LatencyMs)
	}
}

func TestMetricsCollector_Uptime(t *testing.T) {
	collector := NewMetricsCollector()

	// Wait a bit
	time.Sleep(10 * time.Millisecond)

	report := collector.Report()
	if report.UptimeSeconds < 0.01 {
		t.Errorf("expected UptimeSeconds > 0.01, got %f", report.UptimeSeconds)
	}
}

func TestMetricsCollector_CustomMetrics(t *testing.T) {
	collector := NewMetricsCollector()
	collector.SetCustom("my_metric", 42)

	report := collector.Report()
	if report.Custom["my_metric"] != 42 {
		t.Errorf("expected my_metric 42, got %v", report.Custom["my_metric"])
	}
}
