# API Reference

## Agent

The interface for all Zentinel agents.

```go
import zentinel "github.com/zentinelproxy/zentinel-agent-go-sdk"
```

### Required Methods

#### `Name()`

```go
Name() string
```

Returns the agent identifier used for logging.

### Event Handlers

#### `OnConfigure`

```go
OnConfigure(ctx context.Context, config map[string]interface{}) error
```

Called when the agent receives configuration from the proxy. Override to validate and store configuration.

**Default**: Returns `nil`

#### `OnRequest`

```go
OnRequest(ctx context.Context, request *Request) *Decision
```

Called when request headers are received. This is the main entry point for request processing.

**Default**: Returns `Allow()`

#### `OnRequestBody`

```go
OnRequestBody(ctx context.Context, request *Request) *Decision
```

Called when the request body is available (requires body inspection to be enabled in Zentinel).

**Default**: Returns `Allow()`

#### `OnResponse`

```go
OnResponse(ctx context.Context, request *Request, response *Response) *Decision
```

Called when response headers are received from the upstream server.

**Default**: Returns `Allow()`

#### `OnResponseBody`

```go
OnResponseBody(ctx context.Context, request *Request, response *Response) *Decision
```

Called when the response body is available (requires body inspection to be enabled).

**Default**: Returns `Allow()`

#### `OnRequestComplete`

```go
OnRequestComplete(ctx context.Context, request *Request, status int, durationMS int)
```

Called when request processing is complete. Use for logging or metrics.

### BaseAgent

Embed `BaseAgent` to get default implementations for all optional methods:

```go
type MyAgent struct {
    zentinel.BaseAgent
}
```

---

## ConfigurableAgentBase

A generic agent base with typed configuration support.

```go
type RateLimitConfig struct {
    RequestsPerMinute int  `json:"requests_per_minute"`
    Enabled           bool `json:"enabled"`
}

type RateLimitAgent struct {
    *zentinel.ConfigurableAgentBase[RateLimitConfig]
}

func NewRateLimitAgent() *RateLimitAgent {
    return &RateLimitAgent{
        ConfigurableAgentBase: zentinel.NewConfigurableAgent(RateLimitConfig{
            RequestsPerMinute: 60,
            Enabled:           true,
        }),
    }
}

func (a *RateLimitAgent) Name() string {
    return "rate-limiter"
}

func (a *RateLimitAgent) OnRequest(ctx context.Context, request *zentinel.Request) *zentinel.Decision {
    cfg := a.Config()
    if !cfg.Enabled {
        return zentinel.Allow()
    }
    // Use cfg.RequestsPerMinute...
    return zentinel.Allow()
}
```

### Methods

#### `Config()`

```go
Config() T
```

Returns the current configuration instance.

#### `SetConfig(config T)`

```go
SetConfig(config T)
```

Update the configuration.

---

## Decision

Fluent builder for agent decisions.

```go
import zentinel "github.com/zentinelproxy/zentinel-agent-go-sdk"
```

### Factory Functions

#### `Allow()`

Create an allow decision (pass request through).

```go
return zentinel.Allow()
```

#### `Block(status int)`

Create a block decision with a status code.

```go
return zentinel.Block(403)
return zentinel.Block(500)
```

#### `Deny()`

Shorthand for `Block(403)`.

```go
return zentinel.Deny()
```

#### `Unauthorized()`

Shorthand for `Block(401)`.

```go
return zentinel.Unauthorized()
```

#### `RateLimited()`

Shorthand for `Block(429)`.

```go
return zentinel.RateLimited()
```

#### `Redirect(url string, status int)`

Create a redirect decision.

```go
return zentinel.Redirect("https://example.com/login", 302)
return zentinel.Redirect("https://example.com/new-path", 301)
```

#### `RedirectPermanent(url string)`

Shorthand for `Redirect(url, 301)`.

```go
return zentinel.RedirectPermanent("https://example.com/new-path")
```

#### `Challenge(challengeType string, params map[string]interface{})`

Create a challenge decision (e.g., CAPTCHA).

```go
return zentinel.Challenge("captcha", map[string]interface{}{"site_key": "..."})
```

### Chaining Methods

All methods return `*Decision` for chaining.

#### `WithBody(body string)`

Set the response body for block decisions.

```go
zentinel.Deny().WithBody("Access denied")
```

#### `WithJSONBody(value interface{})`

Set a JSON response body. Automatically sets `Content-Type: application/json`.

```go
zentinel.Block(400).WithJSONBody(map[string]string{"error": "Invalid request"})
```

#### `WithBlockHeader(name, value string)`

Add a header to the block response.

```go
zentinel.Deny().WithBlockHeader("X-Blocked-By", "my-agent")
```

#### `AddRequestHeader(name, value string)`

Add a header to the upstream request.

```go
zentinel.Allow().AddRequestHeader("X-User-ID", "123")
```

#### `RemoveRequestHeader(name string)`

Remove a header from the upstream request.

```go
zentinel.Allow().RemoveRequestHeader("Cookie")
```

#### `AddResponseHeader(name, value string)`

Add a header to the client response.

```go
zentinel.Allow().AddResponseHeader("X-Frame-Options", "DENY")
```

#### `RemoveResponseHeader(name string)`

Remove a header from the client response.

```go
zentinel.Allow().RemoveResponseHeader("Server")
```

### Audit Methods

#### `WithTag(tag string)`

Add an audit tag.

```go
zentinel.Deny().WithTag("security")
```

#### `WithTags(tags []string)`

Add multiple audit tags.

```go
zentinel.Deny().WithTags([]string{"blocked", "rate-limit"})
```

#### `WithRuleID(ruleID string)`

Add a rule ID for audit logging.

```go
zentinel.Deny().WithRuleID("SQLI-001")
```

#### `WithConfidence(confidence float64)`

Set a confidence score (0.0 to 1.0).

```go
zentinel.Deny().WithConfidence(0.95)
```

#### `WithReasonCode(code string)`

Add a reason code.

```go
zentinel.Deny().WithReasonCode("IP_BLOCKED")
```

#### `WithMetadata(key string, value interface{})`

Add custom audit metadata.

```go
zentinel.Deny().WithMetadata("blocked_ip", "192.168.1.100")
```

### Advanced Methods

#### `NeedsMoreData()`

Indicate that more data is needed before deciding.

```go
zentinel.Allow().NeedsMoreData()
```

#### `WithRoutingMetadata(key, value string)`

Add routing metadata for upstream selection.

```go
zentinel.Allow().WithRoutingMetadata("upstream", "backend-v2")
```

#### `WithRequestBodyMutation(data []byte, chunkIndex int)`

Set a mutation for the request body.

```go
zentinel.Allow().WithRequestBodyMutation([]byte("modified body"), 0)
```

#### `WithResponseBodyMutation(data []byte, chunkIndex int)`

Set a mutation for the response body.

```go
zentinel.Allow().WithResponseBodyMutation([]byte("modified body"), 0)
```

---

## Request

Represents an incoming HTTP request.

```go
import zentinel "github.com/zentinelproxy/zentinel-agent-go-sdk"
```

### Methods

#### `Method()`

The HTTP method (GET, POST, etc.).

```go
if request.Method() == "POST" { ... }
```

#### `Path()`

The request path without query string.

```go
path := request.Path() // "/api/users"
```

#### `URI()`

The full URI including query string.

```go
uri := request.URI() // "/api/users?page=1"
```

#### `QueryString()`

The raw query string.

```go
qs := request.QueryString() // "page=1&limit=10"
```

#### `PathStartsWith(prefix string)`

Check if the path starts with a prefix.

```go
if request.PathStartsWith("/api/") { ... }
```

#### `PathEquals(path string)`

Check if the path exactly matches.

```go
if request.PathEquals("/health") { ... }
```

### Header Methods

#### `Header(name string)`

Get a header value (case-insensitive).

```go
auth := request.Header("authorization")
```

#### `HeaderAll(name string)`

Get all values for a header.

```go
accepts := request.HeaderAll("accept")
```

#### `HasHeader(name string)`

Check if a header exists.

```go
if request.HasHeader("Authorization") { ... }
```

#### `Headers()`

Get all headers as a map.

```go
headers := request.Headers()
```

### Common Headers

```go
request.Host()          // Host header
request.UserAgent()     // User-Agent header
request.ContentType()   // Content-Type header
request.Authorization() // Authorization header
```

### Query Methods

#### `Query(name string)`

Get a single query parameter.

```go
page := request.Query("page")
```

#### `QueryAll(name string)`

Get all values for a query parameter.

```go
tags := request.QueryAll("tag")
```

### Body Methods

#### `Body()`

Get the request body as bytes.

```go
body := request.Body()
```

#### `BodyString()`

Get the request body as string.

```go
bodyStr := request.BodyString()
```

#### `BodyJSON(dest interface{})`

Parse the body as JSON.

```go
var payload map[string]interface{}
request.BodyJSON(&payload)
```

### Metadata Methods

```go
request.CorrelationID()  // Request correlation ID
request.RequestID()      // Unique request ID
request.ClientIP()       // Client IP address
request.ClientPort()     // Client port
request.ServerName()     // Server name
request.Protocol()       // HTTP protocol version
```

### Content Type Checks

```go
request.IsJSON()      // Content-Type contains application/json
request.IsForm()      // Content-Type is form-urlencoded
request.IsMultipart() // Content-Type is multipart
```

---

## Response

Represents an HTTP response from the upstream.

```go
import zentinel "github.com/zentinelproxy/zentinel-agent-go-sdk"
```

### Methods

#### `StatusCode()`

The HTTP status code.

```go
if response.StatusCode() == 200 { ... }
```

#### `IsSuccess()`

Check if status is 2xx.

#### `IsRedirect()`

Check if status is 3xx.

#### `IsClientError()`

Check if status is 4xx.

#### `IsServerError()`

Check if status is 5xx.

#### `IsError()`

Check if status is 4xx or 5xx.

### Header Methods

```go
response.Header(name string)
response.HeaderAll(name string)
response.HasHeader(name string)
response.Headers()
```

### Common Headers

```go
response.ContentType()
response.Location()  // For redirects
```

### Content Type Checks

```go
response.IsJSON()
response.IsHTML()
```

### Body Methods

```go
response.Body()
response.BodyString()
response.BodyJSON(dest interface{})
```

---

## AgentRunner

Runner for starting and managing an agent.

```go
import zentinel "github.com/zentinelproxy/zentinel-agent-go-sdk"
```

### Usage

```go
runner := zentinel.NewAgentRunner(&MyAgent{}).
    WithSocket("/tmp/my-agent.sock").
    WithLogLevel("debug")

if err := runner.Run(); err != nil {
    panic(err)
}
```

### Builder Methods

#### `WithName(name string)`

Set the agent name for logging.

#### `WithSocket(path string)`

Set the Unix socket path.

#### `WithJSONLogs()`

Enable JSON log format.

#### `WithLogLevel(level string)`

Set the log level (debug, info, warn, error).

---

## RunAgent

Convenience function to run an agent with CLI argument parsing.

```go
import zentinel "github.com/zentinelproxy/zentinel-agent-go-sdk"

func main() {
    zentinel.RunAgent(&MyAgent{})
}
```

This parses `--socket`, `--log-level`, and `--json-logs` from command line arguments.
