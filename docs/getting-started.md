# Getting Started with Zentinel Agent Go SDK

This guide will walk you through creating your first Zentinel agent in Go.

## Prerequisites

- Go 1.21 or later
- A running Zentinel proxy instance (or just the SDK for development)

## Installation

```bash
go get github.com/zentinelproxy/zentinel-agent-go-sdk
```

## Your First Agent

Create a new file `main.go`:

```go
package main

import (
    "context"
    zentinel "github.com/zentinelproxy/zentinel-agent-go-sdk"
)

type MyAgent struct {
    zentinel.BaseAgent
}

func (a *MyAgent) Name() string {
    return "my-agent"
}

func (a *MyAgent) OnRequest(ctx context.Context, request *zentinel.Request) *zentinel.Decision {
    // Block requests to /admin paths
    if request.PathStartsWith("/admin") {
        return zentinel.Deny().WithBody("Access denied")
    }

    // Allow all other requests
    return zentinel.Allow()
}

func main() {
    zentinel.RunAgent(&MyAgent{})
}
```

## Running Your Agent

```bash
go run main.go --socket /tmp/my-agent.sock
```

Your agent is now listening on `/tmp/my-agent.sock` and ready to receive events from Zentinel.

## Understanding the Agent Interface

The `Agent` interface defines the hooks you can implement:

```go
type Agent interface {
    Name() string
    OnConfigure(ctx context.Context, config map[string]interface{}) error
    OnRequest(ctx context.Context, request *Request) *Decision
    OnRequestBody(ctx context.Context, request *Request) *Decision
    OnResponse(ctx context.Context, request *Request, response *Response) *Decision
    OnResponseBody(ctx context.Context, request *Request, response *Response) *Decision
    OnRequestComplete(ctx context.Context, request *Request, status int, durationMS int)
}
```

Embed `zentinel.BaseAgent` to get default implementations for all methods, then override only the ones you need.

## Making Decisions

The `Decision` builder provides a fluent API:

```go
// Allow the request
zentinel.Allow()

// Block with 403 Forbidden
zentinel.Deny()

// Block with custom status
zentinel.Block(429).WithBody("Too many requests")

// Redirect
zentinel.Redirect("/login", 302)
zentinel.RedirectPermanent("/new-path")

// Allow with header modifications
zentinel.Allow().
    AddRequestHeader("X-User-ID", "12345").
    AddResponseHeader("X-Cache", "HIT").
    RemoveResponseHeader("Server")

// Add audit metadata
zentinel.Deny().
    WithTag("security").
    WithRuleID("ADMIN-001").
    WithMetadata("reason", "blocked by rule")
```

## Working with Requests

The `Request` type provides convenient methods:

```go
func (a *MyAgent) OnRequest(ctx context.Context, request *zentinel.Request) *zentinel.Decision {
    // Path inspection
    path := request.Path()
    if request.PathStartsWith("/api/") { /* ... */ }
    if request.PathEquals("/health") { /* ... */ }

    // Headers (case-insensitive)
    auth := request.Header("Authorization")
    userAgent := request.UserAgent()
    contentType := request.ContentType()

    // Request metadata
    clientIP := request.ClientIP()
    method := request.Method()
    correlationID := request.CorrelationID()

    return zentinel.Allow()
}
```

## Working with Responses

Inspect upstream responses:

```go
func (a *MyAgent) OnResponse(ctx context.Context, request *zentinel.Request, response *zentinel.Response) *zentinel.Decision {
    // Check status code
    if response.StatusCode() >= 500 {
        return zentinel.Allow().WithTag("upstream-error")
    }

    // Inspect headers
    contentType := response.Header("Content-Type")

    // Add security headers
    return zentinel.Allow().
        AddResponseHeader("X-Frame-Options", "DENY").
        AddResponseHeader("X-Content-Type-Options", "nosniff")
}
```

## Typed Configuration

For agents with configuration, use generics:

```go
type MyConfig struct {
    RateLimit int  `json:"rate_limit"`
    Enabled   bool `json:"enabled"`
}

type MyAgent struct {
    *zentinel.ConfigurableAgentBase[MyConfig]
}

func NewMyAgent() *MyAgent {
    return &MyAgent{
        ConfigurableAgentBase: zentinel.NewConfigurableAgent(MyConfig{
            RateLimit: 100,
            Enabled:   true,
        }),
    }
}

func (a *MyAgent) Name() string {
    return "my-configurable-agent"
}

func (a *MyAgent) OnRequest(ctx context.Context, request *zentinel.Request) *zentinel.Decision {
    cfg := a.Config()
    if !cfg.Enabled {
        return zentinel.Allow()
    }
    // Use cfg.RateLimit...
    return zentinel.Allow()
}

func (a *MyAgent) OnConfigApplied(ctx context.Context, config MyConfig) {
    fmt.Printf("Config updated: rate_limit=%d\n", config.RateLimit)
}
```

## Connecting to Zentinel

Configure Zentinel to use your agent:

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

## CLI Options

The SDK provides built-in CLI argument parsing:

```bash
# Basic usage
go run main.go --socket /tmp/my-agent.sock

# With options
go run main.go \
    --socket /tmp/my-agent.sock \
    --log-level DEBUG \
    --json-logs
```

| Option | Description | Default |
|--------|-------------|---------|
| `--socket PATH` | Unix socket path | `/tmp/zentinel-agent.sock` |
| `--log-level LEVEL` | debug, info, warn, error | `info` |
| `--json-logs` | Output logs as JSON | disabled |

## Request Logging

Use `OnRequestComplete` for logging and metrics:

```go
func (a *MyAgent) OnRequestComplete(ctx context.Context, request *zentinel.Request, status int, durationMS int) {
    fmt.Printf("%s - %s %s -> %d (%dms)\n",
        request.ClientIP(),
        request.Method(),
        request.Path(),
        status,
        durationMS,
    )
}
```

## Error Handling

Return appropriate decisions for errors:

```go
func (a *MyAgent) OnRequest(ctx context.Context, request *zentinel.Request) *zentinel.Decision {
    token := request.Header("Authorization")
    if token == "" {
        return zentinel.Unauthorized().
            WithBody("Authorization header required").
            WithTag("auth-missing")
    }

    userID, err := validateToken(token)
    if err != nil {
        return zentinel.Unauthorized().
            WithBody("Invalid token").
            WithTag("auth-failed").
            WithMetadata("error", err.Error())
    }

    return zentinel.Allow().AddRequestHeader("X-User-ID", userID)
}
```

## Testing Your Agent

Write unit tests for your agent:

```go
package main

import (
    "context"
    "testing"
    zentinel "github.com/zentinelproxy/zentinel-agent-go-sdk"
)

func TestBlocksAdminPath(t *testing.T) {
    agent := &MyAgent{}
    request := zentinel.NewRequest().WithPath("/admin/users")

    decision := agent.OnRequest(context.Background(), request)

    if decision.IsAllow() {
        t.Error("Expected request to be blocked")
    }
}
```

## Next Steps

- Read the [API Reference](api.md) for complete documentation
- Browse [Examples](../examples/) for common patterns
- See the [Configuration](configuration.md) guide for Zentinel setup

## Need Help?

- [GitHub Issues](https://github.com/zentinelproxy/zentinel-agent-go-sdk/issues)
- [Zentinel Documentation](https://zentinelproxy.io/docs)
