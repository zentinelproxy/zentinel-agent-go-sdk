// Configurable Zentinel agent example.
//
// This example demonstrates an agent with typed configuration that:
// - Accepts rate limit configuration from the proxy
// - Tracks request counts per client IP
// - Rate limits clients exceeding the threshold
package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	zentinel "github.com/zentinelproxy/zentinel-agent-go-sdk"
)

// RateLimitConfig is the configuration for the rate limiting agent.
type RateLimitConfig struct {
	Enabled           bool     `json:"enabled"`
	RequestsPerMinute int      `json:"requests_per_minute"`
	BlockedPaths      []string `json:"blocked_paths"`
}

// RateLimitAgent is an agent that rate limits requests per client IP.
type RateLimitAgent struct {
	*zentinel.ConfigurableAgentBase[RateLimitConfig]
	requestCounts map[string]int
	mu            sync.Mutex
	resetOnce     sync.Once
}

// NewRateLimitAgent creates a new rate limit agent.
func NewRateLimitAgent() *RateLimitAgent {
	return &RateLimitAgent{
		ConfigurableAgentBase: zentinel.NewConfigurableAgent(RateLimitConfig{
			Enabled:           true,
			RequestsPerMinute: 100,
			BlockedPaths:      []string{},
		}),
		requestCounts: make(map[string]int),
	}
}

// Name returns the agent name.
func (a *RateLimitAgent) Name() string {
	return "rate-limit-agent"
}

// OnConfigApplied is called after configuration is applied.
func (a *RateLimitAgent) OnConfigApplied(ctx context.Context, config RateLimitConfig) {
	fmt.Printf("Configuration applied: %+v\n", config)

	// Start reset task if not running
	a.resetOnce.Do(func() {
		go a.resetCounts()
	})
}

// resetCounts resets request counts every minute.
func (a *RateLimitAgent) resetCounts() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		a.mu.Lock()
		a.requestCounts = make(map[string]int)
		a.mu.Unlock()
	}
}

// OnRequest processes incoming requests with rate limiting.
func (a *RateLimitAgent) OnRequest(ctx context.Context, request *zentinel.Request) *zentinel.Decision {
	config := a.Config()

	// Check if agent is enabled
	if !config.Enabled {
		return zentinel.Allow()
	}

	// Check blocked paths
	for _, blockedPath := range config.BlockedPaths {
		if request.PathStartsWith(blockedPath) {
			return zentinel.Deny().
				WithBody(fmt.Sprintf("Path %s is blocked", blockedPath)).
				WithTag("blocked_path")
		}
	}

	// Check rate limit
	clientIP := request.ClientIP()
	a.mu.Lock()
	a.requestCounts[clientIP]++
	count := a.requestCounts[clientIP]
	a.mu.Unlock()

	if count > config.RequestsPerMinute {
		return zentinel.RateLimited().
			WithBody("Rate limit exceeded").
			WithTag("rate_limited").
			WithMetadata("client_ip", clientIP).
			WithMetadata("request_count", count).
			WithMetadata("limit", config.RequestsPerMinute)
	}

	// Allow with rate limit headers
	remaining := config.RequestsPerMinute - count
	return zentinel.Allow().
		AddResponseHeader("X-RateLimit-Limit", fmt.Sprintf("%d", config.RequestsPerMinute)).
		AddResponseHeader("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
}

// OnResponse adds rate limit headers to response.
func (a *RateLimitAgent) OnResponse(ctx context.Context, request *zentinel.Request, response *zentinel.Response) *zentinel.Decision {
	return zentinel.Allow()
}

func main() {
	zentinel.RunAgent(NewRateLimitAgent())
}
