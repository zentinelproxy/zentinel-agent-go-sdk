// Reverse connection Zentinel v2 agent example.
//
// This example demonstrates a v2 agent using reverse connections:
// - Agent initiates connection to the proxy (useful behind NAT/firewall)
// - Automatic reconnection on connection loss
// - Authentication token support
// - Health monitoring
//
// Run with:
//
//	go run main.go --reverse localhost:9001
//
// Or with authentication:
//
//	go run main.go --reverse localhost:9001 --auth-token my-secret-token
package main

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	zentinel "github.com/zentinelproxy/zentinel-agent-go-sdk"
	"github.com/zentinelproxy/zentinel-agent-go-sdk/v2"
)

// WAFAgentV2 is a simple WAF agent using reverse connections.
type WAFAgentV2 struct {
	v2.BaseAgentV2
	blockedPatterns []string
	requestCount    atomic.Uint64
	blockCount      atomic.Uint64
	startTime       time.Time
}

// NewWAFAgentV2 creates a new WAF v2 agent.
func NewWAFAgentV2() *WAFAgentV2 {
	agent := &WAFAgentV2{
		BaseAgentV2: *v2.NewBaseAgentV2(),
		blockedPatterns: []string{
			"<script>",
			"javascript:",
			"onerror=",
			"onload=",
			"eval(",
			"../",
			"..\\",
			"; DROP TABLE",
			"UNION SELECT",
			"' OR '1'='1",
		},
		startTime: time.Now(),
	}

	// Configure capabilities for body inspection
	agent.SetCapabilities(
		v2.NewAgentCapabilities().
			HandleRequestHeaders().
			HandleRequestBody().
			WithStreaming().
			WithMaxConcurrentRequests(500),
	)

	return agent
}

// Name returns the agent name.
func (a *WAFAgentV2) Name() string {
	return "waf-agent-v2"
}

// OnRequest inspects request headers for malicious patterns.
func (a *WAFAgentV2) OnRequest(ctx context.Context, request *zentinel.Request) *zentinel.Decision {
	a.requestCount.Add(1)

	// Check URI for malicious patterns
	uri := request.URI()
	for _, pattern := range a.blockedPatterns {
		if strings.Contains(strings.ToLower(uri), strings.ToLower(pattern)) {
			a.blockCount.Add(1)
			return zentinel.Deny().
				WithBody("Request blocked by WAF").
				WithTag("waf").
				WithRuleID("URI_PATTERN_MATCH").
				WithMetadata("pattern", pattern).
				WithMetadata("location", "uri")
		}
	}

	// Check common headers
	headersToCheck := []string{
		request.Header("User-Agent"),
		request.Header("Referer"),
		request.Header("Cookie"),
	}

	for _, headerValue := range headersToCheck {
		for _, pattern := range a.blockedPatterns {
			if strings.Contains(strings.ToLower(headerValue), strings.ToLower(pattern)) {
				a.blockCount.Add(1)
				return zentinel.Deny().
					WithBody("Request blocked by WAF").
					WithTag("waf").
					WithRuleID("HEADER_PATTERN_MATCH").
					WithMetadata("pattern", pattern).
					WithMetadata("location", "header")
			}
		}
	}

	// Add WAF header to indicate inspection
	return zentinel.Allow().
		AddRequestHeader("X-WAF-Inspected", "true")
}

// OnRequestBody inspects request body for malicious patterns.
func (a *WAFAgentV2) OnRequestBody(ctx context.Context, request *zentinel.Request) *zentinel.Decision {
	body := request.BodyString()

	for _, pattern := range a.blockedPatterns {
		if strings.Contains(strings.ToLower(body), strings.ToLower(pattern)) {
			a.blockCount.Add(1)
			return zentinel.Deny().
				WithBody("Request body blocked by WAF").
				WithTag("waf").
				WithRuleID("BODY_PATTERN_MATCH").
				WithMetadata("pattern", pattern).
				WithMetadata("location", "body")
		}
	}

	return zentinel.Allow()
}

// HealthCheck returns detailed health status.
func (a *WAFAgentV2) HealthCheck(ctx context.Context) *v2.HealthStatus {
	totalRequests := a.requestCount.Load()
	totalBlocks := a.blockCount.Load()

	var blockRate float64
	if totalRequests > 0 {
		blockRate = float64(totalBlocks) / float64(totalRequests) * 100
	}

	status := v2.Healthy("operational").
		WithDetail("total_requests", totalRequests).
		WithDetail("total_blocks", totalBlocks).
		WithDetail("block_rate_percent", blockRate).
		WithDetail("uptime_seconds", time.Since(a.startTime).Seconds()).
		WithDetail("patterns_loaded", len(a.blockedPatterns))

	// Warn if block rate is high (might indicate attack or false positives)
	if blockRate > 50 && totalRequests > 100 {
		return v2.Degraded("high block rate").
			WithDetail("block_rate_percent", blockRate)
	}

	return status
}

// Metrics returns agent metrics.
func (a *WAFAgentV2) Metrics(ctx context.Context) *v2.MetricsReport {
	report := a.BaseAgentV2.Metrics(ctx)
	report.WithCustomMetric("waf_requests_total", a.requestCount.Load())
	report.WithCustomMetric("waf_blocks_total", a.blockCount.Load())
	report.WithCustomMetric("waf_patterns_count", len(a.blockedPatterns))
	return report
}

// OnShutdown handles graceful shutdown.
func (a *WAFAgentV2) OnShutdown(ctx context.Context) {
	fmt.Printf("WAF agent shutting down. Processed %d requests, blocked %d\n",
		a.requestCount.Load(), a.blockCount.Load())
}

// OnStreamClosed handles connection closure (important for reverse connections).
func (a *WAFAgentV2) OnStreamClosed(ctx context.Context, streamID string) {
	fmt.Printf("Connection to proxy lost: %s (will attempt reconnect)\n", streamID)
}

func main() {
	v2.RunAgentV2(NewWAFAgentV2())
}
