package sentinel

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

// ProtocolVersion is the version of the Sentinel agent protocol.
const ProtocolVersion = 1

// MaxMessageSize is the maximum size of a protocol message (10MB).
const MaxMessageSize = 10 * 1024 * 1024

// EventType represents the type of event sent from proxy to agent.
type EventType string

const (
	EventTypeRequestHeaders      EventType = "request_headers"
	EventTypeRequestBodyChunk    EventType = "request_body_chunk"
	EventTypeResponseHeaders     EventType = "response_headers"
	EventTypeResponseBodyChunk   EventType = "response_body_chunk"
	EventTypeRequestComplete     EventType = "request_complete"
	EventTypeWebSocketFrame      EventType = "websocket_frame"
	EventTypeConfigure           EventType = "configure"
	EventTypeGuardrailInspect    EventType = "guardrail_inspect"
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
	BytesReceived int    `json:"bytes_received"`
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
	CorrelationID string  `json:"correlation_id"`
	Status        int     `json:"status"`
	DurationMS    int     `json:"duration_ms"`
	RequestSize   int     `json:"request_size"`
	ResponseSize  int     `json:"response_size"`
	Error         *string `json:"error,omitempty"`
}

// WebSocketFrameEvent represents a WebSocket frame.
type WebSocketFrameEvent struct {
	CorrelationID string `json:"correlation_id"`
	Opcode        int    `json:"opcode"`
	Data          string `json:"data"` // Base64-encoded
	Direction     string `json:"direction"`
	FrameIndex    int    `json:"frame_index"`
}

// DecodedData returns the decoded frame data.
func (e *WebSocketFrameEvent) DecodedData() ([]byte, error) {
	if e.Data == "" {
		return []byte{}, nil
	}
	return base64.StdEncoding.DecodeString(e.Data)
}

// ConfigureEvent contains agent configuration.
type ConfigureEvent struct {
	AgentID string                 `json:"agent_id"`
	Config  map[string]interface{} `json:"config"`
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

// ReadMessage reads a length-prefixed JSON message from a reader.
func ReadMessage(r io.Reader) (map[string]interface{}, error) {
	// Read length prefix (4 bytes, big-endian)
	lengthBuf := make([]byte, 4)
	if _, err := io.ReadFull(r, lengthBuf); err != nil {
		if err == io.EOF {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read message length: %w", err)
	}

	length := binary.BigEndian.Uint32(lengthBuf)
	if length > MaxMessageSize {
		return nil, fmt.Errorf("message size %d exceeds maximum %d", length, MaxMessageSize)
	}

	// Read message body
	msgBuf := make([]byte, length)
	if _, err := io.ReadFull(r, msgBuf); err != nil {
		return nil, fmt.Errorf("failed to read message body: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(msgBuf, &result); err != nil {
		return nil, fmt.Errorf("failed to parse message JSON: %w", err)
	}

	return result, nil
}

// WriteMessage writes a length-prefixed JSON message to a writer.
func WriteMessage(w io.Writer, data interface{}) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	if len(jsonBytes) > MaxMessageSize {
		return fmt.Errorf("message size %d exceeds maximum %d", len(jsonBytes), MaxMessageSize)
	}

	// Write length prefix
	lengthBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuf, uint32(len(jsonBytes)))

	if _, err := w.Write(lengthBuf); err != nil {
		return fmt.Errorf("failed to write message length: %w", err)
	}

	if _, err := w.Write(jsonBytes); err != nil {
		return fmt.Errorf("failed to write message body: %w", err)
	}

	return nil
}
