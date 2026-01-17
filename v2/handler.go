package v2

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"sync"
	"time"

	sentinel "github.com/raskell-io/sentinel-agent-go-sdk"
	"github.com/rs/zerolog/log"
)

// AgentHandlerV2 handles v2 protocol events and routes them to the agent.
type AgentHandlerV2 struct {
	agent AgentV2

	// Request state tracking
	requests       map[uint64]*sentinel.Request
	requestBodies  map[uint64][]byte
	responseBodies map[uint64][]byte
	responseEvents map[uint64]*V2ResponseHeaders
	mu             sync.RWMutex

	// Metrics tracking
	metrics *MetricsCollector

	// Cancellation
	cancelFuncs map[uint64]context.CancelFunc
	cancelMu    sync.Mutex
}

// NewAgentHandlerV2 creates a new v2 handler for the given agent.
func NewAgentHandlerV2(agent AgentV2) *AgentHandlerV2 {
	return &AgentHandlerV2{
		agent:          agent,
		requests:       make(map[uint64]*sentinel.Request),
		requestBodies:  make(map[uint64][]byte),
		responseBodies: make(map[uint64][]byte),
		responseEvents: make(map[uint64]*V2ResponseHeaders),
		metrics:        NewMetricsCollector(),
		cancelFuncs:    make(map[uint64]context.CancelFunc),
	}
}

// HandleMessage handles an incoming v2 protocol message.
func (h *AgentHandlerV2) HandleMessage(ctx context.Context, msg *V2Message) (*V2Message, error) {
	switch msg.Type {
	case MsgTypeHandshakeRequest:
		return h.handleHandshake(ctx, msg)
	case MsgTypeRequestHeaders:
		return h.handleRequestHeaders(ctx, msg)
	case MsgTypeRequestBodyChunk:
		return h.handleRequestBodyChunk(ctx, msg)
	case MsgTypeResponseHeaders:
		return h.handleResponseHeaders(ctx, msg)
	case MsgTypeResponseBodyChunk:
		return h.handleResponseBodyChunk(ctx, msg)
	case MsgTypeCancelRequest:
		return h.handleCancelRequest(ctx, msg)
	case MsgTypeCancelAll:
		return h.handleCancelAll(ctx, msg)
	case MsgTypePing:
		return h.handlePing(ctx, msg)
	case MsgTypeHealthRequest:
		return h.handleHealthRequest(ctx, msg)
	case MsgTypeMetricsRequest:
		return h.handleMetricsRequest(ctx, msg)
	default:
		log.Warn().Str("type", msg.TypeName()).Msg("Unknown v2 message type")
		return h.buildAllowDecision(0)
	}
}

func (h *AgentHandlerV2) handleHandshake(ctx context.Context, msg *V2Message) (*V2Message, error) {
	var req HandshakeRequest
	if err := msg.ParsePayload(&req); err != nil {
		log.Error().Err(err).Msg("Failed to parse handshake request")
		resp := NewHandshakeResponseError(h.agent.Name(), "failed to parse handshake")
		return NewV2Message(MsgTypeHandshakeResponse, resp)
	}

	// Validate protocol version
	if req.ProtocolVersion != ProtocolVersionV2 {
		resp := NewHandshakeResponseError(h.agent.Name(), "unsupported protocol version")
		return NewV2Message(MsgTypeHandshakeResponse, resp)
	}

	log.Info().
		Str("client", req.ClientName).
		Uint32("version", req.ProtocolVersion).
		Msg("Handshake request received")

	resp := NewHandshakeResponse(h.agent.Name(), h.agent.Capabilities())
	return NewV2Message(MsgTypeHandshakeResponse, resp)
}

func (h *AgentHandlerV2) handleRequestHeaders(ctx context.Context, msg *V2Message) (*V2Message, error) {
	var headers V2RequestHeaders
	if err := msg.ParsePayload(&headers); err != nil {
		log.Error().Err(err).Msg("Failed to parse request headers")
		return h.buildAllowDecision(0)
	}

	startTime := time.Now()
	h.metrics.IncrementActive()
	defer h.metrics.DecrementActive()

	// Create cancellable context
	reqCtx, cancel := context.WithCancel(ctx)
	h.cancelMu.Lock()
	h.cancelFuncs[headers.RequestID] = cancel
	h.cancelMu.Unlock()
	defer func() {
		h.cancelMu.Lock()
		delete(h.cancelFuncs, headers.RequestID)
		h.cancelMu.Unlock()
	}()

	// Convert to v1 format for agent interface compatibility
	event := &sentinel.RequestHeadersEvent{
		Metadata: sentinel.RequestMetadata{
			CorrelationID: headers.Metadata.CorrelationID,
			RequestID:     headers.Metadata.CorrelationID,
			ClientIP:      headers.Metadata.ClientIP,
			ClientPort:    headers.Metadata.ClientPort,
			ServerName:    headers.Metadata.ServerName,
			Protocol:      headers.Metadata.Protocol,
			TLSVersion:    headers.Metadata.TLSVersion,
			RouteID:       headers.Metadata.RouteID,
			UpstreamID:    headers.Metadata.UpstreamID,
			Traceparent:   headers.Metadata.Traceparent,
		},
		Method:  headers.Method,
		URI:     headers.URI,
		Headers: headers.Headers,
	}

	request := sentinel.NewRequest(event, nil)

	// Cache request for response correlation
	h.mu.Lock()
	h.requests[headers.RequestID] = request
	h.requestBodies[headers.RequestID] = []byte{}
	h.mu.Unlock()

	decision := h.agent.OnRequest(reqCtx, request)
	elapsed := time.Since(startTime).Seconds() * 1000

	// Record metrics
	response := decision.Build()
	isAllowed := response.Decision == "allow"
	h.metrics.RecordRequest(isAllowed, elapsed)

	return h.buildDecisionMessage(headers.RequestID, decision)
}

func (h *AgentHandlerV2) handleRequestBodyChunk(ctx context.Context, msg *V2Message) (*V2Message, error) {
	var chunk V2RequestBodyChunk
	if err := msg.ParsePayload(&chunk); err != nil {
		log.Error().Err(err).Msg("Failed to parse request body chunk")
		return h.buildAllowDecision(0)
	}

	data, err := base64.StdEncoding.DecodeString(chunk.Data)
	if err != nil {
		log.Error().Err(err).Msg("Failed to decode body chunk data")
		return h.buildAllowDecision(chunk.RequestID)
	}

	// Accumulate body chunks
	h.mu.Lock()
	h.requestBodies[chunk.RequestID] = append(h.requestBodies[chunk.RequestID], data...)
	body := h.requestBodies[chunk.RequestID]
	request := h.requests[chunk.RequestID]
	h.mu.Unlock()

	// Only call handler on last chunk
	if chunk.IsLast && request != nil {
		requestWithBody := request.WithBody(body)
		decision := h.agent.OnRequestBody(ctx, requestWithBody)
		return h.buildDecisionMessage(chunk.RequestID, decision)
	}

	// For non-final chunks, return allow with needs_more
	return h.buildNeedsMoreDecision(chunk.RequestID)
}

func (h *AgentHandlerV2) handleResponseHeaders(ctx context.Context, msg *V2Message) (*V2Message, error) {
	var headers V2ResponseHeaders
	if err := msg.ParsePayload(&headers); err != nil {
		log.Error().Err(err).Msg("Failed to parse response headers")
		return h.buildAllowDecision(0)
	}

	h.mu.RLock()
	request := h.requests[headers.RequestID]
	h.mu.RUnlock()

	if request == nil {
		log.Warn().Uint64("request_id", headers.RequestID).Msg("No cached request for request_id")
		return h.buildAllowDecision(headers.RequestID)
	}

	// Convert to v1 format
	event := &sentinel.ResponseHeadersEvent{
		CorrelationID: request.CorrelationID(),
		Status:        int(headers.StatusCode),
		Headers:       headers.Headers,
	}

	response := sentinel.NewResponse(event, nil)

	// Cache response event for body processing
	h.mu.Lock()
	h.responseEvents[headers.RequestID] = &headers
	h.responseBodies[headers.RequestID] = []byte{}
	h.mu.Unlock()

	decision := h.agent.OnResponse(ctx, request, response)
	return h.buildDecisionMessage(headers.RequestID, decision)
}

func (h *AgentHandlerV2) handleResponseBodyChunk(ctx context.Context, msg *V2Message) (*V2Message, error) {
	var chunk V2ResponseBodyChunk
	if err := msg.ParsePayload(&chunk); err != nil {
		log.Error().Err(err).Msg("Failed to parse response body chunk")
		return h.buildAllowDecision(0)
	}

	data, err := base64.StdEncoding.DecodeString(chunk.Data)
	if err != nil {
		log.Error().Err(err).Msg("Failed to decode response body chunk data")
		return h.buildAllowDecision(chunk.RequestID)
	}

	// Accumulate body chunks
	h.mu.Lock()
	h.responseBodies[chunk.RequestID] = append(h.responseBodies[chunk.RequestID], data...)
	body := h.responseBodies[chunk.RequestID]
	request := h.requests[chunk.RequestID]
	responseEvent := h.responseEvents[chunk.RequestID]
	h.mu.Unlock()

	// Only call handler on last chunk
	if chunk.IsLast && request != nil && responseEvent != nil {
		event := &sentinel.ResponseHeadersEvent{
			CorrelationID: request.CorrelationID(),
			Status:        int(responseEvent.StatusCode),
			Headers:       responseEvent.Headers,
		}
		response := sentinel.NewResponse(event, body)

		decision := h.agent.OnResponseBody(ctx, request, response)
		return h.buildDecisionMessage(chunk.RequestID, decision)
	}

	return h.buildNeedsMoreDecision(chunk.RequestID)
}

func (h *AgentHandlerV2) handleCancelRequest(ctx context.Context, msg *V2Message) (*V2Message, error) {
	var cancel CancelRequestMessage
	if err := msg.ParsePayload(&cancel); err != nil {
		log.Error().Err(err).Msg("Failed to parse cancel request")
		return nil, nil
	}

	log.Debug().Uint64("request_id", cancel.RequestID).Msg("Cancelling request")

	// Cancel the context
	h.cancelMu.Lock()
	if cancelFunc, ok := h.cancelFuncs[cancel.RequestID]; ok {
		cancelFunc()
		delete(h.cancelFuncs, cancel.RequestID)
	}
	h.cancelMu.Unlock()

	// Cleanup cached state
	h.mu.Lock()
	delete(h.requests, cancel.RequestID)
	delete(h.requestBodies, cancel.RequestID)
	delete(h.responseBodies, cancel.RequestID)
	delete(h.responseEvents, cancel.RequestID)
	h.mu.Unlock()

	// Notify agent
	h.agent.OnCancel(ctx, cancel.RequestID)

	// No response for cancel
	return nil, nil
}

func (h *AgentHandlerV2) handleCancelAll(ctx context.Context, msg *V2Message) (*V2Message, error) {
	log.Debug().Msg("Cancelling all requests")

	// Cancel all contexts
	h.cancelMu.Lock()
	for requestID, cancelFunc := range h.cancelFuncs {
		cancelFunc()
		h.agent.OnCancel(ctx, requestID)
	}
	h.cancelFuncs = make(map[uint64]context.CancelFunc)
	h.cancelMu.Unlock()

	// Cleanup all cached state
	h.mu.Lock()
	h.requests = make(map[uint64]*sentinel.Request)
	h.requestBodies = make(map[uint64][]byte)
	h.responseBodies = make(map[uint64][]byte)
	h.responseEvents = make(map[uint64]*V2ResponseHeaders)
	h.mu.Unlock()

	// No response for cancel all
	return nil, nil
}

func (h *AgentHandlerV2) handlePing(ctx context.Context, msg *V2Message) (*V2Message, error) {
	var ping PingMessage
	if err := msg.ParsePayload(&ping); err != nil {
		ping.Timestamp = time.Now().UnixNano()
	}

	pong := PongMessage{Timestamp: ping.Timestamp}
	return NewV2Message(MsgTypePong, pong)
}

func (h *AgentHandlerV2) handleHealthRequest(ctx context.Context, msg *V2Message) (*V2Message, error) {
	health := h.agent.HealthCheck(ctx)
	return NewV2Message(MsgTypeHealthResponse, health)
}

func (h *AgentHandlerV2) handleMetricsRequest(ctx context.Context, msg *V2Message) (*V2Message, error) {
	metrics := h.agent.Metrics(ctx)
	return NewV2Message(MsgTypeMetricsResponse, metrics)
}

func (h *AgentHandlerV2) buildDecisionMessage(requestID uint64, decision *sentinel.Decision) (*V2Message, error) {
	response := decision.Build()

	v2Decision := V2Decision{
		RequestID: requestID,
		Decision:  response.Decision,
	}

	// Convert header operations
	for _, op := range response.RequestHeaders {
		v2Op := V2HeaderOp{
			Operation: op.Operation,
			Name:      op.Name,
			Value:     op.Value,
		}
		v2Decision.RequestHeaders = append(v2Decision.RequestHeaders, v2Op)
	}

	for _, op := range response.ResponseHeaders {
		v2Op := V2HeaderOp{
			Operation: op.Operation,
			Name:      op.Name,
			Value:     op.Value,
		}
		v2Decision.ResponseHeaders = append(v2Decision.ResponseHeaders, v2Op)
	}

	// Add audit metadata
	audit := make(map[string]interface{})
	if len(response.Audit.Tags) > 0 {
		audit["tags"] = response.Audit.Tags
	}
	if len(response.Audit.RuleIDs) > 0 {
		audit["rule_ids"] = response.Audit.RuleIDs
	}
	if response.Audit.Confidence != nil {
		audit["confidence"] = *response.Audit.Confidence
	}
	if len(response.Audit.ReasonCodes) > 0 {
		audit["reason_codes"] = response.Audit.ReasonCodes
	}
	if len(response.Audit.Custom) > 0 {
		audit["custom"] = response.Audit.Custom
	}
	if len(audit) > 0 {
		v2Decision.Audit = audit
	}

	return NewV2Message(MsgTypeDecision, v2Decision)
}

func (h *AgentHandlerV2) buildAllowDecision(requestID uint64) (*V2Message, error) {
	v2Decision := V2Decision{
		RequestID: requestID,
		Decision:  "allow",
	}
	return NewV2Message(MsgTypeDecision, v2Decision)
}

func (h *AgentHandlerV2) buildNeedsMoreDecision(requestID uint64) (*V2Message, error) {
	v2Decision := V2Decision{
		RequestID: requestID,
		Decision: map[string]interface{}{
			"needs_more": true,
		},
	}
	return NewV2Message(MsgTypeDecision, v2Decision)
}

// Cleanup cleans up resources for a completed request.
func (h *AgentHandlerV2) Cleanup(requestID uint64) {
	h.mu.Lock()
	delete(h.requests, requestID)
	delete(h.requestBodies, requestID)
	delete(h.responseBodies, requestID)
	delete(h.responseEvents, requestID)
	h.mu.Unlock()

	h.cancelMu.Lock()
	if cancelFunc, ok := h.cancelFuncs[requestID]; ok {
		cancelFunc()
		delete(h.cancelFuncs, requestID)
	}
	h.cancelMu.Unlock()
}

// HandleLegacyEvent handles a v1 protocol event for backward compatibility.
// This allows v2 agents to also work with v1 protocol.
func (h *AgentHandlerV2) HandleLegacyEvent(ctx context.Context, event map[string]interface{}) (interface{}, error) {
	eventType, _ := event["event_type"].(string)
	payload, _ := event["payload"].(map[string]interface{})

	switch sentinel.EventType(eventType) {
	case sentinel.EventTypeConfigure:
		return h.handleLegacyConfigure(ctx, payload)
	case sentinel.EventTypeRequestHeaders:
		return h.handleLegacyRequestHeaders(ctx, payload)
	case sentinel.EventTypeRequestBodyChunk:
		return h.handleLegacyRequestBodyChunk(ctx, payload)
	case sentinel.EventTypeResponseHeaders:
		return h.handleLegacyResponseHeaders(ctx, payload)
	case sentinel.EventTypeResponseBodyChunk:
		return h.handleLegacyResponseBodyChunk(ctx, payload)
	case sentinel.EventTypeRequestComplete:
		return h.handleLegacyRequestComplete(ctx, payload)
	default:
		log.Warn().Str("event_type", eventType).Msg("Unknown legacy event type")
		return sentinel.Allow().Build(), nil
	}
}

func (h *AgentHandlerV2) handleLegacyConfigure(ctx context.Context, payload map[string]interface{}) (interface{}, error) {
	config, _ := payload["config"].(map[string]interface{})
	if err := h.agent.OnConfigure(ctx, config); err != nil {
		return map[string]interface{}{"success": false, "error": err.Error()}, nil
	}
	return map[string]interface{}{"success": true}, nil
}

func (h *AgentHandlerV2) handleLegacyRequestHeaders(ctx context.Context, payload map[string]interface{}) (interface{}, error) {
	jsonBytes, _ := json.Marshal(payload)
	var event sentinel.RequestHeadersEvent
	if err := json.Unmarshal(jsonBytes, &event); err != nil {
		return sentinel.Allow().Build(), nil
	}

	request := sentinel.NewRequest(&event, nil)
	correlationID := event.Metadata.CorrelationID

	// Use correlation ID hash as request ID for legacy compatibility
	requestID := hashString(correlationID)

	h.mu.Lock()
	h.requests[requestID] = request
	h.requestBodies[requestID] = []byte{}
	h.mu.Unlock()

	decision := h.agent.OnRequest(ctx, request)
	return decision.Build(), nil
}

func (h *AgentHandlerV2) handleLegacyRequestBodyChunk(ctx context.Context, payload map[string]interface{}) (interface{}, error) {
	jsonBytes, _ := json.Marshal(payload)
	var event sentinel.RequestBodyChunkEvent
	if err := json.Unmarshal(jsonBytes, &event); err != nil {
		return sentinel.Allow().Build(), nil
	}

	requestID := hashString(event.CorrelationID)
	data, _ := event.DecodedData()

	h.mu.Lock()
	h.requestBodies[requestID] = append(h.requestBodies[requestID], data...)
	body := h.requestBodies[requestID]
	request := h.requests[requestID]
	h.mu.Unlock()

	if event.IsLast && request != nil {
		requestWithBody := request.WithBody(body)
		decision := h.agent.OnRequestBody(ctx, requestWithBody)
		return decision.Build(), nil
	}

	return sentinel.Allow().NeedsMoreData().Build(), nil
}

func (h *AgentHandlerV2) handleLegacyResponseHeaders(ctx context.Context, payload map[string]interface{}) (interface{}, error) {
	jsonBytes, _ := json.Marshal(payload)
	var event sentinel.ResponseHeadersEvent
	if err := json.Unmarshal(jsonBytes, &event); err != nil {
		return sentinel.Allow().Build(), nil
	}

	requestID := hashString(event.CorrelationID)

	h.mu.RLock()
	request := h.requests[requestID]
	h.mu.RUnlock()

	if request == nil {
		return sentinel.Allow().Build(), nil
	}

	response := sentinel.NewResponse(&event, nil)

	h.mu.Lock()
	h.responseEvents[requestID] = &V2ResponseHeaders{
		RequestID:  requestID,
		StatusCode: uint16(event.Status),
		Headers:    event.Headers,
	}
	h.responseBodies[requestID] = []byte{}
	h.mu.Unlock()

	decision := h.agent.OnResponse(ctx, request, response)
	return decision.Build(), nil
}

func (h *AgentHandlerV2) handleLegacyResponseBodyChunk(ctx context.Context, payload map[string]interface{}) (interface{}, error) {
	jsonBytes, _ := json.Marshal(payload)
	var event sentinel.ResponseBodyChunkEvent
	if err := json.Unmarshal(jsonBytes, &event); err != nil {
		return sentinel.Allow().Build(), nil
	}

	requestID := hashString(event.CorrelationID)
	data, _ := event.DecodedData()

	h.mu.Lock()
	h.responseBodies[requestID] = append(h.responseBodies[requestID], data...)
	body := h.responseBodies[requestID]
	request := h.requests[requestID]
	responseEvent := h.responseEvents[requestID]
	h.mu.Unlock()

	if event.IsLast && request != nil && responseEvent != nil {
		sentinelEvent := &sentinel.ResponseHeadersEvent{
			CorrelationID: request.CorrelationID(),
			Status:        int(responseEvent.StatusCode),
			Headers:       responseEvent.Headers,
		}
		response := sentinel.NewResponse(sentinelEvent, body)
		decision := h.agent.OnResponseBody(ctx, request, response)
		return decision.Build(), nil
	}

	return sentinel.Allow().NeedsMoreData().Build(), nil
}

func (h *AgentHandlerV2) handleLegacyRequestComplete(ctx context.Context, payload map[string]interface{}) (interface{}, error) {
	jsonBytes, _ := json.Marshal(payload)
	var event sentinel.RequestCompleteEvent
	if err := json.Unmarshal(jsonBytes, &event); err != nil {
		return map[string]interface{}{"success": true}, nil
	}

	requestID := hashString(event.CorrelationID)

	h.mu.Lock()
	request := h.requests[requestID]
	delete(h.requests, requestID)
	delete(h.requestBodies, requestID)
	delete(h.responseBodies, requestID)
	delete(h.responseEvents, requestID)
	h.mu.Unlock()

	if request != nil {
		h.agent.OnRequestComplete(ctx, request, event.Status, event.DurationMS)
	}

	return map[string]interface{}{"success": true}, nil
}

// hashString creates a simple uint64 hash from a string.
func hashString(s string) uint64 {
	var h uint64 = 5381
	for i := 0; i < len(s); i++ {
		h = ((h << 5) + h) + uint64(s[i])
	}
	return h
}
