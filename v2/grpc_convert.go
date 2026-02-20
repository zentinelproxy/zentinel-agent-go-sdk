package v2

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// grpcProxyToAgentMessage is the JSON representation of a ProxyToAgent gRPC message.
// This mirrors the proto oneof structure using JSON fields.
type grpcProxyToAgentMessage struct {
	Handshake       *grpcHandshakeRequest    `json:"handshake,omitempty"`
	RequestHeaders  *grpcRequestHeadersEvent `json:"request_headers,omitempty"`
	RequestBody     *grpcBodyChunkEvent      `json:"request_body_chunk,omitempty"`
	ResponseHeaders *grpcResponseHeadersEvent `json:"response_headers,omitempty"`
	ResponseBody    *grpcBodyChunkEvent      `json:"response_body_chunk,omitempty"`
	Cancel          *grpcCancelRequest       `json:"cancel,omitempty"`
	Configure       *grpcConfigureEvent      `json:"configure,omitempty"`
	Ping            *grpcPing                `json:"ping,omitempty"`
	RequestComplete *grpcRequestCompleteEvent `json:"request_complete,omitempty"`
}

// grpcAgentToProxyMessage is the JSON representation of an AgentToProxy gRPC message.
type grpcAgentToProxyMessage struct {
	Handshake *grpcHandshakeResponse `json:"handshake,omitempty"`
	Response  *grpcAgentResponse     `json:"response,omitempty"`
	Health    *grpcHealthStatus      `json:"health,omitempty"`
	Metrics   json.RawMessage        `json:"metrics,omitempty"`
	Pong      *grpcPong              `json:"pong,omitempty"`
}

// grpcHandshakeRequest maps from the proto HandshakeRequest.
type grpcHandshakeRequest struct {
	SupportedVersions []uint32 `json:"supported_versions"`
	ProxyID           string   `json:"proxy_id"`
	ProxyVersion      string   `json:"proxy_version"`
	ConfigJSON        string   `json:"config_json,omitempty"`
}

// grpcHandshakeResponse maps to the proto HandshakeResponse.
type grpcHandshakeResponse struct {
	ProtocolVersion uint32                  `json:"protocol_version"`
	Capabilities    *grpcAgentCapabilities  `json:"capabilities,omitempty"`
	Success         bool                    `json:"success"`
	Error           *string                 `json:"error,omitempty"`
}

// grpcAgentCapabilities maps to the proto AgentCapabilities.
type grpcAgentCapabilities struct {
	ProtocolVersion uint32          `json:"protocol_version"`
	AgentID         string          `json:"agent_id"`
	Name            string          `json:"name"`
	Version         string          `json:"version"`
	SupportedEvents []int32         `json:"supported_events,omitempty"`
	Features        *grpcFeatures   `json:"features,omitempty"`
}

// grpcFeatures maps to the proto AgentFeatures.
type grpcFeatures struct {
	StreamingBody      bool   `json:"streaming_body"`
	Websocket          bool   `json:"websocket"`
	Guardrails         bool   `json:"guardrails"`
	ConfigPush         bool   `json:"config_push"`
	MetricsExport      bool   `json:"metrics_export"`
	ConcurrentRequests uint32 `json:"concurrent_requests"`
	Cancellation       bool   `json:"cancellation"`
	FlowControl        bool   `json:"flow_control"`
	HealthReporting    bool   `json:"health_reporting"`
}

// grpcRequestMetadata maps from the proto RequestMetadata.
type grpcRequestMetadata struct {
	CorrelationID string  `json:"correlation_id"`
	RequestID     string  `json:"request_id"`
	ClientIP      string  `json:"client_ip"`
	ClientPort    uint32  `json:"client_port"`
	ServerName    *string `json:"server_name,omitempty"`
	Protocol      string  `json:"protocol"`
	TLSVersion    *string `json:"tls_version,omitempty"`
	RouteID       *string `json:"route_id,omitempty"`
	UpstreamID    *string `json:"upstream_id,omitempty"`
	TimestampMs   uint64  `json:"timestamp_ms"`
	Traceparent   *string `json:"traceparent,omitempty"`
}

// grpcHeader maps from the proto Header.
type grpcHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// grpcRequestHeadersEvent maps from the proto RequestHeadersEvent.
type grpcRequestHeadersEvent struct {
	Metadata    *grpcRequestMetadata `json:"metadata"`
	Method      string               `json:"method"`
	URI         string               `json:"uri"`
	HTTPVersion string               `json:"http_version"`
	Headers     []grpcHeader         `json:"headers"`
}

// grpcResponseHeadersEvent maps from the proto ResponseHeadersEvent.
type grpcResponseHeadersEvent struct {
	CorrelationID string       `json:"correlation_id"`
	StatusCode    uint32       `json:"status_code"`
	Headers       []grpcHeader `json:"headers"`
}

// grpcBodyChunkEvent maps from the proto BodyChunkEvent.
type grpcBodyChunkEvent struct {
	CorrelationID        string  `json:"correlation_id"`
	ChunkIndex           uint32  `json:"chunk_index"`
	Data                 string  `json:"data"` // Base64-encoded bytes
	IsLast               bool    `json:"is_last"`
	TotalSize            *uint64 `json:"total_size,omitempty"`
	BytesTransferred     uint64  `json:"bytes_transferred"`
	ProxyBufferAvailable uint64  `json:"proxy_buffer_available"`
	TimestampMs          uint64  `json:"timestamp_ms"`
}

// grpcCancelRequest maps from the proto CancelRequest.
type grpcCancelRequest struct {
	CorrelationID string `json:"correlation_id"`
	Reason        int32  `json:"reason"`
	TimestampMs   uint64 `json:"timestamp_ms"`
}

// grpcConfigureEvent maps from the proto ConfigureEvent.
type grpcConfigureEvent struct {
	ConfigJSON    string  `json:"config_json"`
	ConfigVersion *string `json:"config_version,omitempty"`
	IsInitial     bool    `json:"is_initial"`
	TimestampMs   uint64  `json:"timestamp_ms"`
}

// grpcPing maps from the proto Ping.
type grpcPing struct {
	Sequence    uint64 `json:"sequence"`
	TimestampMs uint64 `json:"timestamp_ms"`
}

// grpcPong maps to the proto Pong.
type grpcPong struct {
	Sequence        uint64 `json:"sequence"`
	PingTimestampMs uint64 `json:"ping_timestamp_ms"`
	TimestampMs     uint64 `json:"timestamp_ms"`
}

// grpcRequestCompleteEvent maps from the proto RequestCompleteEvent.
type grpcRequestCompleteEvent struct {
	CorrelationID string  `json:"correlation_id"`
	StatusCode    uint32  `json:"status_code"`
	DurationMs    uint64  `json:"duration_ms"`
	BytesReceived uint64  `json:"bytes_received"`
	BytesSent     uint64  `json:"bytes_sent"`
	Upstream      *string `json:"upstream,omitempty"`
	FromCache     bool    `json:"from_cache"`
	Error         *string `json:"error,omitempty"`
}

// grpcAgentResponse maps to the proto AgentResponse.
type grpcAgentResponse struct {
	CorrelationID   string                 `json:"correlation_id"`
	Decision        interface{}            `json:"decision"`
	RequestHeaders  []grpcHeaderOp         `json:"request_headers,omitempty"`
	ResponseHeaders []grpcHeaderOp         `json:"response_headers,omitempty"`
	Audit           map[string]interface{} `json:"audit,omitempty"`
	ProcessingTimeMs *uint64               `json:"processing_time_ms,omitempty"`
	NeedsMore       bool                   `json:"needs_more"`
}

// grpcHeaderOp maps to the proto HeaderOp.
type grpcHeaderOp struct {
	Set    *grpcHeader `json:"set,omitempty"`
	Add    *grpcHeader `json:"add,omitempty"`
	Remove string      `json:"remove,omitempty"`
}

// grpcHealthStatus maps to the proto HealthStatus.
type grpcHealthStatus struct {
	AgentID     string `json:"agent_id"`
	State       int32  `json:"state"`
	Message     string `json:"message,omitempty"`
	TimestampMs uint64 `json:"timestamp_ms"`
}

// grpcProxyToV2Message converts a gRPC ProxyToAgent JSON message into the existing V2Message format
// so it can be processed by the existing handler.HandleMessage().
func grpcProxyToV2Message(data []byte) (*V2Message, error) {
	var msg grpcProxyToAgentMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ProxyToAgent: %w", err)
	}

	if msg.Handshake != nil {
		return convertHandshakeToV2(msg.Handshake)
	}
	if msg.RequestHeaders != nil {
		return convertRequestHeadersToV2(msg.RequestHeaders)
	}
	if msg.RequestBody != nil {
		return convertBodyChunkToV2(msg.RequestBody, MsgTypeRequestBodyChunk)
	}
	if msg.ResponseHeaders != nil {
		return convertResponseHeadersToV2(msg.ResponseHeaders)
	}
	if msg.ResponseBody != nil {
		return convertBodyChunkToV2(msg.ResponseBody, MsgTypeResponseBodyChunk)
	}
	if msg.Cancel != nil {
		return convertCancelToV2(msg.Cancel)
	}
	if msg.Configure != nil {
		return convertConfigureToV2(msg.Configure)
	}
	if msg.Ping != nil {
		return convertPingToV2(msg.Ping)
	}
	if msg.RequestComplete != nil {
		return convertRequestCompleteToV2(msg.RequestComplete)
	}

	return nil, fmt.Errorf("empty ProxyToAgent message: no oneof field set")
}

func convertHandshakeToV2(req *grpcHandshakeRequest) (*V2Message, error) {
	hsReq := HandshakeRequest{
		ProtocolVersion: ProtocolVersionV2,
		ClientName:      req.ProxyID,
	}
	return NewV2Message(MsgTypeHandshakeRequest, hsReq)
}

func convertRequestHeadersToV2(event *grpcRequestHeadersEvent) (*V2Message, error) {
	// Convert flat header list to map
	headers := make(map[string][]string)
	for _, h := range event.Headers {
		headers[h.Name] = append(headers[h.Name], h.Value)
	}

	// Generate a request ID from the correlation ID
	correlationID := ""
	var clientPort int
	var serverName *string
	var protocol string
	var clientIP string
	var tlsVersion *string
	var routeID *string
	var upstreamID *string
	var traceparent *string

	if event.Metadata != nil {
		correlationID = event.Metadata.CorrelationID
		clientIP = event.Metadata.ClientIP
		clientPort = int(event.Metadata.ClientPort)
		serverName = event.Metadata.ServerName
		protocol = event.Metadata.Protocol
		tlsVersion = event.Metadata.TLSVersion
		routeID = event.Metadata.RouteID
		upstreamID = event.Metadata.UpstreamID
		traceparent = event.Metadata.Traceparent
	}

	requestID := hashString(correlationID)

	v2Req := V2RequestHeaders{
		RequestID: requestID,
		Method:    event.Method,
		URI:       event.URI,
		Headers:   headers,
		HasBody:   false,
		Metadata: V2RequestMetadata{
			CorrelationID: correlationID,
			ClientIP:      clientIP,
			ClientPort:    clientPort,
			ServerName:    serverName,
			Protocol:      protocol,
			TLSVersion:    tlsVersion,
			RouteID:       routeID,
			UpstreamID:    upstreamID,
			Traceparent:   traceparent,
		},
	}
	return NewV2Message(MsgTypeRequestHeaders, v2Req)
}

func convertResponseHeadersToV2(event *grpcResponseHeadersEvent) (*V2Message, error) {
	headers := make(map[string][]string)
	for _, h := range event.Headers {
		headers[h.Name] = append(headers[h.Name], h.Value)
	}

	requestID := hashString(event.CorrelationID)

	v2Resp := V2ResponseHeaders{
		RequestID:  requestID,
		StatusCode: uint16(event.StatusCode),
		Headers:    headers,
		HasBody:    false,
	}
	return NewV2Message(MsgTypeResponseHeaders, v2Resp)
}

func convertBodyChunkToV2(event *grpcBodyChunkEvent, msgType byte) (*V2Message, error) {
	requestID := hashString(event.CorrelationID)

	// The data from proto is bytes, transmitted as base64 in JSON.
	// Our existing handler expects base64-encoded data as well.
	data := event.Data
	if data == "" {
		data = base64.StdEncoding.EncodeToString([]byte{})
	}

	if msgType == MsgTypeRequestBodyChunk {
		chunk := V2RequestBodyChunk{
			RequestID:  requestID,
			ChunkIndex: event.ChunkIndex,
			Data:       data,
			IsLast:     event.IsLast,
		}
		return NewV2Message(MsgTypeRequestBodyChunk, chunk)
	}

	chunk := V2ResponseBodyChunk{
		RequestID:  requestID,
		ChunkIndex: event.ChunkIndex,
		Data:       data,
		IsLast:     event.IsLast,
	}
	return NewV2Message(MsgTypeResponseBodyChunk, chunk)
}

func convertCancelToV2(req *grpcCancelRequest) (*V2Message, error) {
	requestID := hashString(req.CorrelationID)
	cancel := CancelRequestMessage{
		RequestID: requestID,
	}
	return NewV2Message(MsgTypeCancelRequest, cancel)
}

func convertConfigureToV2(event *grpcConfigureEvent) (*V2Message, error) {
	// Configure events are handled directly by the gRPC service layer
	// since they require calling agent.OnConfigure() which is not part of
	// the V2Message handler flow. Return nil to signal direct handling.
	return nil, nil
}

func convertPingToV2(ping *grpcPing) (*V2Message, error) {
	p := PingMessage{
		Timestamp: int64(ping.TimestampMs),
	}
	return NewV2Message(MsgTypePing, p)
}

func convertRequestCompleteToV2(event *grpcRequestCompleteEvent) (*V2Message, error) {
	// Request complete doesn't have a direct V2Message equivalent in the existing handler,
	// but we can handle it as a health request or a no-op. The handler ignores unknown types
	// by returning an allow decision. We create a ping-like message that the handler can process.
	// Actually, looking at the handler, it doesn't handle request complete directly via V2,
	// so we return nil to indicate no response is needed.
	return nil, nil
}

// v2MessageToGRPCResponse converts a V2Message (output from handler.HandleMessage) into
// a gRPC AgentToProxy JSON message for sending back over the gRPC stream.
func v2MessageToGRPCResponse(msg *V2Message) ([]byte, error) {
	if msg == nil {
		return nil, nil
	}

	response := &grpcAgentToProxyMessage{}

	switch msg.Type {
	case MsgTypeHandshakeResponse:
		var hsResp HandshakeResponse
		if err := msg.ParsePayload(&hsResp); err != nil {
			return nil, fmt.Errorf("failed to parse handshake response: %w", err)
		}
		grpcResp := convertHandshakeResponseToGRPC(&hsResp)
		response.Handshake = grpcResp

	case MsgTypeDecision:
		var decision V2Decision
		if err := msg.ParsePayload(&decision); err != nil {
			return nil, fmt.Errorf("failed to parse decision: %w", err)
		}
		grpcResp := convertDecisionToGRPC(&decision)
		response.Response = grpcResp

	case MsgTypePong:
		var pong PongMessage
		if err := msg.ParsePayload(&pong); err != nil {
			return nil, fmt.Errorf("failed to parse pong: %w", err)
		}
		response.Pong = &grpcPong{
			PingTimestampMs: uint64(pong.Timestamp),
			TimestampMs:     uint64(time.Now().UnixMilli()),
		}

	case MsgTypeHealthResponse:
		// Pass through the raw JSON as the health status
		var health HealthStatus
		if err := msg.ParsePayload(&health); err != nil {
			return nil, fmt.Errorf("failed to parse health response: %w", err)
		}
		state := int32(1) // HEALTH_STATE_HEALTHY
		switch health.State {
		case HealthStateDegraded:
			state = 2
		case HealthStateUnhealthy:
			state = 4
		}
		response.Health = &grpcHealthStatus{
			State:       state,
			Message:     health.Message,
			TimestampMs: uint64(health.Timestamp.UnixMilli()),
		}

	case MsgTypeMetricsResponse:
		// Pass through raw metrics JSON
		response.Metrics = msg.Payload

	default:
		return nil, fmt.Errorf("unsupported response message type: %s", msg.TypeName())
	}

	return json.Marshal(response)
}

func convertHandshakeResponseToGRPC(resp *HandshakeResponse) *grpcHandshakeResponse {
	grpcResp := &grpcHandshakeResponse{
		ProtocolVersion: resp.ProtocolVersion,
		Success:         resp.Accepted,
	}
	if resp.Error != "" {
		grpcResp.Error = &resp.Error
	}
	if resp.Capabilities != nil {
		caps := resp.Capabilities
		grpcCaps := &grpcAgentCapabilities{
			ProtocolVersion: ProtocolVersionV2,
			Name:            resp.AgentName,
			AgentID:         resp.AgentName,
			Version:         "1.0.0",
		}

		// Build supported events list
		var events []int32
		if caps.HandlesRequestHeaders {
			events = append(events, 1) // EVENT_TYPE_REQUEST_HEADERS
		}
		if caps.HandlesRequestBody {
			events = append(events, 2) // EVENT_TYPE_REQUEST_BODY_CHUNK
		}
		if caps.HandlesResponseHeaders {
			events = append(events, 3) // EVENT_TYPE_RESPONSE_HEADERS
		}
		if caps.HandlesResponseBody {
			events = append(events, 4) // EVENT_TYPE_RESPONSE_BODY_CHUNK
		}
		grpcCaps.SupportedEvents = events

		concurrency := uint32(0)
		if caps.MaxConcurrentRequests != nil {
			concurrency = *caps.MaxConcurrentRequests
		}
		grpcCaps.Features = &grpcFeatures{
			StreamingBody:      caps.SupportsStreaming,
			Cancellation:       caps.SupportsCancellation,
			ConcurrentRequests: concurrency,
			HealthReporting:    true,
			MetricsExport:      true,
		}

		grpcResp.Capabilities = grpcCaps
	}
	return grpcResp
}

func convertDecisionToGRPC(decision *V2Decision) *grpcAgentResponse {
	// Reconstruct correlation ID from the decision. The decision has a RequestID (uint64),
	// but the proto AgentResponse uses a string correlation_id. Since we hashed the correlation ID
	// to create the request ID, we cannot reverse it. Instead, we store the request ID as the
	// correlation ID string for round-trip consistency.
	correlationID := fmt.Sprintf("%d", decision.RequestID)

	resp := &grpcAgentResponse{
		CorrelationID: correlationID,
		Decision:      decision.Decision,
		Audit:         decision.Audit,
	}

	// Check if this is a needs_more decision
	if decisionMap, ok := decision.Decision.(map[string]interface{}); ok {
		if needsMore, ok := decisionMap["needs_more"].(bool); ok && needsMore {
			resp.NeedsMore = true
			resp.Decision = "allow"
		}
	}

	// Convert header operations
	for _, op := range decision.RequestHeaders {
		grpcOp := grpcHeaderOp{}
		switch op.Operation {
		case "set":
			value := ""
			if op.Value != nil {
				value = *op.Value
			}
			grpcOp.Set = &grpcHeader{Name: op.Name, Value: value}
		case "add":
			value := ""
			if op.Value != nil {
				value = *op.Value
			}
			grpcOp.Add = &grpcHeader{Name: op.Name, Value: value}
		case "remove":
			grpcOp.Remove = op.Name
		}
		resp.RequestHeaders = append(resp.RequestHeaders, grpcOp)
	}

	for _, op := range decision.ResponseHeaders {
		grpcOp := grpcHeaderOp{}
		switch op.Operation {
		case "set":
			value := ""
			if op.Value != nil {
				value = *op.Value
			}
			grpcOp.Set = &grpcHeader{Name: op.Name, Value: value}
		case "add":
			value := ""
			if op.Value != nil {
				value = *op.Value
			}
			grpcOp.Add = &grpcHeader{Name: op.Name, Value: value}
		case "remove":
			grpcOp.Remove = op.Name
		}
		resp.ResponseHeaders = append(resp.ResponseHeaders, grpcOp)
	}

	return resp
}
