// Simple Sentinel agent example.
//
// This example demonstrates a basic agent that:
// - Blocks requests to /admin paths
// - Adds custom headers to allowed requests
// - Logs request completions
package main

import (
	"context"
	"fmt"

	sentinel "github.com/raskell-io/sentinel-agent-go-sdk"
)

// SimpleAgent is a simple example agent that blocks admin paths.
type SimpleAgent struct {
	sentinel.BaseAgent
}

// Name returns the agent name.
func (a *SimpleAgent) Name() string {
	return "simple-agent"
}

// OnRequest processes incoming requests.
func (a *SimpleAgent) OnRequest(ctx context.Context, request *sentinel.Request) *sentinel.Decision {
	// Block admin paths
	if request.PathStartsWith("/admin") {
		return sentinel.Deny().
			WithBody("Access denied").
			WithTag("security").
			WithRuleID("ADMIN_BLOCKED")
	}

	// Block requests without User-Agent
	if request.UserAgent() == "" {
		return sentinel.Block(400).
			WithBody("User-Agent header required").
			WithTag("validation")
	}

	// Allow with custom header
	return sentinel.Allow().AddRequestHeader("X-Agent-Processed", "true")
}

// OnResponse processes responses.
func (a *SimpleAgent) OnResponse(ctx context.Context, request *sentinel.Request, response *sentinel.Response) *sentinel.Decision {
	// Add timing header
	return sentinel.Allow().AddResponseHeader("X-Processed-By", a.Name())
}

// OnRequestComplete logs completed requests.
func (a *SimpleAgent) OnRequestComplete(ctx context.Context, request *sentinel.Request, status int, durationMS int) {
	fmt.Printf("Request completed: %s %s -> %d (%dms)\n",
		request.Method(), request.Path(), status, durationMS)
}

func main() {
	sentinel.RunAgent(&SimpleAgent{})
}
