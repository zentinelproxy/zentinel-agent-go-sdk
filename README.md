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
builder, and in the `v2` package a runner with Unix socket and gRPC
transports.

> **Status (checked on 2026-10-22): agents built with this SDK work with Zentinel.**
> Version 0.3.0 (on `main`, not yet tagged) passes the shared
> [conformance harness](https://github.com/zentinelproxy/zentinel/tree/main/sdk-conformance)
> on both transports (handshake, configure, request and response header
> mutations, block, request-body inspection, reconnect), against Zentinel
> 26.10_8. CI runs it on every change. Configuration delivery (your agent's
> `config { }` block reaching `OnConfigure`) needs Zentinel 26.10_8 or later,
> which sends the block on every connection (zentinel#603); with 0.6.50
> (26.10_7) and earlier, `OnConfigure` is never called. Not yet tagged: see
> [Install](#install).

## Quick start

In a module that requires the SDK (see [Install](#install)), create `main.go`:

```go
package main

import (
	"context"

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
	v2.RunAgentV2(&HelloAgent{BaseAgentV2: *v2.NewBaseAgentV2()})
}
```

Run it with `go run . --socket /tmp/hello.sock` (or `--grpc 127.0.0.1:50051`
for the gRPC transport). The same agent is in
[`examples/hello`](examples/hello/main.go), and CI runs it behind a real
Zentinel.

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

For the gRPC transport, use `grpc "http://127.0.0.1:50051"` instead of
`unix-socket`.

## Install

```bash
go get github.com/zentinelproxy/zentinel-agent-go-sdk@main
```

Use `@main` until `v0.3.0` is tagged. A plain `go get` resolves to the
`v0.2.0` tag, whose `go.mod` still declares the old module path
(`github.com/raskell-io/sentinel-agent-go-sdk`) and fails; it also predates the
protocol v2 fixes. The module needs Go 1.24 or later (`go.mod`).

## API overview

- **`Agent`** (root package): `Name`, `OnRequest`, `OnRequestBody`,
  `OnResponse`, `OnResponseBody`, `OnRequestComplete`, `OnConfigure`. Embed
  `BaseAgent` to get allow-everything defaults.
- **`v2.AgentV2`**: adds `Capabilities`, `HealthCheck`, `Metrics`,
  `OnShutdown`, `OnDrain`, `OnCancel`, `OnStreamClosed`. Embed `v2.BaseAgentV2`
  (created with `v2.NewBaseAgentV2()`). `v2.Upgrade(agent)` turns a plain
  `Agent` into one. `OnConfigure` runs on every connection the proxy makes
  (it pools connections and reconnects), so make it idempotent.
- **`Request` / `Response`**: header, path, query, client IP and body accessors.
- **`Decision`**: `Allow()`, `Deny()`, `Unauthorized()`, `RateLimited()`,
  `Block(status)`, `Redirect(url)`, `Challenge(..)`, then chain
  `AddRequestHeader`, `RemoveRequestHeader`, `AddResponseHeader`, `WithBody`,
  `WithJSONBody`, `WithTag`, `WithRuleID`, `WithConfidence`, `WithMetadata`,
  `WithRoutingMetadata`, body mutations.
- **`v2.NewAgentRunnerV2`**: `WithSocket`, `WithGRPC`, `WithGRPCTLS`;
  `v2.RunAgentV2(agent)` reads `--socket`, `--grpc`, `--log-level`,
  `--json-logs`. `v2.RunAgent(agent)` does the same for a plain `Agent`.
- **Lower level**: `v2.ServeUDSConn` serves one Unix socket connection,
  `v2.RegisterGRPCService` registers the gRPC service on your own server.
- **Not supported yet**: health and metrics reports are not sent to the proxy;
  `OnGuardrailInspect` and WebSocket frames are not wired to the transports
  (frames are allowed). Reverse connections, which the proxy does not use,
  were removed in 0.3.0, as was the root package's v1 `RunAgent`.

API reference: [pkg.go.dev](https://pkg.go.dev/github.com/zentinelproxy/zentinel-agent-go-sdk).
Examples are in [`examples/`](examples/).

## Development

```bash
mise install          # Go from mise.toml, the same version as go.mod
go build ./...
go vet ./...
go test -race ./...
gofmt -l .
```

CI runs the same steps with the Go version from `go.mod`, and the
[conformance harness](https://github.com/zentinelproxy/zentinel/tree/main/sdk-conformance)
against `examples/conformance` and `examples/hello`. To run it locally with a
`zentinel` binary from release 26.10_8 or later:

```bash
go build -o /tmp/conformance-agent ./examples/conformance
python3 path/to/zentinel/sdk-conformance/run.py --zentinel-bin path/to/zentinel \
  --agent-cmd '/tmp/conformance-agent --socket {socket}' \
  --grpc-agent-cmd '/tmp/conformance-agent --grpc {grpc}'
```

The generated gRPC code in `v2/proto` comes from the proxy's
`crates/agent-protocol/proto/agent_v2.proto` (`go generate ./v2/proto/`).

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
