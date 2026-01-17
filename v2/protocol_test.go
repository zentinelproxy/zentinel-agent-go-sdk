package v2

import (
	"bytes"
	"testing"
)

func TestV2Message_TypeName(t *testing.T) {
	tests := []struct {
		msgType byte
		name    string
	}{
		{MsgTypeHandshakeRequest, "HandshakeRequest"},
		{MsgTypeHandshakeResponse, "HandshakeResponse"},
		{MsgTypeRequestHeaders, "RequestHeaders"},
		{MsgTypeRequestBodyChunk, "RequestBodyChunk"},
		{MsgTypeResponseHeaders, "ResponseHeaders"},
		{MsgTypeResponseBodyChunk, "ResponseBodyChunk"},
		{MsgTypeDecision, "Decision"},
		{MsgTypeBodyMutation, "BodyMutation"},
		{MsgTypeCancelRequest, "CancelRequest"},
		{MsgTypeCancelAll, "CancelAll"},
		{MsgTypePing, "Ping"},
		{MsgTypePong, "Pong"},
		{MsgTypeHealthRequest, "HealthRequest"},
		{MsgTypeHealthResponse, "HealthResponse"},
		{MsgTypeMetricsRequest, "MetricsRequest"},
		{MsgTypeMetricsResponse, "MetricsResponse"},
		{MsgTypeRegistration, "Registration"},
		{MsgTypeRegistrationAck, "RegistrationAck"},
		{0xFF, "Unknown(0xFF)"},
	}

	for _, tt := range tests {
		msg := &V2Message{Type: tt.msgType}
		if got := msg.TypeName(); got != tt.name {
			t.Errorf("TypeName() for 0x%02X = %s, want %s", tt.msgType, got, tt.name)
		}
	}
}

func TestReadWriteMessageV2(t *testing.T) {
	payload := map[string]interface{}{
		"request_id": float64(123), // JSON numbers are float64
		"method":     "GET",
		"uri":        "/api/users",
	}

	msg, err := NewV2Message(MsgTypeRequestHeaders, payload)
	if err != nil {
		t.Fatalf("failed to create message: %v", err)
	}

	// Write to buffer
	var buf bytes.Buffer
	if err := WriteMessageV2(&buf, msg); err != nil {
		t.Fatalf("failed to write message: %v", err)
	}

	// Read back
	readMsg, err := ReadMessageV2(&buf)
	if err != nil {
		t.Fatalf("failed to read message: %v", err)
	}

	if readMsg.Type != MsgTypeRequestHeaders {
		t.Errorf("expected type %d, got %d", MsgTypeRequestHeaders, readMsg.Type)
	}

	// Parse payload
	var parsed map[string]interface{}
	if err := readMsg.ParsePayload(&parsed); err != nil {
		t.Fatalf("failed to parse payload: %v", err)
	}

	if parsed["method"] != "GET" {
		t.Errorf("expected method 'GET', got %v", parsed["method"])
	}
	if parsed["uri"] != "/api/users" {
		t.Errorf("expected uri '/api/users', got %v", parsed["uri"])
	}
}

func TestReadMessageV2_EmptyPayload(t *testing.T) {
	msg := &V2Message{
		Type:    MsgTypePing,
		Payload: nil,
	}

	var buf bytes.Buffer
	if err := WriteMessageV2(&buf, msg); err != nil {
		t.Fatalf("failed to write message: %v", err)
	}

	readMsg, err := ReadMessageV2(&buf)
	if err != nil {
		t.Fatalf("failed to read message: %v", err)
	}

	if readMsg.Type != MsgTypePing {
		t.Errorf("expected type %d, got %d", MsgTypePing, readMsg.Type)
	}
}

func TestReadMessageV2_EOF(t *testing.T) {
	var buf bytes.Buffer
	msg, err := ReadMessageV2(&buf)

	if err != nil {
		t.Errorf("expected nil error for EOF, got %v", err)
	}
	if msg != nil {
		t.Errorf("expected nil message for EOF, got %v", msg)
	}
}

func TestReadMessageV2_MessageTooLarge(t *testing.T) {
	// Create a message claiming to be larger than max
	var buf bytes.Buffer
	// Write length (MaxMessageSizeV2 + 1)
	length := uint32(MaxMessageSizeV2 + 1)
	buf.WriteByte(byte(length >> 24))
	buf.WriteByte(byte(length >> 16))
	buf.WriteByte(byte(length >> 8))
	buf.WriteByte(byte(length))

	_, err := ReadMessageV2(&buf)
	if err == nil {
		t.Error("expected error for oversized message")
	}
}

func TestNewV2MessageWithRequestID(t *testing.T) {
	payload := map[string]string{"key": "value"}
	msg, err := NewV2MessageWithRequestID(MsgTypeDecision, 42, payload)
	if err != nil {
		t.Fatalf("failed to create message: %v", err)
	}

	if msg.RequestID != 42 {
		t.Errorf("expected request ID 42, got %d", msg.RequestID)
	}
	if msg.Type != MsgTypeDecision {
		t.Errorf("expected type %d, got %d", MsgTypeDecision, msg.Type)
	}
}

func TestV2RequestHeaders(t *testing.T) {
	headers := V2RequestHeaders{
		RequestID: 1,
		Method:    "POST",
		URI:       "/api/data",
		Headers: map[string][]string{
			"Content-Type": {"application/json"},
		},
		HasBody: true,
		Metadata: V2RequestMetadata{
			CorrelationID: "corr-123",
			ClientIP:      "192.168.1.1",
			ClientPort:    54321,
			Protocol:      "HTTP/1.1",
		},
	}

	if headers.RequestID != 1 {
		t.Errorf("expected RequestID 1, got %d", headers.RequestID)
	}
	if headers.Method != "POST" {
		t.Errorf("expected Method 'POST', got %s", headers.Method)
	}
	if !headers.HasBody {
		t.Error("expected HasBody to be true")
	}
	if headers.Metadata.CorrelationID != "corr-123" {
		t.Errorf("expected CorrelationID 'corr-123', got %s", headers.Metadata.CorrelationID)
	}
}

func TestV2Decision(t *testing.T) {
	value := "header-value"
	decision := V2Decision{
		RequestID: 1,
		Decision:  "allow",
		RequestHeaders: []V2HeaderOp{
			{Operation: "set", Name: "X-Custom", Value: &value},
		},
		ResponseHeaders: []V2HeaderOp{
			{Operation: "remove", Name: "X-Internal", Value: nil},
		},
		Audit: map[string]interface{}{
			"tags": []string{"security"},
		},
	}

	if decision.RequestID != 1 {
		t.Errorf("expected RequestID 1, got %d", decision.RequestID)
	}
	if decision.Decision != "allow" {
		t.Errorf("expected Decision 'allow', got %v", decision.Decision)
	}
	if len(decision.RequestHeaders) != 1 {
		t.Errorf("expected 1 request header op, got %d", len(decision.RequestHeaders))
	}
	if decision.RequestHeaders[0].Operation != "set" {
		t.Errorf("expected operation 'set', got %s", decision.RequestHeaders[0].Operation)
	}
}

func TestCancelRequestMessage(t *testing.T) {
	reason := "client disconnected"
	cancel := CancelRequestMessage{
		RequestID: 123,
		Reason:    &reason,
	}

	if cancel.RequestID != 123 {
		t.Errorf("expected RequestID 123, got %d", cancel.RequestID)
	}
	if cancel.Reason == nil || *cancel.Reason != "client disconnected" {
		t.Errorf("expected reason 'client disconnected', got %v", cancel.Reason)
	}
}

func TestPingPongMessages(t *testing.T) {
	ping := PingMessage{Timestamp: 1234567890}
	pong := PongMessage{Timestamp: 1234567890}

	if ping.Timestamp != pong.Timestamp {
		t.Error("expected ping and pong timestamps to match")
	}
}
