# Examples

Common patterns and use cases for Sentinel agents.

## Basic Request Blocking

Block requests based on path patterns:

```go
package main

import (
    "context"

    sentinel "github.com/raskell-io/sentinel-agent-go-sdk"
)

type BlockingAgent struct {
    sentinel.BaseAgent
    blockedPaths []string
}

func NewBlockingAgent() *BlockingAgent {
    return &BlockingAgent{
        blockedPaths: []string{"/admin", "/internal", "/.git", "/.env"},
    }
}

func (a *BlockingAgent) Name() string {
    return "blocking-agent"
}

func (a *BlockingAgent) OnRequest(ctx context.Context, request *sentinel.Request) *sentinel.Decision {
    for _, blocked := range a.blockedPaths {
        if request.PathStartsWith(blocked) {
            return sentinel.Deny().
                WithBody("Not Found").
                WithTag("path-blocked")
        }
    }
    return sentinel.Allow()
}

func main() {
    sentinel.RunAgent(NewBlockingAgent())
}
```

## IP-Based Access Control

Block or allow requests based on client IP:

```go
package main

import (
    "context"

    sentinel "github.com/raskell-io/sentinel-agent-go-sdk"
)

type IPFilterAgent struct {
    sentinel.BaseAgent
    allowedIPs map[string]bool
}

func NewIPFilterAgent() *IPFilterAgent {
    return &IPFilterAgent{
        allowedIPs: map[string]bool{
            "10.0.0.1":     true,
            "192.168.1.1":  true,
            "127.0.0.1":    true,
        },
    }
}

func (a *IPFilterAgent) Name() string {
    return "ip-filter"
}

func (a *IPFilterAgent) OnRequest(ctx context.Context, request *sentinel.Request) *sentinel.Decision {
    clientIP := request.ClientIP()

    if a.allowedIPs[clientIP] {
        return sentinel.Allow()
    }

    return sentinel.Deny().
        WithTag("ip-blocked").
        WithMetadata("blocked_ip", clientIP)
}

func main() {
    sentinel.RunAgent(NewIPFilterAgent())
}
```

## Authentication Validation

Validate JWT tokens:

```go
package main

import (
    "context"
    "strings"

    "github.com/golang-jwt/jwt/v5"
    sentinel "github.com/raskell-io/sentinel-agent-go-sdk"
)

type AuthAgent struct {
    sentinel.BaseAgent
    secret []byte
}

func NewAuthAgent(secret string) *AuthAgent {
    return &AuthAgent{secret: []byte(secret)}
}

func (a *AuthAgent) Name() string {
    return "auth-agent"
}

func (a *AuthAgent) OnRequest(ctx context.Context, request *sentinel.Request) *sentinel.Decision {
    // Skip auth for public paths
    if request.PathStartsWith("/public") {
        return sentinel.Allow()
    }

    auth := request.Authorization()
    if !strings.HasPrefix(auth, "Bearer ") {
        return sentinel.Unauthorized().
            WithBody("Missing or invalid Authorization header").
            WithTag("auth-missing")
    }

    tokenString := strings.TrimPrefix(auth, "Bearer ")

    token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
        return a.secret, nil
    })

    if err != nil {
        return sentinel.Unauthorized().
            WithBody("Invalid token").
            WithTag("auth-invalid")
    }

    if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
        userID, _ := claims["sub"].(string)
        role, _ := claims["role"].(string)
        return sentinel.Allow().
            AddRequestHeader("X-User-ID", userID).
            AddRequestHeader("X-User-Role", role)
    }

    return sentinel.Unauthorized().
        WithBody("Invalid token claims").
        WithTag("auth-invalid")
}

func main() {
    sentinel.RunAgent(NewAuthAgent("your-secret-key"))
}
```

## Rate Limiting

Simple in-memory rate limiting:

```go
package main

import (
    "context"
    "strconv"
    "sync"
    "time"

    sentinel "github.com/raskell-io/sentinel-agent-go-sdk"
)

type RateLimitAgent struct {
    sentinel.BaseAgent
    maxRequests   int
    windowSeconds int
    requests      map[string][]time.Time
    mu            sync.Mutex
}

func NewRateLimitAgent() *RateLimitAgent {
    return &RateLimitAgent{
        maxRequests:   100,
        windowSeconds: 60,
        requests:      make(map[string][]time.Time),
    }
}

func (a *RateLimitAgent) Name() string {
    return "rate-limit"
}

func (a *RateLimitAgent) OnRequest(ctx context.Context, request *sentinel.Request) *sentinel.Decision {
    key := request.ClientIP()
    now := time.Now()
    windowStart := now.Add(-time.Duration(a.windowSeconds) * time.Second)

    a.mu.Lock()
    defer a.mu.Unlock()

    // Clean old entries and add current
    var timestamps []time.Time
    for _, t := range a.requests[key] {
        if t.After(windowStart) {
            timestamps = append(timestamps, t)
        }
    }
    timestamps = append(timestamps, now)
    a.requests[key] = timestamps

    if len(timestamps) > a.maxRequests {
        return sentinel.RateLimited().
            WithBody("Too many requests").
            WithTag("rate-limited").
            AddResponseHeader("Retry-After", strconv.Itoa(a.windowSeconds))
    }

    remaining := a.maxRequests - len(timestamps)
    return sentinel.Allow().
        AddResponseHeader("X-RateLimit-Limit", strconv.Itoa(a.maxRequests)).
        AddResponseHeader("X-RateLimit-Remaining", strconv.Itoa(remaining))
}

func main() {
    sentinel.RunAgent(NewRateLimitAgent())
}
```

## Header Modification

Add, remove, or modify headers:

```go
package main

import (
    "context"

    sentinel "github.com/raskell-io/sentinel-agent-go-sdk"
)

type HeaderAgent struct {
    sentinel.BaseAgent
}

func (a *HeaderAgent) Name() string {
    return "header-agent"
}

func (a *HeaderAgent) OnRequest(ctx context.Context, request *sentinel.Request) *sentinel.Decision {
    return sentinel.Allow().
        // Add headers for upstream
        AddRequestHeader("X-Forwarded-By", "sentinel").
        AddRequestHeader("X-Request-ID", request.CorrelationID()).
        // Remove sensitive headers
        RemoveRequestHeader("X-Internal-Token")
}

func (a *HeaderAgent) OnResponse(ctx context.Context, request *sentinel.Request, response *sentinel.Response) *sentinel.Decision {
    return sentinel.Allow().
        // Add security headers
        AddResponseHeader("X-Frame-Options", "DENY").
        AddResponseHeader("X-Content-Type-Options", "nosniff").
        AddResponseHeader("X-XSS-Protection", "1; mode=block").
        // Remove server info
        RemoveResponseHeader("Server").
        RemoveResponseHeader("X-Powered-By")
}

func main() {
    sentinel.RunAgent(&HeaderAgent{})
}
```

## Configurable Agent

Agent with runtime configuration:

```go
package main

import (
    "context"
    "fmt"

    sentinel "github.com/raskell-io/sentinel-agent-go-sdk"
)

type Config struct {
    Enabled      bool     `json:"enabled"`
    BlockedPaths []string `json:"blocked_paths"`
    LogRequests  bool     `json:"log_requests"`
}

type ConfigurableBlocker struct {
    *sentinel.ConfigurableAgentBase[Config]
}

func NewConfigurableBlocker() *ConfigurableBlocker {
    return &ConfigurableBlocker{
        ConfigurableAgentBase: sentinel.NewConfigurableAgent(Config{
            Enabled:      true,
            BlockedPaths: []string{"/admin"},
            LogRequests:  false,
        }),
    }
}

func (a *ConfigurableBlocker) Name() string {
    return "configurable-blocker"
}

func (a *ConfigurableBlocker) OnConfigure(ctx context.Context, config map[string]interface{}) error {
    if err := a.ConfigurableAgentBase.OnConfigure(ctx, config); err != nil {
        return err
    }
    fmt.Printf("Configuration updated: enabled=%v\n", a.Config().Enabled)
    return nil
}

func (a *ConfigurableBlocker) OnRequest(ctx context.Context, request *sentinel.Request) *sentinel.Decision {
    cfg := a.Config()

    if !cfg.Enabled {
        return sentinel.Allow()
    }

    if cfg.LogRequests {
        fmt.Printf("Request: %s %s\n", request.Method(), request.Path())
    }

    for _, blocked := range cfg.BlockedPaths {
        if request.PathStartsWith(blocked) {
            return sentinel.Deny()
        }
    }

    return sentinel.Allow()
}

func main() {
    sentinel.RunAgent(NewConfigurableBlocker())
}
```

## Request Logging

Log all requests with timing:

```go
package main

import (
    "context"
    "fmt"

    sentinel "github.com/raskell-io/sentinel-agent-go-sdk"
)

type LoggingAgent struct {
    sentinel.BaseAgent
}

func (a *LoggingAgent) Name() string {
    return "logging-agent"
}

func (a *LoggingAgent) OnRequest(ctx context.Context, request *sentinel.Request) *sentinel.Decision {
    return sentinel.Allow().
        WithTag("method:" + request.Method()).
        WithMetadata("path", request.Path()).
        WithMetadata("client_ip", request.ClientIP())
}

func (a *LoggingAgent) OnRequestComplete(ctx context.Context, request *sentinel.Request, status int, durationMS int) {
    fmt.Printf("%s - %s %s -> %d (%dms)\n",
        request.ClientIP(),
        request.Method(),
        request.Path(),
        status,
        durationMS,
    )
}

func main() {
    sentinel.RunAgent(&LoggingAgent{})
}
```

## Content-Type Validation

Validate request content types:

```go
package main

import (
    "context"
    "strings"

    sentinel "github.com/raskell-io/sentinel-agent-go-sdk"
)

type ContentTypeAgent struct {
    sentinel.BaseAgent
    allowedTypes map[string]bool
}

func NewContentTypeAgent() *ContentTypeAgent {
    return &ContentTypeAgent{
        allowedTypes: map[string]bool{
            "application/json":                  true,
            "application/x-www-form-urlencoded": true,
            "multipart/form-data":               true,
        },
    }
}

func (a *ContentTypeAgent) Name() string {
    return "content-type-validator"
}

func (a *ContentTypeAgent) OnRequest(ctx context.Context, request *sentinel.Request) *sentinel.Decision {
    // Only check methods with body
    method := request.Method()
    if method != "POST" && method != "PUT" && method != "PATCH" {
        return sentinel.Allow()
    }

    contentType := request.ContentType()
    if contentType == "" {
        return sentinel.Block(400).
            WithBody("Content-Type header required")
    }

    // Check against allowed types (ignore params like charset)
    baseType := strings.ToLower(strings.Split(contentType, ";")[0])
    baseType = strings.TrimSpace(baseType)

    if !a.allowedTypes[baseType] {
        return sentinel.Block(415).
            WithBody("Unsupported Content-Type: " + baseType).
            WithTag("invalid-content-type")
    }

    return sentinel.Allow()
}

func main() {
    sentinel.RunAgent(NewContentTypeAgent())
}
```

## Redirect Agent

Redirect requests to different URLs:

```go
package main

import (
    "context"

    sentinel "github.com/raskell-io/sentinel-agent-go-sdk"
)

type RedirectAgent struct {
    sentinel.BaseAgent
    redirects map[string]string
}

func NewRedirectAgent() *RedirectAgent {
    return &RedirectAgent{
        redirects: map[string]string{
            "/old-path": "/new-path",
            "/legacy":   "/v2/api",
            "/blog":     "https://blog.example.com",
        },
    }
}

func (a *RedirectAgent) Name() string {
    return "redirect-agent"
}

func (a *RedirectAgent) OnRequest(ctx context.Context, request *sentinel.Request) *sentinel.Decision {
    if target, ok := a.redirects[request.Path()]; ok {
        return sentinel.Redirect(target, 302)
    }

    // Redirect HTTP to HTTPS
    proto := request.Header("x-forwarded-proto")
    if proto == "http" {
        httpsURL := "https://" + request.Host() + request.URI()
        return sentinel.RedirectPermanent(httpsURL)
    }

    return sentinel.Allow()
}

func main() {
    sentinel.RunAgent(NewRedirectAgent())
}
```

## Combining Multiple Checks

Agent that performs multiple validations:

```go
package main

import (
    "context"
    "strings"

    sentinel "github.com/raskell-io/sentinel-agent-go-sdk"
)

type SecurityAgent struct {
    sentinel.BaseAgent
    suspiciousPatterns []string
}

func NewSecurityAgent() *SecurityAgent {
    return &SecurityAgent{
        suspiciousPatterns: []string{"/../", "/etc/", "/proc/", ".php"},
    }
}

func (a *SecurityAgent) Name() string {
    return "security-agent"
}

func (a *SecurityAgent) OnRequest(ctx context.Context, request *sentinel.Request) *sentinel.Decision {
    // Check 1: User-Agent required
    if request.UserAgent() == "" {
        return sentinel.Block(400).WithBody("User-Agent required")
    }

    // Check 2: Block suspicious paths
    pathLower := strings.ToLower(request.Path())
    for _, pattern := range a.suspiciousPatterns {
        if strings.Contains(pathLower, pattern) {
            return sentinel.Deny().
                WithTag("path-traversal").
                WithRuleID("SEC-001")
        }
    }

    // Check 3: Block large requests without content-length
    method := request.Method()
    if method == "POST" || method == "PUT" {
        if !request.HasHeader("content-length") {
            return sentinel.Block(411).WithBody("Content-Length required")
        }
    }

    // All checks passed
    return sentinel.Allow().
        WithTag("security-passed").
        AddResponseHeader("X-Security-Check", "passed")
}

func main() {
    sentinel.RunAgent(NewSecurityAgent())
}
```
