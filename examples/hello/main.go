// The README quick-start agent: adds X-Hello to every request and allows it.
//
//	go run ./examples/hello --socket /tmp/hello.sock
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
