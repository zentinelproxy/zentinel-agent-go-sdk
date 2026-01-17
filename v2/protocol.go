package v2

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

// MaxMessageSizeV2 is the maximum message size for v2 UDS protocol (16MB).
const MaxMessageSizeV2 = 16 * 1024 * 1024

// Message type IDs for v2 binary protocol.
const (
	MsgTypeHandshakeRequest   byte = 0x01
	MsgTypeHandshakeResponse  byte = 0x02
	MsgTypeRequestHeaders     byte = 0x10
	MsgTypeRequestBodyChunk   byte = 0x11
	MsgTypeResponseHeaders    byte = 0x12
	MsgTypeResponseBodyChunk  byte = 0x13
	MsgTypeDecision           byte = 0x20
	MsgTypeBodyMutation       byte = 0x21
	MsgTypeCancelRequest      byte = 0x30
	MsgTypeCancelAll          byte = 0x31
	MsgTypePing               byte = 0xF0
	MsgTypePong               byte = 0xF1
	MsgTypeHealthRequest      byte = 0xE0
	MsgTypeHealthResponse     byte = 0xE1
	MsgTypeMetricsRequest     byte = 0xE2
	MsgTypeMetricsResponse    byte = 0xE3
	MsgTypeRegistration       byte = 0x03
	MsgTypeRegistrationAck    byte = 0x04
)

// V2Message represents a v2 protocol message.
type V2Message struct {
	// Type is the message type ID.
	Type byte `json:"type"`

	// RequestID identifies the request this message belongs to.
	// Not used for handshake, health, and metrics messages.
	RequestID uint64 `json:"request_id,omitempty"`

	// Payload is the message-specific data.
	Payload json.RawMessage `json:"payload"`
}

// CancelRequestMessage requests cancellation of a specific request.
type CancelRequestMessage struct {
	RequestID uint64  `json:"request_id"`
	Reason    *string `json:"reason,omitempty"`
}

// CancelAllMessage requests cancellation of all requests.
type CancelAllMessage struct {
	Reason *string `json:"reason,omitempty"`
}

// PingMessage is a keep-alive ping.
type PingMessage struct {
	Timestamp int64 `json:"timestamp"`
}

// PongMessage is a keep-alive response.
type PongMessage struct {
	Timestamp int64 `json:"timestamp"`
}

// ReadMessageV2 reads a v2 length-prefixed message from a reader.
// Format: [length:4][type:1][payload:variable]
func ReadMessageV2(r io.Reader) (*V2Message, error) {
	// Read length prefix (4 bytes, big-endian, includes type byte)
	lengthBuf := make([]byte, 4)
	if _, err := io.ReadFull(r, lengthBuf); err != nil {
		if err == io.EOF {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read message length: %w", err)
	}

	length := binary.BigEndian.Uint32(lengthBuf)
	if length > MaxMessageSizeV2 {
		return nil, fmt.Errorf("message size %d exceeds maximum %d", length, MaxMessageSizeV2)
	}
	if length < 1 {
		return nil, fmt.Errorf("message length too small: %d", length)
	}

	// Read type byte
	typeBuf := make([]byte, 1)
	if _, err := io.ReadFull(r, typeBuf); err != nil {
		return nil, fmt.Errorf("failed to read message type: %w", err)
	}

	msg := &V2Message{
		Type: typeBuf[0],
	}

	// Read payload (length - 1 for type byte)
	payloadLength := length - 1
	if payloadLength > 0 {
		payloadBuf := make([]byte, payloadLength)
		if _, err := io.ReadFull(r, payloadBuf); err != nil {
			return nil, fmt.Errorf("failed to read message payload: %w", err)
		}
		msg.Payload = payloadBuf
	}

	return msg, nil
}

// WriteMessageV2 writes a v2 length-prefixed message to a writer.
// Format: [length:4][type:1][payload:variable]
func WriteMessageV2(w io.Writer, msg *V2Message) error {
	payload := msg.Payload
	if payload == nil {
		payload = []byte("{}")
	}

	// Calculate total length (type byte + payload)
	totalLength := 1 + len(payload)
	if totalLength > MaxMessageSizeV2 {
		return fmt.Errorf("message size %d exceeds maximum %d", totalLength, MaxMessageSizeV2)
	}

	// Write length prefix
	lengthBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuf, uint32(totalLength))
	if _, err := w.Write(lengthBuf); err != nil {
		return fmt.Errorf("failed to write message length: %w", err)
	}

	// Write type byte
	if _, err := w.Write([]byte{msg.Type}); err != nil {
		return fmt.Errorf("failed to write message type: %w", err)
	}

	// Write payload
	if _, err := w.Write(payload); err != nil {
		return fmt.Errorf("failed to write message payload: %w", err)
	}

	return nil
}

// NewV2Message creates a new v2 message with the given type and payload.
func NewV2Message(msgType byte, payload interface{}) (*V2Message, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}
	return &V2Message{
		Type:    msgType,
		Payload: payloadBytes,
	}, nil
}

// NewV2MessageWithRequestID creates a new v2 message with request ID.
func NewV2MessageWithRequestID(msgType byte, requestID uint64, payload interface{}) (*V2Message, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}
	return &V2Message{
		Type:      msgType,
		RequestID: requestID,
		Payload:   payloadBytes,
	}, nil
}

// ParsePayload unmarshals the message payload into the given destination.
func (m *V2Message) ParsePayload(dest interface{}) error {
	if m.Payload == nil {
		return nil
	}
	return json.Unmarshal(m.Payload, dest)
}

// TypeName returns a human-readable name for the message type.
func (m *V2Message) TypeName() string {
	switch m.Type {
	case MsgTypeHandshakeRequest:
		return "HandshakeRequest"
	case MsgTypeHandshakeResponse:
		return "HandshakeResponse"
	case MsgTypeRequestHeaders:
		return "RequestHeaders"
	case MsgTypeRequestBodyChunk:
		return "RequestBodyChunk"
	case MsgTypeResponseHeaders:
		return "ResponseHeaders"
	case MsgTypeResponseBodyChunk:
		return "ResponseBodyChunk"
	case MsgTypeDecision:
		return "Decision"
	case MsgTypeBodyMutation:
		return "BodyMutation"
	case MsgTypeCancelRequest:
		return "CancelRequest"
	case MsgTypeCancelAll:
		return "CancelAll"
	case MsgTypePing:
		return "Ping"
	case MsgTypePong:
		return "Pong"
	case MsgTypeHealthRequest:
		return "HealthRequest"
	case MsgTypeHealthResponse:
		return "HealthResponse"
	case MsgTypeMetricsRequest:
		return "MetricsRequest"
	case MsgTypeMetricsResponse:
		return "MetricsResponse"
	case MsgTypeRegistration:
		return "Registration"
	case MsgTypeRegistrationAck:
		return "RegistrationAck"
	default:
		return fmt.Sprintf("Unknown(0x%02X)", m.Type)
	}
}

// V2RequestHeaders represents request headers in v2 format.
type V2RequestHeaders struct {
	RequestID uint64              `json:"request_id"`
	Method    string              `json:"method"`
	URI       string              `json:"uri"`
	Headers   map[string][]string `json:"headers"`
	HasBody   bool                `json:"has_body"`
	Metadata  V2RequestMetadata   `json:"metadata"`
}

// V2RequestMetadata contains v2 request metadata.
type V2RequestMetadata struct {
	CorrelationID string  `json:"correlation_id"`
	ClientIP      string  `json:"client_ip"`
	ClientPort    int     `json:"client_port"`
	ServerName    *string `json:"server_name,omitempty"`
	Protocol      string  `json:"protocol"`
	TLSVersion    *string `json:"tls_version,omitempty"`
	RouteID       *string `json:"route_id,omitempty"`
	UpstreamID    *string `json:"upstream_id,omitempty"`
	Traceparent   *string `json:"traceparent,omitempty"`
}

// V2RequestBodyChunk represents a request body chunk in v2 format.
type V2RequestBodyChunk struct {
	RequestID  uint64 `json:"request_id"`
	ChunkIndex uint32 `json:"chunk_index"`
	Data       string `json:"data"` // Base64-encoded
	IsLast     bool   `json:"is_last"`
}

// V2ResponseHeaders represents response headers in v2 format.
type V2ResponseHeaders struct {
	RequestID  uint64              `json:"request_id"`
	StatusCode uint16              `json:"status_code"`
	Headers    map[string][]string `json:"headers"`
	HasBody    bool                `json:"has_body"`
}

// V2ResponseBodyChunk represents a response body chunk in v2 format.
type V2ResponseBodyChunk struct {
	RequestID  uint64 `json:"request_id"`
	ChunkIndex uint32 `json:"chunk_index"`
	Data       string `json:"data"` // Base64-encoded
	IsLast     bool   `json:"is_last"`
}

// V2Decision represents a decision in v2 format.
type V2Decision struct {
	RequestID       uint64                 `json:"request_id"`
	Decision        interface{}            `json:"decision"`
	RequestHeaders  []V2HeaderOp           `json:"request_headers,omitempty"`
	ResponseHeaders []V2HeaderOp           `json:"response_headers,omitempty"`
	Audit           map[string]interface{} `json:"audit,omitempty"`
}

// V2HeaderOp represents a header operation in v2 format.
type V2HeaderOp struct {
	Operation string  `json:"operation"` // "set", "add", "remove"
	Name      string  `json:"name"`
	Value     *string `json:"value,omitempty"`
}
