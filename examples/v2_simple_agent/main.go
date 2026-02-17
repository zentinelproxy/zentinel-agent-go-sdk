// Simple Zentinel v2 agent example.
//
// This example demonstrates a basic v2 agent that:
// - Declares capabilities for request headers and response headers
// - Blocks requests to /admin paths
// - Adds custom headers to allowed requests
// - Implements health checking
// - Handles graceful shutdown
//
// Run with:
//
//	go run main.go --socket /tmp/my-agent.sock
//
// Or with JSON logs:
//
//	go run main.go --socket /tmp/my-agent.sock --json-logs
package main

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	zentinel "github.com/zentinelproxy/zentinel-agent-go-sdk"
	"github.com/zentinelproxy/zentinel-agent-go-sdk/v2"
)

// SimpleAgentV2 is a simple v2 agent that blocks admin paths.
type SimpleAgentV2 struct {
	v2.BaseAgentV2
	requestCount atomic.Uint64
	startTime    time.Time
}

// NewSimpleAgentV2 creates a new simple v2 agent.
func NewSimpleAgentV2() *SimpleAgentV2 {
	agent := &SimpleAgentV2{
		BaseAgentV2: *v2.NewBaseAgentV2(),
		startTime:   time.Now(),
	}
	// Configure capabilities
	agent.SetCapabilities(
		v2.NewAgentCapabilities().
			HandleRequestHeaders().
			HandleResponseHeaders().
			WithMaxConcurrentRequests(100),
	)
	return agent
}

// Name returns the agent name.
func (a *SimpleAgentV2) Name() string {
	return "simple-agent-v2"
}

// OnRequest processes incoming requests.
func (a *SimpleAgentV2) OnRequest(ctx context.Context, request *zentinel.Request) *zentinel.Decision {
	a.requestCount.Add(1)

	// Block admin paths
	if request.PathStartsWith("/admin") {
		return zentinel.Deny().
			WithBody("Access denied").
			WithTag("security").
			WithRuleID("ADMIN_BLOCKED")
	}

	// Block requests without User-Agent
	if request.UserAgent() == "" {
		return zentinel.Block(400).
			WithBody("User-Agent header required").
			WithTag("validation")
	}

	// Allow with custom header
	return zentinel.Allow().
		AddRequestHeader("X-Agent-Processed", "true").
		AddRequestHeader("X-Agent-Version", "v2")
}

// OnResponse processes responses.
func (a *SimpleAgentV2) OnResponse(ctx context.Context, request *zentinel.Request, response *zentinel.Response) *zentinel.Decision {
	// Add security headers to HTML responses
	if response.IsHTML() {
		return zentinel.Allow().
			AddResponseHeader("X-Content-Type-Options", "nosniff").
			AddResponseHeader("X-Frame-Options", "DENY")
	}

	return zentinel.Allow().
		AddResponseHeader("X-Processed-By", a.Name())
}

// HealthCheck returns the agent's health status.
func (a *SimpleAgentV2) HealthCheck(ctx context.Context) *v2.HealthStatus {
	return v2.Healthy("operational").
		WithDetail("requests_processed", a.requestCount.Load()).
		WithDetail("uptime_seconds", time.Since(a.startTime).Seconds())
}

// Metrics returns the agent's metrics.
func (a *SimpleAgentV2) Metrics(ctx context.Context) *v2.MetricsReport {
	report := a.BaseAgentV2.Metrics(ctx)
	report.WithCustomMetric("total_requests", a.requestCount.Load())
	return report
}

// OnShutdown handles graceful shutdown.
func (a *SimpleAgentV2) OnShutdown(ctx context.Context) {
	fmt.Printf("Shutting down after processing %d requests\n", a.requestCount.Load())
}

// OnDrain handles drain mode (stop accepting new requests).
func (a *SimpleAgentV2) OnDrain(ctx context.Context) {
	fmt.Println("Entering drain mode...")
}

// OnStreamClosed handles connection closure.
func (a *SimpleAgentV2) OnStreamClosed(ctx context.Context, streamID string) {
	fmt.Printf("Stream closed: %s\n", streamID)
}

// OnCancel handles request cancellation.
func (a *SimpleAgentV2) OnCancel(ctx context.Context, requestID uint64) {
	fmt.Printf("Request cancelled: %d\n", requestID)
}

func main() {
	v2.RunAgentV2(NewSimpleAgentV2())
}
