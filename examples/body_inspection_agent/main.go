// Body inspection Sentinel agent example.
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

	sentinel "github.com/raskell-io/sentinel-agent-go-sdk"
)

// BodyInspectionAgent inspects request and response bodies.
type BodyInspectionAgent struct {
	sentinel.BaseAgent
}

// Name returns the agent name.
func (a *BodyInspectionAgent) Name() string {
	return "body-inspection-agent"
}

// OnRequest checks if body inspection is needed.
func (a *BodyInspectionAgent) OnRequest(ctx context.Context, request *sentinel.Request) *sentinel.Decision {
	// Request body inspection for POST/PUT requests with JSON
	if (request.IsPost() || request.IsPut()) && request.IsJSON() {
		return sentinel.Allow().NeedsMoreData()
	}
	return sentinel.Allow()
}

// OnRequestBody inspects the request body.
func (a *BodyInspectionAgent) OnRequestBody(ctx context.Context, request *sentinel.Request) *sentinel.Decision {
	body := request.Body()

	// Validate JSON
	if request.IsJSON() {
		var data interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			return sentinel.Block(400).
				WithBody("Invalid JSON body").
				WithTag("validation_error")
		}

		// Check for prohibited content
		bodyStr := strings.ToLower(string(body))
		prohibitedWords := []string{"script", "eval", "onclick"}
		for _, word := range prohibitedWords {
			if strings.Contains(bodyStr, word) {
				return sentinel.Deny().
					WithBody("Request contains prohibited content").
					WithTag("security").
					WithRuleID("PROHIBITED_CONTENT")
			}
		}
	}

	return sentinel.Allow().AddRequestHeader("X-Body-Validated", "true")
}

// OnResponse checks if response body inspection is needed.
func (a *BodyInspectionAgent) OnResponse(ctx context.Context, request *sentinel.Request, response *sentinel.Response) *sentinel.Decision {
	// Add security headers to HTML responses
	if response.IsHTML() {
		return sentinel.Allow().
			AddResponseHeader("X-Content-Type-Options", "nosniff").
			AddResponseHeader("X-Frame-Options", "DENY").
			AddResponseHeader("X-XSS-Protection", "1; mode=block")
	}
	return sentinel.Allow()
}

// OnResponseBody inspects the response body.
func (a *BodyInspectionAgent) OnResponseBody(ctx context.Context, request *sentinel.Request, response *sentinel.Response) *sentinel.Decision {
	// Could add response body inspection here
	return sentinel.Allow()
}

func main() {
	sentinel.RunAgent(&BodyInspectionAgent{})
}
