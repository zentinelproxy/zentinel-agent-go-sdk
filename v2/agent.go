package v2

import (
	"context"

	zentinel "github.com/zentinelproxy/zentinel-agent-go-sdk"
)

// AgentV2 extends the base Agent interface with v2 protocol features.
//
// Implement this interface to create an agent that supports the v2 protocol
// features including capability negotiation, health checks, and lifecycle hooks.
//
// Example:
//
//	type MyAgent struct {
//	    v2.BaseAgentV2
//	}
//
//	func (a *MyAgent) Name() string { return "my-agent" }
//
//	func (a *MyAgent) Capabilities() *v2.AgentCapabilities {
//	    return v2.NewAgentCapabilities().
//	        HandleRequestHeaders().
//	        HandleRequestBody()
//	}
//
//	func (a *MyAgent) OnRequest(ctx context.Context, req *zentinel.Request) *zentinel.Decision {
//	    // Your logic here
//	    return zentinel.Allow()
//	}
type AgentV2 interface {
	zentinel.Agent

	// Capabilities returns the agent's processing capabilities.
	// Called during handshake to negotiate features with the proxy.
	Capabilities() *AgentCapabilities

	// HealthCheck returns the current health status of the agent.
	// Called periodically by the proxy to verify agent health.
	HealthCheck(ctx context.Context) *HealthStatus

	// Metrics returns the current metrics for the agent.
	// Called by the proxy to collect agent performance data.
	Metrics(ctx context.Context) *MetricsReport

	// OnShutdown is called when the agent is being shut down.
	// Use this to clean up resources gracefully.
	OnShutdown(ctx context.Context)

	// OnDrain is called when the agent should stop accepting new requests.
	// Existing requests should be completed.
	OnDrain(ctx context.Context)

	// OnStreamClosed is called when a connection to the proxy is closed.
	// The streamID identifies the specific connection.
	OnStreamClosed(ctx context.Context, streamID string)

	// OnCancel is called when a request is cancelled.
	// The requestID identifies the cancelled request.
	OnCancel(ctx context.Context, requestID uint64)
}

// BaseAgentV2 provides default implementations for all AgentV2 methods.
// Embed this in your agent struct to only implement the methods you need.
//
// Example:
//
//	type MyAgent struct {
//	    v2.BaseAgentV2
//	}
//
//	func (a *MyAgent) Name() string { return "my-agent" }
//
//	func (a *MyAgent) OnRequest(ctx context.Context, req *zentinel.Request) *zentinel.Decision {
//	    // Your custom logic here
//	    return zentinel.Allow()
//	}
type BaseAgentV2 struct {
	zentinel.BaseAgent
	caps    *AgentCapabilities
	metrics *MetricsCollector
}

// NewBaseAgentV2 creates a new BaseAgentV2 with default capabilities.
func NewBaseAgentV2() *BaseAgentV2 {
	return &BaseAgentV2{
		caps:    NewAgentCapabilities(),
		metrics: NewMetricsCollector(),
	}
}

// Capabilities returns the default capabilities (request headers only).
func (a *BaseAgentV2) Capabilities() *AgentCapabilities {
	if a.caps == nil {
		a.caps = NewAgentCapabilities()
	}
	return a.caps
}

// HealthCheck returns a healthy status by default.
func (a *BaseAgentV2) HealthCheck(ctx context.Context) *HealthStatus {
	return NewHealthStatus()
}

// Metrics returns the current metrics.
func (a *BaseAgentV2) Metrics(ctx context.Context) *MetricsReport {
	if a.metrics == nil {
		a.metrics = NewMetricsCollector()
	}
	return a.metrics.Report()
}

// OnShutdown is a no-op by default.
func (a *BaseAgentV2) OnShutdown(ctx context.Context) {
}

// OnDrain is a no-op by default.
func (a *BaseAgentV2) OnDrain(ctx context.Context) {
}

// OnStreamClosed is a no-op by default.
func (a *BaseAgentV2) OnStreamClosed(ctx context.Context, streamID string) {
}

// OnCancel is a no-op by default.
func (a *BaseAgentV2) OnCancel(ctx context.Context, requestID uint64) {
}

// MetricsCollectorRef returns a reference to the metrics collector.
// Use this to record custom metrics.
func (a *BaseAgentV2) MetricsCollectorRef() *MetricsCollector {
	if a.metrics == nil {
		a.metrics = NewMetricsCollector()
	}
	return a.metrics
}

// SetCapabilities sets the agent capabilities.
func (a *BaseAgentV2) SetCapabilities(caps *AgentCapabilities) {
	a.caps = caps
}

// ConfigurableAgentV2 is an AgentV2 with typed configuration support.
//
// Example:
//
//	type MyConfig struct {
//	    RateLimit int  `json:"rate_limit"`
//	    Enabled   bool `json:"enabled"`
//	}
//
//	type MyAgent struct {
//	    *v2.ConfigurableAgentV2Base[MyConfig]
//	}
//
//	func NewMyAgent() *MyAgent {
//	    return &MyAgent{
//	        ConfigurableAgentV2Base: v2.NewConfigurableAgentV2(MyConfig{
//	            RateLimit: 100,
//	            Enabled:   true,
//	        }),
//	    }
//	}
type ConfigurableAgentV2[T any] interface {
	AgentV2
	zentinel.ConfigurableAgent[T]
}

// ConfigurableAgentV2Base provides a base implementation for ConfigurableAgentV2.
type ConfigurableAgentV2Base[T any] struct {
	*zentinel.ConfigurableAgentBase[T]
	caps    *AgentCapabilities
	metrics *MetricsCollector
}

// NewConfigurableAgentV2 creates a new ConfigurableAgentV2Base with default config.
func NewConfigurableAgentV2[T any](defaultConfig T) *ConfigurableAgentV2Base[T] {
	return &ConfigurableAgentV2Base[T]{
		ConfigurableAgentBase: zentinel.NewConfigurableAgent(defaultConfig),
		caps:                  NewAgentCapabilities(),
		metrics:               NewMetricsCollector(),
	}
}

// Capabilities returns the agent capabilities.
func (a *ConfigurableAgentV2Base[T]) Capabilities() *AgentCapabilities {
	if a.caps == nil {
		a.caps = NewAgentCapabilities()
	}
	return a.caps
}

// SetCapabilities sets the agent capabilities.
func (a *ConfigurableAgentV2Base[T]) SetCapabilities(caps *AgentCapabilities) {
	a.caps = caps
}

// HealthCheck returns a healthy status by default.
func (a *ConfigurableAgentV2Base[T]) HealthCheck(ctx context.Context) *HealthStatus {
	return NewHealthStatus()
}

// Metrics returns the current metrics.
func (a *ConfigurableAgentV2Base[T]) Metrics(ctx context.Context) *MetricsReport {
	if a.metrics == nil {
		a.metrics = NewMetricsCollector()
	}
	return a.metrics.Report()
}

// OnShutdown is a no-op by default.
func (a *ConfigurableAgentV2Base[T]) OnShutdown(ctx context.Context) {
}

// OnDrain is a no-op by default.
func (a *ConfigurableAgentV2Base[T]) OnDrain(ctx context.Context) {
}

// OnStreamClosed is a no-op by default.
func (a *ConfigurableAgentV2Base[T]) OnStreamClosed(ctx context.Context, streamID string) {
}

// OnCancel is a no-op by default.
func (a *ConfigurableAgentV2Base[T]) OnCancel(ctx context.Context, requestID uint64) {
}

// MetricsCollectorRef returns a reference to the metrics collector.
func (a *ConfigurableAgentV2Base[T]) MetricsCollectorRef() *MetricsCollector {
	if a.metrics == nil {
		a.metrics = NewMetricsCollector()
	}
	return a.metrics
}
