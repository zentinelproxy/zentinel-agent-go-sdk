package v2

import (
	"time"
)

// HealthState represents the health state of an agent.
type HealthState string

const (
	// HealthStateHealthy indicates the agent is fully operational.
	HealthStateHealthy HealthState = "healthy"

	// HealthStateDegraded indicates the agent is operational but with issues.
	HealthStateDegraded HealthState = "degraded"

	// HealthStateUnhealthy indicates the agent is not operational.
	HealthStateUnhealthy HealthState = "unhealthy"
)

// HealthStatus represents the current health status of an agent.
type HealthStatus struct {
	// State is the current health state.
	State HealthState `json:"state"`

	// Message provides additional context about the health state.
	Message string `json:"message,omitempty"`

	// Details contains structured health check details.
	Details map[string]interface{} `json:"details,omitempty"`

	// Timestamp is when this status was generated.
	Timestamp time.Time `json:"timestamp"`

	// Checks contains individual health check results.
	Checks []HealthCheck `json:"checks,omitempty"`
}

// HealthCheck represents an individual health check result.
type HealthCheck struct {
	// Name identifies this health check.
	Name string `json:"name"`

	// State is the result of this check.
	State HealthState `json:"state"`

	// Message provides context about the check result.
	Message string `json:"message,omitempty"`

	// Duration is how long the check took.
	Duration time.Duration `json:"duration,omitempty"`
}

// NewHealthStatus creates a new healthy status.
func NewHealthStatus() *HealthStatus {
	return &HealthStatus{
		State:     HealthStateHealthy,
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
		Checks:    []HealthCheck{},
	}
}

// Healthy creates a healthy status with an optional message.
func Healthy(message string) *HealthStatus {
	return &HealthStatus{
		State:     HealthStateHealthy,
		Message:   message,
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
		Checks:    []HealthCheck{},
	}
}

// Degraded creates a degraded status with a message.
func Degraded(message string) *HealthStatus {
	return &HealthStatus{
		State:     HealthStateDegraded,
		Message:   message,
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
		Checks:    []HealthCheck{},
	}
}

// Unhealthy creates an unhealthy status with a message.
func Unhealthy(message string) *HealthStatus {
	return &HealthStatus{
		State:     HealthStateUnhealthy,
		Message:   message,
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
		Checks:    []HealthCheck{},
	}
}

// WithMessage sets the status message.
func (s *HealthStatus) WithMessage(message string) *HealthStatus {
	s.Message = message
	return s
}

// WithDetail adds a detail key-value pair.
func (s *HealthStatus) WithDetail(key string, value interface{}) *HealthStatus {
	if s.Details == nil {
		s.Details = make(map[string]interface{})
	}
	s.Details[key] = value
	return s
}

// WithCheck adds a health check result.
func (s *HealthStatus) WithCheck(check HealthCheck) *HealthStatus {
	s.Checks = append(s.Checks, check)
	// Update overall state based on worst check
	if check.State == HealthStateUnhealthy && s.State != HealthStateUnhealthy {
		s.State = HealthStateUnhealthy
	} else if check.State == HealthStateDegraded && s.State == HealthStateHealthy {
		s.State = HealthStateDegraded
	}
	return s
}

// IsHealthy returns true if the status is healthy.
func (s *HealthStatus) IsHealthy() bool {
	return s.State == HealthStateHealthy
}

// IsDegraded returns true if the status is degraded.
func (s *HealthStatus) IsDegraded() bool {
	return s.State == HealthStateDegraded
}

// IsUnhealthy returns true if the status is unhealthy.
func (s *HealthStatus) IsUnhealthy() bool {
	return s.State == HealthStateUnhealthy
}

// MetricsReport contains agent performance metrics.
type MetricsReport struct {
	// RequestsTotal is the total number of requests processed.
	RequestsTotal uint64 `json:"requests_total"`

	// RequestsActive is the current number of in-flight requests.
	RequestsActive uint32 `json:"requests_active"`

	// RequestsAllowed is the number of allowed requests.
	RequestsAllowed uint64 `json:"requests_allowed"`

	// RequestsBlocked is the number of blocked requests.
	RequestsBlocked uint64 `json:"requests_blocked"`

	// RequestsErrored is the number of requests that resulted in errors.
	RequestsErrored uint64 `json:"requests_errored"`

	// AverageLatencyMs is the average request processing latency in milliseconds.
	AverageLatencyMs float64 `json:"average_latency_ms"`

	// P50LatencyMs is the 50th percentile latency in milliseconds.
	P50LatencyMs float64 `json:"p50_latency_ms,omitempty"`

	// P95LatencyMs is the 95th percentile latency in milliseconds.
	P95LatencyMs float64 `json:"p95_latency_ms,omitempty"`

	// P99LatencyMs is the 99th percentile latency in milliseconds.
	P99LatencyMs float64 `json:"p99_latency_ms,omitempty"`

	// UptimeSeconds is the agent uptime in seconds.
	UptimeSeconds float64 `json:"uptime_seconds"`

	// Custom contains agent-specific metrics.
	Custom map[string]interface{} `json:"custom,omitempty"`

	// Timestamp is when these metrics were collected.
	Timestamp time.Time `json:"timestamp"`
}

// NewMetricsReport creates a new empty metrics report.
func NewMetricsReport() *MetricsReport {
	return &MetricsReport{
		Custom:    make(map[string]interface{}),
		Timestamp: time.Now(),
	}
}

// WithCustomMetric adds a custom metric.
func (m *MetricsReport) WithCustomMetric(name string, value interface{}) *MetricsReport {
	if m.Custom == nil {
		m.Custom = make(map[string]interface{})
	}
	m.Custom[name] = value
	return m
}

// MetricsCollector collects agent metrics over time.
type MetricsCollector struct {
	startTime       time.Time
	requestsTotal   uint64
	requestsActive  uint32
	requestsAllowed uint64
	requestsBlocked uint64
	requestsErrored uint64
	latencies       []float64
	custom          map[string]interface{}
}

// NewMetricsCollector creates a new metrics collector.
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		startTime: time.Now(),
		latencies: make([]float64, 0, 1000),
		custom:    make(map[string]interface{}),
	}
}

// RecordRequest records a completed request.
func (c *MetricsCollector) RecordRequest(allowed bool, latencyMs float64) {
	c.requestsTotal++
	if allowed {
		c.requestsAllowed++
	} else {
		c.requestsBlocked++
	}
	c.latencies = append(c.latencies, latencyMs)
	// Keep only last 1000 latencies for percentile calculation
	if len(c.latencies) > 1000 {
		c.latencies = c.latencies[len(c.latencies)-1000:]
	}
}

// RecordError records an error.
func (c *MetricsCollector) RecordError() {
	c.requestsTotal++
	c.requestsErrored++
}

// IncrementActive increments the active request count.
func (c *MetricsCollector) IncrementActive() {
	c.requestsActive++
}

// DecrementActive decrements the active request count.
func (c *MetricsCollector) DecrementActive() {
	if c.requestsActive > 0 {
		c.requestsActive--
	}
}

// SetCustom sets a custom metric value.
func (c *MetricsCollector) SetCustom(name string, value interface{}) {
	c.custom[name] = value
}

// Report generates a metrics report.
func (c *MetricsCollector) Report() *MetricsReport {
	report := &MetricsReport{
		RequestsTotal:   c.requestsTotal,
		RequestsActive:  c.requestsActive,
		RequestsAllowed: c.requestsAllowed,
		RequestsBlocked: c.requestsBlocked,
		RequestsErrored: c.requestsErrored,
		UptimeSeconds:   time.Since(c.startTime).Seconds(),
		Custom:          c.custom,
		Timestamp:       time.Now(),
	}

	if len(c.latencies) > 0 {
		var sum float64
		for _, l := range c.latencies {
			sum += l
		}
		report.AverageLatencyMs = sum / float64(len(c.latencies))

		// Calculate percentiles (simplified - would need proper implementation)
		sorted := make([]float64, len(c.latencies))
		copy(sorted, c.latencies)
		sortFloat64s(sorted)

		report.P50LatencyMs = percentile(sorted, 0.50)
		report.P95LatencyMs = percentile(sorted, 0.95)
		report.P99LatencyMs = percentile(sorted, 0.99)
	}

	return report
}

// sortFloat64s sorts a slice of float64s in ascending order.
func sortFloat64s(s []float64) {
	for i := 0; i < len(s); i++ {
		for j := i + 1; j < len(s); j++ {
			if s[j] < s[i] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

// percentile calculates the p-th percentile of a sorted slice.
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * p)
	return sorted[idx]
}
