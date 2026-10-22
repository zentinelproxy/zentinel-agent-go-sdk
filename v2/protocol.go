package v2

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// MaxMessageSizeV2 is the maximum message size on the Unix socket transport (16 MB).
const MaxMessageSizeV2 = 16 * 1024 * 1024

// Message types of the Unix socket transport, as defined by the proxy
// (zentinel-agent-protocol, v2/uds.rs).
const (
	// Handshake.
	MsgTypeHandshakeRequest  byte = 0x01 // proxy -> agent
	MsgTypeHandshakeResponse byte = 0x02 // agent -> proxy

	// Events, proxy -> agent. Each is answered with MsgTypeAgentResponse.
	MsgTypeRequestHeaders    byte = 0x10
	MsgTypeRequestBodyChunk  byte = 0x11
	MsgTypeResponseHeaders   byte = 0x12
	MsgTypeResponseBodyChunk byte = 0x13
	MsgTypeRequestComplete   byte = 0x14
	MsgTypeWebSocketFrame    byte = 0x15
	MsgTypeGuardrailInspect  byte = 0x16
	MsgTypeConfigure         byte = 0x17

	// The agent's answer to an event.
	MsgTypeAgentResponse byte = 0x20

	// Control messages.
	MsgTypeHealthStatus        byte = 0x30
	MsgTypeMetricsReport       byte = 0x31
	MsgTypeConfigUpdateRequest byte = 0x32
	MsgTypeFlowControl         byte = 0x33

	// Management.
	MsgTypeCancel byte = 0x40
	MsgTypePing   byte = 0x41
	MsgTypePong   byte = 0x42
)

// V2Message is one frame of the Unix socket transport.
//
// On the wire a frame is a 4-byte big-endian length (counting the type byte
// and the payload), a 1-byte message type, and the payload: JSON.
type V2Message struct {
	// Type is the message type (MsgType*).
	Type byte

	// Payload is the JSON payload.
	Payload json.RawMessage
}

// ReadMessageV2 reads one frame. It returns (nil, nil) when the stream ends
// cleanly before a frame starts.
func ReadMessageV2(r io.Reader) (*V2Message, error) {
	var header [5]byte
	if _, err := io.ReadFull(r, header[:4]); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read message length: %w", err)
	}

	length := binary.BigEndian.Uint32(header[:4])
	if length < 1 {
		return nil, fmt.Errorf("zero-length message")
	}
	if length > MaxMessageSizeV2 {
		return nil, fmt.Errorf("message size %d exceeds maximum %d", length, MaxMessageSizeV2)
	}

	if _, err := io.ReadFull(r, header[4:5]); err != nil {
		return nil, fmt.Errorf("failed to read message type: %w", err)
	}

	msg := &V2Message{Type: header[4]}
	if length > 1 {
		payload := make([]byte, length-1)
		if _, err := io.ReadFull(r, payload); err != nil {
			return nil, fmt.Errorf("failed to read message payload: %w", err)
		}
		msg.Payload = payload
	}
	return msg, nil
}

// WriteMessageV2 writes one frame with a single Write call, so that writers
// sharing a connection only need to serialize calls to it.
func WriteMessageV2(w io.Writer, msg *V2Message) error {
	payload := msg.Payload
	if payload == nil {
		payload = []byte("{}")
	}
	if len(payload) > MaxMessageSizeV2 {
		return fmt.Errorf("message size %d exceeds maximum %d", len(payload), MaxMessageSizeV2)
	}

	frame := make([]byte, 5+len(payload))
	binary.BigEndian.PutUint32(frame[:4], uint32(1+len(payload)))
	frame[4] = msg.Type
	copy(frame[5:], payload)
	if _, err := w.Write(frame); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}
	return nil
}

// NewV2Message creates a frame with the given type and a JSON-encoded payload.
func NewV2Message(msgType byte, payload interface{}) (*V2Message, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}
	return &V2Message{Type: msgType, Payload: payloadBytes}, nil
}

// ParsePayload unmarshals the payload into dest.
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
	case MsgTypeRequestComplete:
		return "RequestComplete"
	case MsgTypeWebSocketFrame:
		return "WebSocketFrame"
	case MsgTypeGuardrailInspect:
		return "GuardrailInspect"
	case MsgTypeConfigure:
		return "Configure"
	case MsgTypeAgentResponse:
		return "AgentResponse"
	case MsgTypeHealthStatus:
		return "HealthStatus"
	case MsgTypeMetricsReport:
		return "MetricsReport"
	case MsgTypeConfigUpdateRequest:
		return "ConfigUpdateRequest"
	case MsgTypeFlowControl:
		return "FlowControl"
	case MsgTypeCancel:
		return "Cancel"
	case MsgTypePing:
		return "Ping"
	case MsgTypePong:
		return "Pong"
	default:
		return fmt.Sprintf("Unknown(0x%02X)", m.Type)
	}
}
