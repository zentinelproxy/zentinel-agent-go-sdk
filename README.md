# Sentinel Agent Go SDK

Build Sentinel proxy agents with less boilerplate in Go.

## Installation

```bash
go get github.com/raskell-io/sentinel-agent-go-sdk
```

## Quick Start

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
    if request.PathStartsWith("/admin") && request.Header("x-admin-token") == "" {
        return sentinel.Deny().WithBody("Admin access required")
    }
    return sentinel.Allow()
}

func main() {
    sentinel.RunAgent(&MyAgent{})
}
```

## Features

- **Simplified types**: `Request`, `Response`, and `Decision` provide ergonomic APIs
- **Fluent decision builder**: Chain methods to build complex responses
- **Configuration handling**: Receive config from proxy's KDL file
- **CLI support**: Built-in argument parsing
- **Logging**: Automatic logging setup with zerolog

## Agent Interface

Implement the `Agent` interface to create a custom agent:

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

Use `BaseAgent` to only implement the methods you need:

```go
type MyAgent struct {
    sentinel.BaseAgent
}

func (a *MyAgent) Name() string { return "my-agent" }

func (a *MyAgent) OnRequest(ctx context.Context, req *sentinel.Request) *sentinel.Decision {
    // Your logic here
    return sentinel.Allow()
}
```

## Request API

The `Request` type provides convenient access to request data:

```go
// Method checks
request.IsGet()
request.IsPost()
request.IsPut()
request.IsDelete()
request.IsPatch()

// Path access
request.Path()           // Full path including query string
request.PathOnly()       // Path without query string
request.QueryString()    // Raw query string
request.PathStartsWith("/api")
request.PathEquals("/health")

// Query parameters
request.Query("page")         // Single value
request.QueryAll("tags")      // All values

// Headers (case-insensitive)
request.Header("Authorization")
request.HeaderAll("Accept")
request.HasHeader("X-Custom")

// Common headers
request.Host()
request.UserAgent()
request.ContentType()
request.Authorization()
request.ContentLength()

// Body access
request.Body()           // []byte
request.BodyString()     // string
request.BodyJSON(&data)  // Parse into struct
request.IsJSON()         // Check content type

// Metadata
request.ClientIP()
request.CorrelationID()
```

## Response API

The `Response` type provides access to response data:

```go
// Status checks
response.StatusCode()
response.IsSuccess()     // 2xx
response.IsRedirect()    // 3xx
response.IsClientError() // 4xx
response.IsServerError() // 5xx
response.IsError()       // 4xx or 5xx

// Headers
response.Header("Content-Type")
response.HeaderAll("Set-Cookie")
response.HasHeader("Cache-Control")
response.ContentType()
response.Location()
response.ContentLength()

// Content type checks
response.IsJSON()
response.IsHTML()

// Body access
response.Body()
response.BodyString()
response.BodyJSON(&data)
```

## Decision Builder

Build decisions with a fluent API:

```go
// Allow request
sentinel.Allow()

// Block request
sentinel.Deny()                  // 403
sentinel.Unauthorized()          // 401
sentinel.RateLimited()          // 429
sentinel.Block(503)             // Custom status

// Redirect
sentinel.Redirect("/login", 302)
sentinel.RedirectPermanent("/new-path")

// Challenge (e.g., CAPTCHA)
sentinel.Challenge("captcha", map[string]interface{}{"provider": "recaptcha"})

// Header mutations
sentinel.Allow().
    AddRequestHeader("X-User-ID", "123").
    RemoveRequestHeader("X-Internal").
    AddResponseHeader("X-Processed-By", "agent").
    RemoveResponseHeader("Server")

// Block response customization
sentinel.Deny().
    WithBody("Access denied").
    WithJSONBody(map[string]string{"error": "forbidden"}).
    WithBlockHeader("X-Error-Code", "AUTH_001")

// Audit metadata
sentinel.Allow().
    WithTag("authenticated").
    WithTags("user", "admin").
    WithRuleID("RULE_001").
    WithConfidence(0.95).
    WithReasonCode("IP_BLOCKED").
    WithMetadata("user_id", "123")

// Body inspection
sentinel.Allow().NeedsMoreData()

// Body mutation
sentinel.Allow().
    WithRequestBodyMutation([]byte("modified"), 0).
    WithResponseBodyMutation([]byte("modified"), 0)

// Routing metadata
sentinel.Allow().WithRoutingMetadata("upstream", "backend-v2")
```

## Configurable Agent

For agents that need configuration:

```go
type MyConfig struct {
    RateLimit int  `json:"rate_limit"`
    Enabled   bool `json:"enabled"`
}

type MyAgent struct {
    *sentinel.ConfigurableAgentBase[MyConfig]
}

func NewMyAgent() *MyAgent {
    return &MyAgent{
        ConfigurableAgentBase: sentinel.NewConfigurableAgent(MyConfig{
            RateLimit: 100,
            Enabled:   true,
        }),
    }
}

func (a *MyAgent) Name() string { return "my-agent" }

func (a *MyAgent) OnConfigApplied(ctx context.Context, config MyConfig) {
    fmt.Printf("Config applied: %+v\n", config)
}

func (a *MyAgent) OnRequest(ctx context.Context, req *sentinel.Request) *sentinel.Decision {
    cfg := a.Config()
    if !cfg.Enabled {
        return sentinel.Allow()
    }
    // Use cfg.RateLimit...
    return sentinel.Allow()
}
```

## Runner Options

Configure the agent runner:

```go
// Using builder pattern
runner := sentinel.NewAgentRunner(agent).
    WithName("my-agent").
    WithSocket("/tmp/my-agent.sock").
    WithJSONLogs().
    WithLogLevel("debug")

runner.Run()

// Or use convenience function with CLI args
sentinel.RunAgent(agent)
```

Command line arguments:
- `--socket` - Unix socket path (default: /tmp/sentinel-agent.sock)
- `--json-logs` - Enable JSON log format
- `--log-level` - Log level (debug, info, warn, error)

## Examples

See the `examples/` directory for complete examples:

- `simple_agent/` - Basic agent that blocks admin paths
- `configurable_agent/` - Rate limiting with configuration
- `body_inspection_agent/` - Request/response body inspection

Run an example:

```bash
go run ./examples/simple_agent --socket /tmp/simple-agent.sock
```

## Development

### Prerequisites

- Go 1.22+
- mise (for version management)

### Setup

```bash
mise install
go mod download
```

### Building

```bash
go build ./...
```

### Testing

```bash
go test ./...
```

## License

MIT
