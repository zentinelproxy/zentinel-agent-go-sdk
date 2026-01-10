<div align="center">

<h1 align="center">
  Sentinel Agent Go SDK
</h1>

<p align="center">
  <em>Build agents that extend Sentinel's security and policy capabilities.</em><br>
  <em>Inspect, block, redirect, and transform HTTP traffic.</em>
</p>

<p align="center">
  <a href="https://go.dev/">
    <img alt="Go" src="https://img.shields.io/badge/Go-1.22+-00add8?logo=go&logoColor=white&style=for-the-badge">
  </a>
  <a href="https://github.com/raskell-io/sentinel">
    <img alt="Sentinel" src="https://img.shields.io/badge/Built%20for-Sentinel-f5a97f?style=for-the-badge">
  </a>
  <a href="LICENSE">
    <img alt="License" src="https://img.shields.io/badge/License-Apache--2.0-c6a0f6?style=for-the-badge">
  </a>
</p>

<p align="center">
  <a href="docs/index.md">Documentation</a> •
  <a href="docs/quickstart.md">Quickstart</a> •
  <a href="docs/api.md">API Reference</a> •
  <a href="docs/examples.md">Examples</a>
</p>

</div>

---

The Sentinel Agent Go SDK provides a simple, idiomatic Go API for building agents that integrate with the [Sentinel](https://github.com/raskell-io/sentinel) reverse proxy. Agents can inspect requests and responses, block malicious traffic, add headers, and attach audit metadata—all from Go.

## Quick Start

```bash
go get github.com/raskell-io/sentinel-agent-go-sdk
```

Create `main.go`:

```go
package main

import (
    "context"

    sentinel "github.com/raskell-io/sentinel-agent-go-sdk"
)

type MyAgent struct {
    sentinel.BaseAgent
}

func (a *MyAgent) Name() string {
    return "my-agent"
}

func (a *MyAgent) OnRequest(ctx context.Context, request *sentinel.Request) *sentinel.Decision {
    if request.PathStartsWith("/admin") {
        return sentinel.Deny().WithBody("Access denied")
    }
    return sentinel.Allow()
}

func main() {
    sentinel.RunAgent(&MyAgent{})
}
```

Run the agent:

```bash
go run main.go --socket /tmp/my-agent.sock
```

## Features

| Feature | Description |
|---------|-------------|
| **Simple Agent API** | Implement `OnRequest`, `OnResponse`, and other hooks |
| **Fluent Decision Builder** | Chain methods: `Deny().WithBody(...).WithTag(...)` |
| **Request/Response Wrappers** | Ergonomic access to headers, body, query params, metadata |
| **Typed Configuration** | Generic `ConfigurableAgentBase[T]` with struct tag support |
| **Concurrent Safe** | Built for Go's concurrency model with proper synchronization |
| **Protocol Compatible** | Full compatibility with Sentinel agent protocol v1 |

## Why Agents?

Sentinel's agent system moves complex logic **out of the proxy core** and into isolated, testable, independently deployable processes:

- **Security isolation** — WAF engines, auth validation, and custom logic run in separate processes
- **Language flexibility** — Write agents in Python, Rust, Go, or any language
- **Independent deployment** — Update agent logic without restarting the proxy
- **Failure boundaries** — Agent crashes don't take down the dataplane

Agents communicate with Sentinel over Unix sockets using a simple length-prefixed JSON protocol.

## Architecture

```
┌─────────────┐         ┌──────────────┐         ┌──────────────┐
│   Client    │────────▶│   Sentinel   │────────▶│   Upstream   │
└─────────────┘         └──────────────┘         └──────────────┘
                               │
                               │ Unix Socket (JSON)
                               ▼
                        ┌──────────────┐
                        │    Agent     │
                        │     (Go)     │
                        └──────────────┘
```

1. Client sends request to Sentinel
2. Sentinel forwards request headers to agent
3. Agent returns decision (allow, block, redirect) with optional header mutations
4. Sentinel applies the decision
5. Agent can also inspect response headers before they reach the client

---

## Core Concepts

### Agent

The `Agent` interface defines the hooks you can implement:

```go
package main

import (
    "context"

    sentinel "github.com/raskell-io/sentinel-agent-go-sdk"
)

type MyAgent struct {
    sentinel.BaseAgent // Embed for default implementations
}

// Required: Agent identifier for logging
func (a *MyAgent) Name() string {
    return "my-agent"
}

// Called when request headers arrive
func (a *MyAgent) OnRequest(ctx context.Context, request *sentinel.Request) *sentinel.Decision {
    return sentinel.Allow()
}

// Called when request body is available (if body inspection enabled)
func (a *MyAgent) OnRequestBody(ctx context.Context, request *sentinel.Request) *sentinel.Decision {
    return sentinel.Allow()
}

// Called when response headers arrive from upstream
func (a *MyAgent) OnResponse(ctx context.Context, request *sentinel.Request, response *sentinel.Response) *sentinel.Decision {
    return sentinel.Allow()
}

// Called when response body is available (if body inspection enabled)
func (a *MyAgent) OnResponseBody(ctx context.Context, request *sentinel.Request, response *sentinel.Response) *sentinel.Decision {
    return sentinel.Allow()
}

// Called when request processing completes. Use for logging/metrics
func (a *MyAgent) OnRequestComplete(ctx context.Context, request *sentinel.Request, status int, durationMS int) {
}
```

### Request

Access HTTP request data with convenience methods:

```go
func (a *MyAgent) OnRequest(ctx context.Context, request *sentinel.Request) *sentinel.Decision {
    // Path matching
    if request.PathStartsWith("/api/") {
        // ...
    }
    if request.PathEquals("/health") {
        return sentinel.Allow()
    }

    // Headers (case-insensitive)
    auth := request.Header("authorization")
    if !request.HasHeader("x-api-key") {
        return sentinel.Unauthorized()
    }

    // Common headers as methods
    host := request.Host()
    userAgent := request.UserAgent()
    contentType := request.ContentType()

    // Query parameters
    page := request.Query("page")
    tags := request.QueryAll("tag")

    // Request metadata
    clientIP := request.ClientIP()
    correlationID := request.CorrelationID()

    // Body (when body inspection is enabled)
    if len(request.Body()) > 0 {
        data := request.BodyString()
        // Or parse JSON
        var payload map[string]interface{}
        request.BodyJSON(&payload)
    }

    return sentinel.Allow()
}
```

### Response

Inspect upstream responses before they reach the client:

```go
func (a *MyAgent) OnResponse(ctx context.Context, request *sentinel.Request, response *sentinel.Response) *sentinel.Decision {
    // Status code
    if response.StatusCode() >= 500 {
        return sentinel.Allow().WithTag("upstream-error")
    }

    // Headers
    contentType := response.Header("content-type")

    // Add security headers to all responses
    return sentinel.Allow().
        AddResponseHeader("X-Frame-Options", "DENY").
        AddResponseHeader("X-Content-Type-Options", "nosniff").
        RemoveResponseHeader("Server")
}
```

### Decision

Build responses with a fluent API:

```go
// Allow the request
sentinel.Allow()

// Block with common status codes
sentinel.Deny()           // 403 Forbidden
sentinel.Unauthorized()   // 401 Unauthorized
sentinel.RateLimited()    // 429 Too Many Requests
sentinel.Block(503)       // Custom status

// Block with response body
sentinel.Deny().WithBody("Access denied")
sentinel.Block(400).WithJSONBody(map[string]string{"error": "Invalid request"})

// Redirect
sentinel.Redirect("/login", 302)           // 302 temporary
sentinel.Redirect("/new-path", 301)        // 301 permanent
sentinel.RedirectPermanent("/new-path")    // 301 permanent

// Modify headers
sentinel.Allow().
    AddRequestHeader("X-User-ID", userID).
    RemoveRequestHeader("Cookie").
    AddResponseHeader("X-Cache", "HIT").
    RemoveResponseHeader("X-Powered-By")

// Audit metadata (appears in Sentinel logs)
sentinel.Deny().
    WithTag("blocked").
    WithRuleID("SQLI-001").
    WithConfidence(0.95).
    WithMetadata("matched_pattern", pattern)
```

### ConfigurableAgent

For agents with typed configuration:

```go
type RateLimitConfig struct {
    RequestsPerMinute int  `json:"requests_per_minute"`
    Enabled           bool `json:"enabled"`
}

type RateLimitAgent struct {
    *sentinel.ConfigurableAgentBase[RateLimitConfig]
}

func NewRateLimitAgent() *RateLimitAgent {
    return &RateLimitAgent{
        ConfigurableAgentBase: sentinel.NewConfigurableAgent(RateLimitConfig{
            RequestsPerMinute: 60,
            Enabled:           true,
        }),
    }
}

func (a *RateLimitAgent) Name() string {
    return "rate-limiter"
}

func (a *RateLimitAgent) OnRequest(ctx context.Context, request *sentinel.Request) *sentinel.Decision {
    cfg := a.Config()
    if !cfg.Enabled {
        return sentinel.Allow()
    }
    // Use cfg.RequestsPerMinute...
    return sentinel.Allow()
}
```

---

## Running Agents

### Command Line

The `RunAgent` helper parses CLI arguments:

```bash
# Basic usage
go run main.go --socket /tmp/my-agent.sock

# With options
go run main.go \
    --socket /tmp/my-agent.sock \
    --log-level debug \
    --json-logs
```

| Option | Description | Default |
|--------|-------------|---------|
| `--socket PATH` | Unix socket path | `/tmp/sentinel-agent.sock` |
| `--log-level LEVEL` | debug, info, warn, error | `info` |
| `--json-logs` | Output logs as JSON | disabled |

### Programmatic

```go
package main

import (
    sentinel "github.com/raskell-io/sentinel-agent-go-sdk"
)

func main() {
    runner := sentinel.NewAgentRunner(&MyAgent{}).
        WithSocket("/tmp/my-agent.sock").
        WithLogLevel("debug").
        WithJSONLogs()

    if err := runner.Run(); err != nil {
        panic(err)
    }
}
```

---

## Sentinel Configuration

Configure Sentinel to connect to your agent:

```kdl
agents {
    agent "my-agent" type="custom" {
        unix-socket path="/tmp/my-agent.sock"
        events "request_headers"
        timeout-ms 100
        failure-mode "open"
    }
}

filters {
    filter "my-filter" {
        type "agent"
        agent "my-agent"
    }
}

routes {
    route "api" {
        matches {
            path-prefix "/api/"
        }
        upstream "backend"
        filters "my-filter"
    }
}
```

### Configuration Options

| Option | Description | Default |
|--------|-------------|---------|
| `unix-socket path="..."` | Path to agent's Unix socket | required |
| `events` | Events to send: `request_headers`, `request_body`, `response_headers`, `response_body` | `request_headers` |
| `timeout-ms` | Timeout for agent calls | `1000` |
| `failure-mode` | `"open"` (allow on failure) or `"closed"` (block on failure) | `"open"` |

See [docs/configuration.md](docs/configuration.md) for complete configuration reference.

---

## Examples

The `examples/` directory contains complete, runnable examples:

| Example | Description |
|---------|-------------|
| [`simple_agent`](examples/simple_agent/main.go) | Basic request blocking and header modification |
| [`configurable_agent`](examples/configurable_agent/main.go) | Rate limiting with typed configuration |
| [`body_inspection_agent`](examples/body_inspection_agent/main.go) | Request and response body inspection |

Run an example:

```bash
go run ./examples/simple_agent --socket /tmp/simple-agent.sock
```

See [docs/examples.md](docs/examples.md) for more patterns: authentication, rate limiting, IP filtering, header transformation, and more.

---

## Development

This project uses [mise](https://mise.jdx.dev/) for tool management.

```bash
# Install tools
mise install

# Download dependencies
go mod download

# Run tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests with coverage
go test -cover ./...

# Build all packages
go build ./...

# Build examples
go build ./examples/...
```

### Without mise

```bash
# Requires Go 1.22+
go mod download
go test ./...
```

### Project Structure

```
sentinel-agent-go-sdk/
├── agent.go              # Agent interface and ConfigurableAgent
├── agent_test.go         # Agent tests
├── decision.go           # Decision builder
├── decision_test.go      # Decision tests
├── protocol.go           # Wire protocol types and encoding
├── protocol_test.go      # Protocol conformance tests
├── request.go            # Request wrapper
├── request_test.go       # Request tests
├── response.go           # Response wrapper
├── response_test.go      # Response tests
├── runner.go             # AgentRunner and CLI handling
├── examples/             # Example agents
│   ├── simple_agent/
│   ├── configurable_agent/
│   └── body_inspection_agent/
├── go.mod
├── go.sum
└── mise.toml
```

---

## Protocol

This SDK implements Sentinel Agent Protocol v1:

- **Transport**: Unix domain sockets (UDS) or gRPC
- **Encoding**: Length-prefixed JSON (4-byte big-endian length prefix) for UDS
- **Max message size**: 10 MB
- **Events**: `configure`, `request_headers`, `request_body_chunk`, `response_headers`, `response_body_chunk`, `request_complete`, `websocket_frame`, `guardrail_inspect`
- **Decisions**: `allow`, `block`, `redirect`, `challenge`

The protocol is designed for low latency and high throughput, with support for streaming body inspection.

For the canonical protocol specification, see the [Sentinel Agent Protocol documentation](https://github.com/raskell-io/sentinel/tree/main/crates/agent-protocol).

---

## Community

- 🐛 [Issues](https://github.com/raskell-io/sentinel-agent-go-sdk/issues) — Bug reports and feature requests
- 💬 [Sentinel Discussions](https://github.com/raskell-io/sentinel/discussions) — Questions and ideas
- 📖 [Sentinel Documentation](https://sentinel.raskell.io/docs) — Proxy documentation

Contributions welcome. Please open an issue to discuss significant changes before submitting a PR.

---

## License

Apache 2.0 — See [LICENSE](LICENSE).
