<p align="center">
  <a href="https://zentinelproxy.io">
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset="https://raw.githubusercontent.com/zentinelproxy/design/main/marks/lockup/zentinel-lockup-dark.svg">
      <img alt="Zentinel" src="https://raw.githubusercontent.com/zentinelproxy/design/main/marks/lockup/zentinel-lockup-light.svg" height="52">
    </picture>
  </a>
</p>

<h3 align="center">Go agent SDK</h3>

<p align="center">Write Zentinel agents in Go, speaking agent protocol v2 over a Unix socket or gRPC.</p>

<p align="center">
  <a href="https://github.com/zentinelproxy/zentinel-agent-go-sdk/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/zentinelproxy/zentinel-agent-go-sdk/actions/workflows/ci.yml/badge.svg"></a>
  <img alt="License: Apache-2.0" src="https://img.shields.io/badge/license-Apache--2.0-9b4d8a?style=flat&labelColor=1f161e">
  <img alt="Agent protocol: v2" src="https://img.shields.io/badge/agent%20protocol-v2-f68d31?style=flat&labelColor=1f161e">
  <img alt="Go 1.24+" src="https://img.shields.io/badge/go-1.24%2B-f68d31?style=flat&labelColor=1f161e&logo=go&logoColor=white">
</p>

<p align="center">
  <a href="https://docs.zentinelproxy.io/agents/v2/">Agent protocol</a> &middot;
  <a href="https://docs.zentinelproxy.io">Docs</a> &middot;
  <a href="https://github.com/zentinelproxy">All repositories</a>
</p>

The SDK gives you an `Agent` interface with one method per event, a `Decision`
builder, and in the `v2` package a runner with Unix socket, gRPC and reverse
(agent dials the proxy) transports, health and metrics reporting. It is not
tagged at a usable version; install it from `main`.

> **Status (checked on 2026-10-21 against Zentinel 0.6.50): agents built with this SDK do not connect to Zentinel.**
> The `v2` UDS runner's handshake reply does not match what the proxy expects
> (the proxy rejects it: `invalid type: null, expected struct UdsCapabilities`),
> the `v2` gRPC runner panics at startup (`reflect: non-interface type passed to
> Type.Implements`) and uses a JSON codec rather than protobuf, and the root
> package's `RunAgent` still speaks the v1 framing. Unit tests pass in CI, but
> nothing in the repository exercises a real proxy. The quick start shows the
> intended shape.

## Quick start

In a module that requires the SDK (see [Install](#install)), create `main.go`:

```go
package main

import (
	"context"
	"log"

	zentinel "github.com/zentinelproxy/zentinel-agent-go-sdk"
	"github.com/zentinelproxy/zentinel-agent-go-sdk/v2"
)

type HelloAgent struct {
	v2.BaseAgentV2
}

func (a *HelloAgent) Name() string { return "hello" }

func (a *HelloAgent) OnRequest(ctx context.Context, req *zentinel.Request) *zentinel.Decision {
	return zentinel.Allow().AddRequestHeader("X-Hello", "from-agent")
}

func main() {
	agent := &HelloAgent{BaseAgentV2: *v2.NewBaseAgentV2()}
	if err := v2.NewAgentRunnerV2(agent).WithSocket("/tmp/hello.sock").Run(); err != nil {
		log.Fatal(err)
	}
}
```

Run it with `go run .`. It listens on `/tmp/hello.sock`.

Then point Zentinel at the socket. This assumes your config already defines a
listener and an upstream named `backend`:

```kdl
agents {
    agent "hello" type="custom" {
        unix-socket "/tmp/hello.sock"
        events "request_headers"
        timeout-ms 100
    }
}

filters {
    filter "hello" {
        type "agent"
        agent "hello"
        failure-mode "closed"
    }
}

routes {
    route "default" {
        matches {
            path-prefix "/"
        }
        upstream "backend"
        filters "hello"
    }
}
```

Check the file with `zentinel test -c zentinel.kdl` and start Zentinel. Every
request on the route then reaches `backend` with `X-Hello: from-agent`.
`failure-mode "closed"` on the filter turns an unreachable agent into a 503
instead of letting requests through unprocessed, so a wiring mistake shows up
on the first request.

With this SDK as it is today, that 503 is what you get: see the status note
above.

## Install

```bash
go get github.com/zentinelproxy/zentinel-agent-go-sdk@main
```

Use `@main`. A plain `go get` resolves to the `v0.2.0` tag, whose `go.mod`
still declares the old module path (`github.com/raskell-io/sentinel-agent-go-sdk`),
and fails. The module needs Go 1.24 or later (`go.mod`).

## API overview

- **`Agent`** (root package): `Name`, `OnRequest`, `OnRequestBody`,
  `OnResponse`, `OnResponseBody`, `OnRequestComplete`, `OnConfigure`. Embed
  `BaseAgent` to get allow-everything defaults.
- **`v2.AgentV2`**: adds `Capabilities`, `HealthCheck`, `Metrics`,
  `OnShutdown`, `OnDrain`, `OnCancel`, `OnStreamClosed`. Embed `v2.BaseAgentV2`
  (created with `v2.NewBaseAgentV2()`).
- **`Request` / `Response`**: header, path, query, client IP and body accessors.
- **`Decision`**: `Allow()`, `Deny()`, `Unauthorized()`, `RateLimited()`,
  `Block(status)`, `Redirect(url)`, `Challenge(..)`, then chain
  `AddRequestHeader`, `RemoveRequestHeader`, `AddResponseHeader`, `WithBody`,
  `WithJSONBody`, `WithTag`, `WithRuleID`, `WithConfidence`, `WithMetadata`,
  `WithRoutingMetadata`, body mutations.
- **`v2.NewAgentRunnerV2`**: `WithSocket`, `WithGRPC`, `WithGRPCTLS`,
  `WithReverse`; `v2.RunAgentV2(agent)` reads `--socket`, `--grpc`,
  `--reverse`, `--log-level`, `--json-logs`.

API reference: [pkg.go.dev](https://pkg.go.dev/github.com/zentinelproxy/zentinel-agent-go-sdk).
Examples are in [`examples/`](examples/).

## Development

```bash
mise install          # note: mise.toml pins Go 1.22, but go.mod needs 1.24
go build ./...
go vet ./...
go test -race ./...
gofmt -l .
```

CI runs the same steps with the Go version from `go.mod`.

## Related

- [Agent protocol v2](https://docs.zentinelproxy.io/agents/v2/)
- [Agent configuration](https://docs.zentinelproxy.io/configuration/agents/)
- [Guides in this repository](docs/index.md): quickstart, API, configuration, examples
- [API reference on pkg.go.dev](https://pkg.go.dev/github.com/zentinelproxy/zentinel-agent-go-sdk)
- [Other SDKs](https://github.com/zentinelproxy?q=sdk)
- [Zentinel](https://github.com/zentinelproxy/zentinel)

## License

Apache-2.0, see [`LICENSE`](LICENSE).

---

<p align="center">
  <sub>Part of <a href="https://zentinelproxy.io">Zentinel</a>, the security-first reverse proxy. The shisa guards the gate.</sub>
</p>
