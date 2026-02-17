# Quickstart Guide

This guide will help you create your first Zentinel agent in under 5 minutes.

## Prerequisites

- Go 1.22+
- Zentinel proxy (for testing with real traffic)

## Step 1: Create a New Project

```bash
mkdir my-agent
cd my-agent
go mod init my-agent
go get github.com/zentinelproxy/zentinel-agent-go-sdk
```

## Step 2: Create Your Agent

Create `main.go`:

```go
package main

import (
    "context"
    "fmt"

    zentinel "github.com/zentinelproxy/zentinel-agent-go-sdk"
)

type MyAgent struct {
    zentinel.BaseAgent
}

func (a *MyAgent) Name() string {
    return "my-agent"
}

func (a *MyAgent) OnRequest(ctx context.Context, request *zentinel.Request) *zentinel.Decision {
    // Log the request
    fmt.Printf("Processing: %s %s\n", request.Method(), request.Path())

    // Block requests to sensitive paths
    if request.PathStartsWith("/admin") {
        return zentinel.Deny().
            WithBody("Access denied").
            WithTag("blocked")
    }

    // Allow with a custom header
    return zentinel.Allow().
        AddRequestHeader("X-Processed-By", "my-agent")
}

func main() {
    zentinel.RunAgent(&MyAgent{})
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

## Step 4: Configure Zentinel

Add the agent to your Zentinel configuration (`zentinel.kdl`):

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

With Zentinel running, send a test request:

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
| `--socket PATH` | Unix socket path | `/tmp/zentinel-agent.sock` |
| `--log-level LEVEL` | Log level (debug, info, warn, error) | `info` |
| `--json-logs` | Enable JSON log format | disabled |

## Next Steps

- Read the [API Reference](api.md) for complete documentation
- See [Examples](examples.md) for common patterns
- Learn about [Zentinel Configuration](configuration.md) options
