// Body inspection Zentinel agent example.
//
// This example demonstrates an agent that inspects request and response bodies:
// - Validates JSON request bodies
// - Blocks requests with prohibited content
// - Adds security headers to HTML responses
package main

import (
	"context"
	"encoding/json"
	"strings"

	zentinel "github.com/zentinelproxy/zentinel-agent-go-sdk"
)

// BodyInspectionAgent inspects request and response bodies.
type BodyInspectionAgent struct {
	zentinel.BaseAgent
}

// Name returns the agent name.
func (a *BodyInspectionAgent) Name() string {
	return "body-inspection-agent"
}

// OnRequest checks if body inspection is needed.
func (a *BodyInspectionAgent) OnRequest(ctx context.Context, request *zentinel.Request) *zentinel.Decision {
	// Request body inspection for POST/PUT requests with JSON
	if (request.IsPost() || request.IsPut()) && request.IsJSON() {
		return zentinel.Allow().NeedsMoreData()
	}
	return zentinel.Allow()
}

// OnRequestBody inspects the request body.
func (a *BodyInspectionAgent) OnRequestBody(ctx context.Context, request *zentinel.Request) *zentinel.Decision {
	body := request.Body()

	// Validate JSON
	if request.IsJSON() {
		var data interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			return zentinel.Block(400).
				WithBody("Invalid JSON body").
				WithTag("validation_error")
		}

		// Check for prohibited content
		bodyStr := strings.ToLower(string(body))
		prohibitedWords := []string{"script", "eval", "onclick"}
		for _, word := range prohibitedWords {
			if strings.Contains(bodyStr, word) {
				return zentinel.Deny().
					WithBody("Request contains prohibited content").
					WithTag("security").
					WithRuleID("PROHIBITED_CONTENT")
			}
		}
	}

	return zentinel.Allow().AddRequestHeader("X-Body-Validated", "true")
}

// OnResponse checks if response body inspection is needed.
func (a *BodyInspectionAgent) OnResponse(ctx context.Context, request *zentinel.Request, response *zentinel.Response) *zentinel.Decision {
	// Add security headers to HTML responses
	if response.IsHTML() {
		return zentinel.Allow().
			AddResponseHeader("X-Content-Type-Options", "nosniff").
			AddResponseHeader("X-Frame-Options", "DENY").
			AddResponseHeader("X-XSS-Protection", "1; mode=block")
	}
	return zentinel.Allow()
}

// OnResponseBody inspects the response body.
func (a *BodyInspectionAgent) OnResponseBody(ctx context.Context, request *zentinel.Request, response *zentinel.Response) *zentinel.Decision {
	// Could add response body inspection here
	return zentinel.Allow()
}

func main() {
	zentinel.RunAgent(&BodyInspectionAgent{})
}
