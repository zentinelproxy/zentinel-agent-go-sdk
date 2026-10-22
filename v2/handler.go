package v2

import (
	"context"
	"sync"
	"time"

	zentinel "github.com/zentinelproxy/zentinel-agent-go-sdk"
)

// requestStateTTL bounds how long the handler remembers a request whose end
// it never saw (an agent that does not subscribe to request_complete).
const requestStateTTL = 10 * time.Minute

type requestState struct {
	request      *zentinel.Request
	body         []byte
	response     *zentinel.ResponseHeadersEvent
	responseBody []byte
	cancel       context.CancelFunc
	touched      time.Time
}

// AgentHandlerV2 routes protocol events to an AgentV2, independent of the
// transport. One handler serves all connections of a runner: the proxy pools
// connections and may deliver a request's events on different ones.
type AgentHandlerV2 struct {
	agent   AgentV2
	metrics *MetricsCollector

	mu        sync.Mutex
	requests  map[string]*requestState
	lastSweep time.Time
}

// NewAgentHandlerV2 creates a handler for agent.
func NewAgentHandlerV2(agent AgentV2) *AgentHandlerV2 {
	return &AgentHandlerV2{
		agent:     agent,
		metrics:   NewMetricsCollector(),
		requests:  make(map[string]*requestState),
		lastSweep: time.Now(),
	}
}

// Agent returns the agent this handler serves.
func (h *AgentHandlerV2) Agent() AgentV2 { return h.agent }

// state returns the state of a request, creating it if needed. The caller
// holds h.mu.
func (h *AgentHandlerV2) state(correlationID string) *requestState {
	now := time.Now()
	if now.Sub(h.lastSweep) > time.Minute {
		for id, st := range h.requests {
			if now.Sub(st.touched) > requestStateTTL {
				delete(h.requests, id)
			}
		}
		h.lastSweep = now
	}
	st, ok := h.requests[correlationID]
	if !ok {
		// A body or response event for a request whose headers this agent
		// did not see: give the agent a request that carries at least the
		// correlation id rather than skipping its handler.
		st = &requestState{request: zentinel.NewRequest(&zentinel.RequestHeadersEvent{
			Metadata: zentinel.RequestMetadata{CorrelationID: correlationID, RequestID: correlationID},
			Headers:  map[string][]string{},
		}, nil)}
		h.requests[correlationID] = st
	}
	st.touched = now
	return st
}

func (h *AgentHandlerV2) forget(correlationID string) {
	h.mu.Lock()
	if st, ok := h.requests[correlationID]; ok {
		if st.cancel != nil {
			st.cancel()
		}
		delete(h.requests, correlationID)
	}
	h.mu.Unlock()
}

func (h *AgentHandlerV2) requestContext(ctx context.Context, correlationID string) context.Context {
	reqCtx, cancel := context.WithCancel(ctx)
	h.mu.Lock()
	st := h.state(correlationID)
	if st.cancel != nil {
		st.cancel()
	}
	st.cancel = cancel
	h.mu.Unlock()
	return reqCtx
}

func isBlocking(resp zentinel.AgentResponse) bool {
	return resp.Decision != "allow"
}

// Configure applies the agent's configuration (the Configure message).
func (h *AgentHandlerV2) Configure(ctx context.Context, config map[string]interface{}) error {
	if config == nil {
		config = map[string]interface{}{}
	}
	return h.agent.OnConfigure(ctx, config)
}

// RequestHeaders handles a request-headers event.
func (h *AgentHandlerV2) RequestHeaders(ctx context.Context, event *zentinel.RequestHeadersEvent) zentinel.AgentResponse {
	if event.Headers == nil {
		event.Headers = map[string][]string{}
	}
	cid := event.Metadata.CorrelationID
	request := zentinel.NewRequest(event, nil)

	h.mu.Lock()
	st := h.state(cid)
	st.request = request
	st.body = nil
	h.mu.Unlock()

	start := time.Now()
	h.metrics.IncrementActive()
	resp := h.agent.OnRequest(h.requestContext(ctx, cid), request).Build()
	h.metrics.DecrementActive()
	h.metrics.RecordRequest(!isBlocking(resp), float64(time.Since(start).Microseconds())/1000)

	if isBlocking(resp) {
		h.forget(cid)
	}
	return resp
}

// RequestBodyChunk handles one chunk of the request body. The agent's
// OnRequestBody sees the whole body, on the last chunk; earlier chunks are
// answered with allow + needs_more.
func (h *AgentHandlerV2) RequestBodyChunk(ctx context.Context, correlationID string, data []byte, isLast bool) zentinel.AgentResponse {
	h.mu.Lock()
	st := h.state(correlationID)
	st.body = append(st.body, data...)
	body := st.body
	request := st.request
	h.mu.Unlock()

	if !isLast {
		return zentinel.Allow().NeedsMoreData().Build()
	}
	resp := h.agent.OnRequestBody(ctx, request.WithBody(body)).Build()
	if isBlocking(resp) {
		h.forget(correlationID)
	}
	return resp
}

// ResponseHeaders handles a response-headers event.
func (h *AgentHandlerV2) ResponseHeaders(ctx context.Context, event *zentinel.ResponseHeadersEvent) zentinel.AgentResponse {
	if event.Headers == nil {
		event.Headers = map[string][]string{}
	}
	h.mu.Lock()
	st := h.state(event.CorrelationID)
	st.response = event
	st.responseBody = nil
	request := st.request
	h.mu.Unlock()

	return h.agent.OnResponse(ctx, request, zentinel.NewResponse(event, nil)).Build()
}

// ResponseBodyChunk handles one chunk of the response body, like
// RequestBodyChunk.
func (h *AgentHandlerV2) ResponseBodyChunk(ctx context.Context, correlationID string, data []byte, isLast bool) zentinel.AgentResponse {
	h.mu.Lock()
	st := h.state(correlationID)
	st.responseBody = append(st.responseBody, data...)
	body := st.responseBody
	request := st.request
	response := st.response
	h.mu.Unlock()

	if !isLast {
		return zentinel.Allow().NeedsMoreData().Build()
	}
	if response == nil {
		response = &zentinel.ResponseHeadersEvent{CorrelationID: correlationID, Headers: map[string][]string{}}
	}
	return h.agent.OnResponseBody(ctx, request, zentinel.NewResponse(response, body)).Build()
}

// RequestComplete handles the end of a request and forgets its state.
func (h *AgentHandlerV2) RequestComplete(ctx context.Context, event *zentinel.RequestCompleteEvent) zentinel.AgentResponse {
	h.mu.Lock()
	st, ok := h.requests[event.CorrelationID]
	h.mu.Unlock()
	if ok {
		h.agent.OnRequestComplete(ctx, st.request, event.Status, event.DurationMS)
	}
	h.forget(event.CorrelationID)
	return zentinel.Allow().Build()
}

// GuardrailInspect handles a guardrail inspection.
func (h *AgentHandlerV2) GuardrailInspect(ctx context.Context, event *zentinel.GuardrailInspectEvent) *zentinel.GuardrailResponse {
	return h.agent.OnGuardrailInspect(ctx, event)
}

// Cancel cancels the context of an in-flight request and tells the agent.
func (h *AgentHandlerV2) Cancel(ctx context.Context, correlationID string) {
	h.forget(correlationID)
	h.agent.OnCancel(ctx, correlationID)
}

// Cleanup forgets the state kept for a request.
func (h *AgentHandlerV2) Cleanup(correlationID string) {
	h.forget(correlationID)
}
