package zentinel

import (
	"encoding/base64"
	"encoding/json"
)

// Version is the version of this SDK.
const Version = "0.3.0"

// ProtocolVersion is the version of the Zentinel agent protocol.
const ProtocolVersion = 2

// MaxMessageSize is the largest message the gRPC transport accepts (10 MB).
// The Unix socket transport accepts up to 16 MB (v2.MaxMessageSizeV2).
const MaxMessageSize = 10 * 1024 * 1024

// EventType represents the type of event sent from proxy to agent.
type EventType string

const (
	EventTypeRequestHeaders    EventType = "request_headers"
	EventTypeRequestBodyChunk  EventType = "request_body_chunk"
	EventTypeResponseHeaders   EventType = "response_headers"
	EventTypeResponseBodyChunk EventType = "response_body_chunk"
	EventTypeRequestComplete   EventType = "request_complete"
	EventTypeWebSocketFrame    EventType = "websocket_frame"
	EventTypeConfigure         EventType = "configure"
	EventTypeGuardrailInspect  EventType = "guardrail_inspect"
)

// GuardrailInspectionType represents the type of guardrail inspection.
type GuardrailInspectionType string

const (
	GuardrailInspectionTypePromptInjection GuardrailInspectionType = "prompt_injection"
	GuardrailInspectionTypePIIDetection    GuardrailInspectionType = "pii_detection"
)

// DetectionSeverity represents the severity level of a detection.
type DetectionSeverity string

const (
	DetectionSeverityLow      DetectionSeverity = "low"
	DetectionSeverityMedium   DetectionSeverity = "medium"
	DetectionSeverityHigh     DetectionSeverity = "high"
	DetectionSeverityCritical DetectionSeverity = "critical"
)

// RequestMetadata contains metadata about the request being processed.
type RequestMetadata struct {
	CorrelationID string  `json:"correlation_id"`
	RequestID     string  `json:"request_id"`
	ClientIP      string  `json:"client_ip"`
	ClientPort    int     `json:"client_port"`
	ServerName    *string `json:"server_name,omitempty"`
	Protocol      string  `json:"protocol"`
	TLSVersion    *string `json:"tls_version,omitempty"`
	TLSCipher     *string `json:"tls_cipher,omitempty"`
	RouteID       *string `json:"route_id,omitempty"`
	UpstreamID    *string `json:"upstream_id,omitempty"`
	Timestamp     *string `json:"timestamp,omitempty"`
	Traceparent   *string `json:"traceparent,omitempty"`
}

// RequestHeadersEvent represents incoming request headers.
type RequestHeadersEvent struct {
	Metadata RequestMetadata     `json:"metadata"`
	Method   string              `json:"method"`
	URI      string              `json:"uri"`
	Headers  map[string][]string `json:"headers"`
}

// RequestBodyChunkEvent represents a request body chunk.
type RequestBodyChunkEvent struct {
	CorrelationID string `json:"correlation_id"`
	Data          string `json:"data"` // Base64-encoded
	ChunkIndex    int    `json:"chunk_index"`
	IsLast        bool   `json:"is_last"`
	TotalSize     *int   `json:"total_size,omitempty"`
	BytesReceived int    `json:"bytes_received"`
}

// DecodedData returns the decoded body data.
func (e *RequestBodyChunkEvent) DecodedData() ([]byte, error) {
	if e.Data == "" {
		return []byte{}, nil
	}
	return base64.StdEncoding.DecodeString(e.Data)
}

// ResponseHeadersEvent represents response headers from upstream.
type ResponseHeadersEvent struct {
	CorrelationID string              `json:"correlation_id"`
	Status        int                 `json:"status"`
	Headers       map[string][]string `json:"headers"`
}

// ResponseBodyChunkEvent represents a response body chunk.
type ResponseBodyChunkEvent struct {
	CorrelationID string `json:"correlation_id"`
	Data          string `json:"data"` // Base64-encoded
	ChunkIndex    int    `json:"chunk_index"`
	IsLast        bool   `json:"is_last"`
	TotalSize     *int   `json:"total_size,omitempty"`
	BytesSent     int    `json:"bytes_sent"`
}

// DecodedData returns the decoded body data.
func (e *ResponseBodyChunkEvent) DecodedData() ([]byte, error) {
	if e.Data == "" {
		return []byte{}, nil
	}
	return base64.StdEncoding.DecodeString(e.Data)
}

// RequestCompleteEvent indicates request processing is complete.
type RequestCompleteEvent struct {
	CorrelationID    string  `json:"correlation_id"`
	Status           int     `json:"status"`
	DurationMS       int     `json:"duration_ms"`
	RequestBodySize  int     `json:"request_body_size"`
	ResponseBodySize int     `json:"response_body_size"`
	UpstreamAttempts int     `json:"upstream_attempts"`
	Error            *string `json:"error,omitempty"`
}

// WebSocketFrameEvent represents a WebSocket frame.
type WebSocketFrameEvent struct {
	CorrelationID  string  `json:"correlation_id"`
	Opcode         string  `json:"opcode"`
	Data           string  `json:"data"` // Base64-encoded
	ClientToServer bool    `json:"client_to_server"`
	FrameIndex     int     `json:"frame_index"`
	Fin            bool    `json:"fin"`
	RouteID        *string `json:"route_id,omitempty"`
	ClientIP       string  `json:"client_ip"`
}

// DecodedData returns the decoded frame data.
func (e *WebSocketFrameEvent) DecodedData() ([]byte, error) {
	if e.Data == "" {
		return []byte{}, nil
	}
	return base64.StdEncoding.DecodeString(e.Data)
}

// ConfigureEvent is the payload of the Configure message the proxy sends
// right after the handshake: the agent's `config { }` block as JSON.
type ConfigureEvent struct {
	Config        map[string]interface{} `json:"config"`
	ConfigVersion *string                `json:"config_version,omitempty"`
}

// TextSpan represents a span of text with start and end positions.
type TextSpan struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// GuardrailDetection represents a single detection from guardrail inspection.
type GuardrailDetection struct {
	Category    string            `json:"category"`
	Description string            `json:"description"`
	Severity    DetectionSeverity `json:"severity"`
	Confidence  *float64          `json:"confidence,omitempty"`
	Span        *TextSpan         `json:"span,omitempty"`
}

// NewGuardrailDetection creates a new detection with required fields.
func NewGuardrailDetection(category, description string) *GuardrailDetection {
	return &GuardrailDetection{
		Category:    category,
		Description: description,
		Severity:    DetectionSeverityMedium,
	}
}

// WithSeverity sets the severity level.
func (d *GuardrailDetection) WithSeverity(severity DetectionSeverity) *GuardrailDetection {
	d.Severity = severity
	return d
}

// WithConfidence sets the confidence score.
func (d *GuardrailDetection) WithConfidence(confidence float64) *GuardrailDetection {
	d.Confidence = &confidence
	return d
}

// WithSpan sets the text span location.
func (d *GuardrailDetection) WithSpan(start, end int) *GuardrailDetection {
	d.Span = &TextSpan{Start: start, End: end}
	return d
}

// GuardrailInspectEvent represents a guardrail inspection request.
type GuardrailInspectEvent struct {
	CorrelationID  string                  `json:"correlation_id"`
	InspectionType GuardrailInspectionType `json:"inspection_type"`
	Content        string                  `json:"content"`
	Model          *string                 `json:"model,omitempty"`
	Categories     []string                `json:"categories"`
	RouteID        *string                 `json:"route_id,omitempty"`
	Metadata       map[string]string       `json:"metadata"`
}

// GuardrailResponse represents the response from guardrail inspection.
type GuardrailResponse struct {
	Detected        bool                  `json:"detected"`
	Confidence      float64               `json:"confidence"`
	Detections      []*GuardrailDetection `json:"detections"`
	RedactedContent *string               `json:"redacted_content,omitempty"`
}

// NewGuardrailResponse creates a clean response indicating nothing detected.
func NewGuardrailResponse() *GuardrailResponse {
	return &GuardrailResponse{
		Detected:   false,
		Confidence: 0.0,
		Detections: []*GuardrailDetection{},
	}
}

// NewGuardrailResponseWithDetection creates a response with a single detection.
func NewGuardrailResponseWithDetection(detection *GuardrailDetection) *GuardrailResponse {
	confidence := 1.0
	if detection.Confidence != nil {
		confidence = *detection.Confidence
	}
	return &GuardrailResponse{
		Detected:   true,
		Confidence: confidence,
		Detections: []*GuardrailDetection{detection},
	}
}

// AddDetection adds a detection to the response.
func (r *GuardrailResponse) AddDetection(detection *GuardrailDetection) *GuardrailResponse {
	r.Detected = true
	if detection.Confidence != nil && *detection.Confidence > r.Confidence {
		r.Confidence = *detection.Confidence
	}
	r.Detections = append(r.Detections, detection)
	return r
}

// WithRedactedContent sets the redacted content for PII detection.
func (r *GuardrailResponse) WithRedactedContent(content string) *GuardrailResponse {
	r.RedactedContent = &content
	return r
}

// ProtocolEvent represents a protocol event from the proxy.
type ProtocolEvent struct {
	EventType EventType              `json:"event_type"`
	Payload   map[string]interface{} `json:"payload"`
}

// HeaderOp represents a header operation (set, add, or remove).
type HeaderOp struct {
	Operation string  `json:"-"`
	Name      string  `json:"-"`
	Value     *string `json:"-"`
}

// MarshalJSON implements custom JSON marshaling for HeaderOp.
func (h HeaderOp) MarshalJSON() ([]byte, error) {
	if h.Operation == "remove" {
		return json.Marshal(map[string]interface{}{
			"remove": map[string]string{"name": h.Name},
		})
	}
	value := ""
	if h.Value != nil {
		value = *h.Value
	}
	return json.Marshal(map[string]interface{}{
		h.Operation: map[string]string{"name": h.Name, "value": value},
	})
}

// AuditMetadata contains audit information for logging and observability.
type AuditMetadata struct {
	Tags        []string               `json:"tags,omitempty"`
	RuleIDs     []string               `json:"rule_ids,omitempty"`
	Confidence  *float64               `json:"confidence,omitempty"`
	ReasonCodes []string               `json:"reason_codes,omitempty"`
	Custom      map[string]interface{} `json:"custom,omitempty"`
}

// ProtocolDecision represents the decision type in the response.
type ProtocolDecision interface{}

// AgentResponse is the response from agent to proxy.
type AgentResponse struct {
	Version              int                    `json:"version"`
	Decision             interface{}            `json:"decision"`
	RequestHeaders       []HeaderOp             `json:"request_headers"`
	ResponseHeaders      []HeaderOp             `json:"response_headers"`
	RoutingMetadata      map[string]string      `json:"routing_metadata"`
	Audit                AuditMetadata          `json:"audit"`
	NeedsMore            bool                   `json:"needs_more"`
	RequestBodyMutation  map[string]interface{} `json:"request_body_mutation,omitempty"`
	ResponseBodyMutation map[string]interface{} `json:"response_body_mutation,omitempty"`
	WebSocketDecision    map[string]interface{} `json:"websocket_decision,omitempty"`
}

// NewAllowResponse creates a default allow response.
func NewAllowResponse() AgentResponse {
	return AgentResponse{
		Version:         ProtocolVersion,
		Decision:        "allow",
		RequestHeaders:  []HeaderOp{},
		ResponseHeaders: []HeaderOp{},
		RoutingMetadata: map[string]string{},
		Audit:           AuditMetadata{},
	}
}
