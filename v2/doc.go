// Package v2 provides the v2 protocol implementation for Sentinel agents.
//
// The v2 protocol adds support for:
//   - Bidirectional streaming
//   - Request cancellation
//   - Multiple transports (gRPC and UDS)
//   - Reverse connections (agent-initiated)
//   - Connection pooling
//   - Health checking and metrics
//   - Capability negotiation via handshake
//
// # Quick Start
//
// Implement the AgentV2 interface:
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
//	        HandleRequestBody().
//	        HandleResponseHeaders()
//	}
//
//	func (a *MyAgent) OnRequest(ctx context.Context, req *sentinel.Request) *sentinel.Decision {
//	    // Your logic here
//	    return sentinel.Allow()
//	}
//
// Run the agent with the v2 runner:
//
//	func main() {
//	    agent := &MyAgent{}
//
//	    // UDS transport (recommended for co-located agents)
//	    v2.RunAgentV2(agent)
//
//	    // Or with gRPC transport
//	    runner := v2.NewAgentRunnerV2(agent).
//	        WithGRPC("localhost:50051")
//	    runner.Run()
//	}
//
// # Lifecycle Hooks
//
// Implement lifecycle hooks for graceful shutdown:
//
//	func (a *MyAgent) OnShutdown(ctx context.Context) {
//	    // Cleanup resources
//	}
//
//	func (a *MyAgent) OnDrain(ctx context.Context) {
//	    // Stop accepting new requests
//	}
//
//	func (a *MyAgent) OnStreamClosed(ctx context.Context, streamID string) {
//	    // Handle stream disconnection
//	}
//
// # Transport Options
//
//   - UDS (Unix Domain Socket): Best for co-located agents, ~0.4ms latency
//   - gRPC: For remote agents or cross-network, ~1.2ms latency
//   - Reverse Connections: For agents behind NAT/firewalls
package v2
