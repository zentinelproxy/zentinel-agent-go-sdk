// Package v2 provides the v2 protocol implementation for Zentinel agents.
//
// Agents run on one of two transports, both speaking agent protocol v2 as
// the proxy implements it (zentinel-agent-protocol):
//   - Unix domain socket: length-prefixed JSON frames with a message-type
//     byte (ServeUDSConn).
//   - gRPC: the zentinel.agent.v2.AgentServiceV2 service with the protobuf
//     codec (RegisterGRPCService).
//
// On every connection the proxy sends a handshake, then the agent's
// `config { }` block as a Configure message, which reaches
// Agent.OnConfigure. Returning an error rejects the configuration and the
// proxy does not use the connection. OnConfigure runs once per connection
// (the proxy pools connections and reconnects), so it must be idempotent.
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
//	func (a *MyAgent) OnRequest(ctx context.Context, req *zentinel.Request) *zentinel.Decision {
//	    // Your logic here
//	    return zentinel.Allow()
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
//	    // Or with the gRPC transport
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
//   - UDS (Unix domain socket): for agents on the proxy's host.
//   - gRPC: for remote agents.
package v2
