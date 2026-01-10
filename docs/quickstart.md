# Quickstart Guide

This guide will help you create your first Sentinel agent in under 5 minutes.

## Prerequisites

- Go 1.22+
- Sentinel proxy (for testing with real traffic)

## Step 1: Create a New Project

```bash
mkdir my-agent
cd my-agent
go mod init my-agent
go get github.com/raskell-io/sentinel-agent-go-sdk
```

## Step 2: Create Your Agent

Create `main.go`:

```go
package main

import (
    "context"
    "fmt"

    sentinel "github.com/raskell-io/sentinel-agent-go-sdk"
)

type MyAgent struct {
    sentinel.BaseAgent
}

func (a *MyAgent) Name() string {
    return "my-agent"
}

func (a *MyAgent) OnRequest(ctx context.Context, request *sentinel.Request) *sentinel.Decision {
    // Log the request
    fmt.Printf("Processing: %s %s\n", request.Method(), request.Path())

    // Block requests to sensitive paths
    if request.PathStartsWith("/admin") {
        return sentinel.Deny().
            WithBody("Access denied").
            WithTag("blocked")
    }

    // Allow with a custom header
    return sentinel.Allow().
        AddRequestHeader("X-Processed-By", "my-agent")
}

func main() {
    sentinel.RunAgent(&MyAgent{})
}
```

## Step 3: Run the Agent

```bash
go run main.go --socket /tmp/my-agent.sock --log-level debug
```

You should see:

```
[my-agent] INFO: Agent 'my-agent' listening on /tmp/my-agent.sock
```

## Step 4: Configure Sentinel

Add the agent to your Sentinel configuration (`sentinel.kdl`):

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
        timeout-ms 100
        failure-mode "open"
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

## Step 5: Test It

With Sentinel running, send a test request:

```bash
# This should pass through
curl http://localhost:8080/api/users

# This should be blocked
curl http://localhost:8080/api/admin/settings
```

## Command Line Options

The `RunAgent` function supports these CLI arguments:

| Option | Description | Default |
|--------|-------------|---------|
| `--socket PATH` | Unix socket path | `/tmp/sentinel-agent.sock` |
| `--log-level LEVEL` | Log level (debug, info, warn, error) | `info` |
| `--json-logs` | Enable JSON log format | disabled |

## Next Steps

- Read the [API Reference](api.md) for complete documentation
- See [Examples](examples.md) for common patterns
- Learn about [Sentinel Configuration](configuration.md) options
