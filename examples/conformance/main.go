// The conformance agent of the Zentinel SDK conformance harness
// (sdk-conformance/README.md in zentinelproxy/zentinel). CI runs it behind a
// real zentinel binary to prove this SDK speaks agent protocol v2.
//
//	go run ./examples/conformance --socket /tmp/conformance.sock
//	go run ./examples/conformance --grpc 127.0.0.1:50051
package main

import (
	"context"
	"strings"
	"sync"

	zentinel "github.com/zentinelproxy/zentinel-agent-go-sdk"
	"github.com/zentinelproxy/zentinel-agent-go-sdk/v2"
)

type ConformanceAgent struct {
	v2.BaseAgentV2
	mu     sync.RWMutex
	config map[string]interface{}
}

func (a *ConformanceAgent) Name() string { return "conformance" }

func (a *ConformanceAgent) Capabilities() *v2.AgentCapabilities {
	return v2.NewAgentCapabilities().HandleRequestHeaders().HandleRequestBody().HandleResponseHeaders()
}

func (a *ConformanceAgent) OnConfigure(ctx context.Context, config map[string]interface{}) error {
	a.mu.Lock()
	a.config = config
	a.mu.Unlock()
	return nil
}

func (a *ConformanceAgent) configValue() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if nested, ok := a.config["nested"].(map[string]interface{}); ok {
		if value, ok := nested["value"].(string); ok {
			return value
		}
	}
	return "unconfigured"
}

func (a *ConformanceAgent) OnRequest(ctx context.Context, req *zentinel.Request) *zentinel.Decision {
	if req.PathStartsWith("/block") {
		return zentinel.Deny().WithBody("blocked-by-agent")
	}
	return zentinel.Allow().
		AddRequestHeader("X-Hello", "from-agent").
		AddRequestHeader("X-Conformance-Config", a.configValue())
}

func (a *ConformanceAgent) OnRequestBody(ctx context.Context, req *zentinel.Request) *zentinel.Decision {
	body := req.BodyString()
	if strings.Contains(body, "block-this-body") {
		return zentinel.Deny().WithBody("body-seen:" + body)
	}
	return zentinel.Allow()
}

func (a *ConformanceAgent) OnResponse(ctx context.Context, req *zentinel.Request, resp *zentinel.Response) *zentinel.Decision {
	return zentinel.Allow().AddResponseHeader("X-Conformance-Response", "from-agent")
}

func main() {
	v2.RunAgentV2(&ConformanceAgent{BaseAgentV2: *v2.NewBaseAgentV2()})
}
