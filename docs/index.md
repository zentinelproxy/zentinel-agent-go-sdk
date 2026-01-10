# Sentinel Agent Go SDK

A Go SDK for building agents that integrate with the [Sentinel](https://github.com/raskell-io/sentinel) reverse proxy.

## Overview

Sentinel agents are external processors that can inspect and modify HTTP traffic passing through the Sentinel proxy. They communicate with Sentinel over Unix sockets using a length-prefixed JSON protocol.

Agents can:

- **Inspect requests** - Examine headers, paths, query parameters, and body content
- **Block requests** - Return custom error responses (403, 401, 429, etc.)
- **Redirect requests** - Send clients to different URLs
- **Modify headers** - Add, remove, or modify request/response headers
- **Add audit metadata** - Attach tags, rule IDs, and custom data for logging

## Installation

```bash
go get github.com/raskell-io/sentinel-agent-go-sdk
```

## Quick Example

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
    // Block requests to /admin
    if request.PathStartsWith("/admin") {
        return sentinel.Deny().WithBody("Access denied")
    }

    // Allow everything else
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

## Documentation

- [Quickstart Guide](quickstart.md) - Get up and running in 5 minutes
- [API Reference](api.md) - Complete API documentation
- [Examples](examples.md) - Common patterns and use cases
- [Sentinel Configuration](configuration.md) - How to configure Sentinel to use agents

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌──────────────┐
│   Client    │────▶│   Sentinel   │────▶│   Upstream   │
└─────────────┘     └──────────────┘     └──────────────┘
                           │
                           │ Unix Socket
                           ▼
                    ┌──────────────┐
                    │    Agent     │
                    │     (Go)     │
                    └──────────────┘
```

1. Client sends request to Sentinel
2. Sentinel forwards request headers to agent via Unix socket
3. Agent returns a decision (allow, block, redirect)
4. Sentinel applies the decision and forwards to upstream (if allowed)
5. Agent can also process response headers

## Protocol

The SDK implements version 1 of the Sentinel Agent Protocol:

- **Transport**: Unix domain sockets
- **Encoding**: Length-prefixed JSON (4-byte big-endian length prefix)
- **Max message size**: 10MB

## License

Apache 2.0
