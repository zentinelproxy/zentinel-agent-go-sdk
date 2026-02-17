// Simple Zentinel agent example.
//
// This example demonstrates a basic agent that:
// - Blocks requests to /admin paths
// - Adds custom headers to allowed requests
// - Logs request completions
package main

import (
	"context"
	"fmt"

	zentinel "github.com/zentinelproxy/zentinel-agent-go-sdk"
)

// SimpleAgent is a simple example agent that blocks admin paths.
type SimpleAgent struct {
	zentinel.BaseAgent
}

// Name returns the agent name.
func (a *SimpleAgent) Name() string {
	return "simple-agent"
}

// OnRequest processes incoming requests.
func (a *SimpleAgent) OnRequest(ctx context.Context, request *zentinel.Request) *zentinel.Decision {
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
	return zentinel.Allow().AddRequestHeader("X-Agent-Processed", "true")
}

// OnResponse processes responses.
func (a *SimpleAgent) OnResponse(ctx context.Context, request *zentinel.Request, response *zentinel.Response) *zentinel.Decision {
	// Add timing header
	return zentinel.Allow().AddResponseHeader("X-Processed-By", a.Name())
}

// OnRequestComplete logs completed requests.
func (a *SimpleAgent) OnRequestComplete(ctx context.Context, request *zentinel.Request, status int, durationMS int) {
	fmt.Printf("Request completed: %s %s -> %d (%dms)\n",
		request.Method(), request.Path(), status, durationMS)
}

func main() {
	zentinel.RunAgent(&SimpleAgent{})
}
