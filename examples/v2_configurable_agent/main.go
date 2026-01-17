// Configurable Sentinel v2 agent example.
//
// This example demonstrates a v2 agent with typed configuration that:
// - Accepts rate limit configuration from the proxy
// - Tracks request counts per client IP
// - Rate limits clients exceeding the threshold
// - Provides detailed health and metrics
// - Supports body inspection for request validation
//
// Run with:
//
//	go run main.go --socket /tmp/rate-limit-v2.sock
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	sentinel "github.com/raskell-io/sentinel-agent-go-sdk"
	"github.com/raskell-io/sentinel-agent-go-sdk/v2"
)

// RateLimitConfig is the configuration for the rate limiting agent.
type RateLimitConfig struct {
	Enabled           bool     `json:"enabled"`
	RequestsPerMinute int      `json:"requests_per_minute"`
	BlockedPaths      []string `json:"blocked_paths"`
	MaxBodySize       int      `json:"max_body_size"`
}

// RateLimitAgentV2 is a v2 agent that rate limits requests per client IP.
type RateLimitAgentV2 struct {
	*v2.ConfigurableAgentV2Base[RateLimitConfig]
	requestCounts map[string]int
	mu            sync.Mutex
	resetOnce     sync.Once
}

// NewRateLimitAgentV2 creates a new rate limit v2 agent.
func NewRateLimitAgentV2() *RateLimitAgentV2 {
	agent := &RateLimitAgentV2{
		ConfigurableAgentV2Base: v2.NewConfigurableAgentV2(RateLimitConfig{
			Enabled:           true,
			RequestsPerMinute: 100,
			BlockedPaths:      []string{},
			MaxBodySize:       1024 * 1024, // 1MB
		}),
		requestCounts: make(map[string]int),
	}

	// Configure capabilities
	agent.SetCapabilities(
		v2.NewAgentCapabilities().
			HandleRequestHeaders().
			HandleRequestBody().
			HandleResponseHeaders().
			WithMaxConcurrentRequests(200),
	)

	return agent
}

// Name returns the agent name.
func (a *RateLimitAgentV2) Name() string {
	return "rate-limit-agent-v2"
}

// OnConfigApplied is called after configuration is applied.
func (a *RateLimitAgentV2) OnConfigApplied(ctx context.Context, config RateLimitConfig) {
	fmt.Printf("Configuration applied: %+v\n", config)

	// Start reset task if not running
	a.resetOnce.Do(func() {
		go a.resetCounts()
	})
}

// OnConfigure overrides to call our OnConfigApplied.
func (a *RateLimitAgentV2) OnConfigure(ctx context.Context, configMap map[string]interface{}) error {
	config, err := a.ParseConfig(configMap)
	if err != nil {
		return err
	}
	a.SetConfig(config)
	a.OnConfigApplied(ctx, config)
	return nil
}

// resetCounts resets request counts every minute.
func (a *RateLimitAgentV2) resetCounts() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		a.mu.Lock()
		a.requestCounts = make(map[string]int)
		a.mu.Unlock()
	}
}

// OnRequest processes incoming requests with rate limiting.
func (a *RateLimitAgentV2) OnRequest(ctx context.Context, request *sentinel.Request) *sentinel.Decision {
	config := a.Config()

	// Check if agent is enabled
	if !config.Enabled {
		return sentinel.Allow()
	}

	// Check blocked paths
	for _, blockedPath := range config.BlockedPaths {
		if request.PathStartsWith(blockedPath) {
			return sentinel.Deny().
				WithBody(fmt.Sprintf("Path %s is blocked", blockedPath)).
				WithTag("blocked_path").
				WithRuleID("PATH_BLOCKED")
		}
	}

	// Check rate limit
	clientIP := request.ClientIP()
	a.mu.Lock()
	a.requestCounts[clientIP]++
	count := a.requestCounts[clientIP]
	a.mu.Unlock()

	if count > config.RequestsPerMinute {
		return sentinel.RateLimited().
			WithBody("Rate limit exceeded").
			WithTag("rate_limited").
			WithMetadata("client_ip", clientIP).
			WithMetadata("request_count", count).
			WithMetadata("limit", config.RequestsPerMinute).
			WithBlockHeader("Retry-After", "60")
	}

	// Allow with rate limit headers
	remaining := config.RequestsPerMinute - count
	return sentinel.Allow().
		AddResponseHeader("X-RateLimit-Limit", fmt.Sprintf("%d", config.RequestsPerMinute)).
		AddResponseHeader("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining)).
		AddResponseHeader("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(time.Minute).Unix()))
}

// OnRequestBody validates request body size.
func (a *RateLimitAgentV2) OnRequestBody(ctx context.Context, request *sentinel.Request) *sentinel.Decision {
	config := a.Config()

	body := request.Body()
	if len(body) > config.MaxBodySize {
		return sentinel.Block(413).
			WithBody("Request body too large").
			WithTag("body_too_large").
			WithMetadata("body_size", len(body)).
			WithMetadata("max_size", config.MaxBodySize)
	}

	// Validate JSON if content type is JSON
	if request.IsJSON() && len(body) > 0 {
		var js interface{}
		if err := json.Unmarshal(body, &js); err != nil {
			return sentinel.Block(400).
				WithBody("Invalid JSON in request body").
				WithTag("invalid_json")
		}
	}

	return sentinel.Allow()
}

// OnResponse adds rate limit headers to response.
func (a *RateLimitAgentV2) OnResponse(ctx context.Context, request *sentinel.Request, response *sentinel.Response) *sentinel.Decision {
	return sentinel.Allow()
}

// HealthCheck returns detailed health status.
func (a *RateLimitAgentV2) HealthCheck(ctx context.Context) *v2.HealthStatus {
	a.mu.Lock()
	activeClients := len(a.requestCounts)
	a.mu.Unlock()

	config := a.Config()

	status := v2.Healthy("operational").
		WithDetail("enabled", config.Enabled).
		WithDetail("rate_limit", config.RequestsPerMinute).
		WithDetail("active_clients", activeClients)

	// Add degraded state if too many clients
	if activeClients > 1000 {
		return v2.Degraded("high client count").
			WithDetail("active_clients", activeClients)
	}

	return status
}

// Metrics returns detailed metrics.
func (a *RateLimitAgentV2) Metrics(ctx context.Context) *v2.MetricsReport {
	report := a.ConfigurableAgentV2Base.Metrics(ctx)

	a.mu.Lock()
	report.WithCustomMetric("active_clients", len(a.requestCounts))
	var totalRequests int
	for _, count := range a.requestCounts {
		totalRequests += count
	}
	report.WithCustomMetric("requests_in_window", totalRequests)
	a.mu.Unlock()

	return report
}

// OnShutdown handles graceful shutdown.
func (a *RateLimitAgentV2) OnShutdown(ctx context.Context) {
	fmt.Println("Rate limit agent shutting down...")
}

func main() {
	v2.RunAgentV2(NewRateLimitAgentV2())
}
