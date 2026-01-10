package sentinel

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"testing"
)

// TestProtocolVersion verifies the protocol version matches.
func TestProtocolVersion(t *testing.T) {
	if ProtocolVersion != 1 {
		t.Errorf("expected ProtocolVersion 1, got %d", ProtocolVersion)
	}
}

// TestMaxMessageSize verifies the maximum message size constant.
func TestMaxMessageSize(t *testing.T) {
	expected := 10 * 1024 * 1024 // 10MB
	if MaxMessageSize != expected {
		t.Errorf("expected MaxMessageSize %d, got %d", expected, MaxMessageSize)
	}
}

// TestEventTypes verifies event type constants.
func TestEventTypes(t *testing.T) {
	tests := []struct {
		eventType EventType
		expected  string
	}{
		{EventTypeRequestHeaders, "request_headers"},
		{EventTypeRequestBodyChunk, "request_body_chunk"},
		{EventTypeResponseHeaders, "response_headers"},
		{EventTypeResponseBodyChunk, "response_body_chunk"},
		{EventTypeRequestComplete, "request_complete"},
		{EventTypeWebSocketFrame, "websocket_frame"},
		{EventTypeConfigure, "configure"},
	}

	for _, tt := range tests {
		if string(tt.eventType) != tt.expected {
			t.Errorf("expected event type %s, got %s", tt.expected, tt.eventType)
		}
	}
}

// TestDecisionSerialization tests that Decision serialization matches Rust serde output.
func TestDecisionSerialization_Allow(t *testing.T) {
	response := Allow().Build()
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if parsed["decision"] != "allow" {
		t.Errorf("expected decision 'allow', got %v", parsed["decision"])
	}
}

func TestDecisionSerialization_Block(t *testing.T) {
	response := Block(403).Build()
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	decision := parsed["decision"].(map[string]interface{})
	block := decision["block"].(map[string]interface{})

	if block["status"].(float64) != 403 {
		t.Errorf("expected status 403, got %v", block["status"])
	}
}

func TestDecisionSerialization_BlockWithBody(t *testing.T) {
	response := Block(403).WithBody("Forbidden").Build()
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	decision := parsed["decision"].(map[string]interface{})
	block := decision["block"].(map[string]interface{})

	if block["status"].(float64) != 403 {
		t.Errorf("expected status 403, got %v", block["status"])
	}
	if block["body"] != "Forbidden" {
		t.Errorf("expected body 'Forbidden', got %v", block["body"])
	}
}

func TestDecisionSerialization_BlockWithHeaders(t *testing.T) {
	response := Block(403).
		WithBody("Forbidden").
		WithBlockHeader("X-Reason", "policy").
		Build()

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	decision := parsed["decision"].(map[string]interface{})
	block := decision["block"].(map[string]interface{})
	headers := block["headers"].(map[string]interface{})

	if headers["X-Reason"] != "policy" {
		t.Errorf("expected X-Reason 'policy', got %v", headers["X-Reason"])
	}
}

func TestDecisionSerialization_Redirect(t *testing.T) {
	response := Redirect("/login", 302).Build()
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	decision := parsed["decision"].(map[string]interface{})
	redirect := decision["redirect"].(map[string]interface{})

	if redirect["url"] != "/login" {
		t.Errorf("expected url '/login', got %v", redirect["url"])
	}
	if redirect["status"].(float64) != 302 {
		t.Errorf("expected status 302, got %v", redirect["status"])
	}
}

func TestDecisionSerialization_Challenge(t *testing.T) {
	response := Challenge("captcha", map[string]interface{}{"site_key": "abc123"}).Build()
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	decision := parsed["decision"].(map[string]interface{})
	challenge := decision["challenge"].(map[string]interface{})

	if challenge["challenge_type"] != "captcha" {
		t.Errorf("expected challenge_type 'captcha', got %v", challenge["challenge_type"])
	}
	params := challenge["params"].(map[string]interface{})
	if params["site_key"] != "abc123" {
		t.Errorf("expected site_key 'abc123', got %v", params["site_key"])
	}
}

// TestHeaderOpSerialization tests HeaderOp serialization matches Rust serde output.
func TestHeaderOpSerialization_Set(t *testing.T) {
	value := "value"
	op := HeaderOp{Operation: "set", Name: "X-Custom", Value: &value}
	data, err := json.Marshal(op)
	if err != nil {
		t.Fatalf("failed to marshal HeaderOp: %v", err)
	}

	expected := `{"set":{"name":"X-Custom","value":"value"}}`
	if string(data) != expected {
		t.Errorf("expected %s, got %s", expected, string(data))
	}
}

func TestHeaderOpSerialization_Remove(t *testing.T) {
	op := HeaderOp{Operation: "remove", Name: "X-Custom"}
	data, err := json.Marshal(op)
	if err != nil {
		t.Fatalf("failed to marshal HeaderOp: %v", err)
	}

	expected := `{"remove":{"name":"X-Custom"}}`
	if string(data) != expected {
		t.Errorf("expected %s, got %s", expected, string(data))
	}
}

// TestAgentResponseSerialization tests AgentResponse serialization.
func TestAgentResponseSerialization_FullStructure(t *testing.T) {
	response := Allow().Build()
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Required fields per Rust AgentResponse
	requiredFields := []string{
		"version",
		"decision",
		"request_headers",
		"response_headers",
		"routing_metadata",
		"audit",
		"needs_more",
	}

	for _, field := range requiredFields {
		if _, ok := parsed[field]; !ok {
			t.Errorf("missing required field: %s", field)
		}
	}
}

func TestAgentResponseSerialization_Version(t *testing.T) {
	response := Allow().Build()
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if parsed["version"].(float64) != float64(ProtocolVersion) {
		t.Errorf("expected version %d, got %v", ProtocolVersion, parsed["version"])
	}
}

func TestAgentResponseSerialization_WithHeaderOps(t *testing.T) {
	response := Allow().
		AddRequestHeader("X-Forwarded-By", "sentinel").
		RemoveRequestHeader("X-Internal").
		AddResponseHeader("X-Cache", "HIT").
		Build()

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	requestHeaders := parsed["request_headers"].([]interface{})
	if len(requestHeaders) != 2 {
		t.Fatalf("expected 2 request headers, got %d", len(requestHeaders))
	}

	// Check first header (set)
	h1 := requestHeaders[0].(map[string]interface{})
	setOp := h1["set"].(map[string]interface{})
	if setOp["name"] != "X-Forwarded-By" {
		t.Errorf("expected name 'X-Forwarded-By', got %v", setOp["name"])
	}
	if setOp["value"] != "sentinel" {
		t.Errorf("expected value 'sentinel', got %v", setOp["value"])
	}

	// Check second header (remove)
	h2 := requestHeaders[1].(map[string]interface{})
	removeOp := h2["remove"].(map[string]interface{})
	if removeOp["name"] != "X-Internal" {
		t.Errorf("expected name 'X-Internal', got %v", removeOp["name"])
	}
}

func TestAgentResponseSerialization_WithAuditMetadata(t *testing.T) {
	response := Deny().
		WithTag("security").
		WithTags("blocked", "waf").
		WithRuleID("RULE-001").
		WithConfidence(0.95).
		WithReasonCode("SQL_INJECTION").
		WithMetadata("matched_pattern", "SELECT.*FROM").
		Build()

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	audit := parsed["audit"].(map[string]interface{})

	tags := audit["tags"].([]interface{})
	expectedTags := []string{"security", "blocked", "waf"}
	for i, tag := range expectedTags {
		if tags[i] != tag {
			t.Errorf("expected tag %s at index %d, got %v", tag, i, tags[i])
		}
	}

	ruleIDs := audit["rule_ids"].([]interface{})
	if ruleIDs[0] != "RULE-001" {
		t.Errorf("expected rule_id 'RULE-001', got %v", ruleIDs[0])
	}

	if audit["confidence"].(float64) != 0.95 {
		t.Errorf("expected confidence 0.95, got %v", audit["confidence"])
	}

	reasonCodes := audit["reason_codes"].([]interface{})
	if reasonCodes[0] != "SQL_INJECTION" {
		t.Errorf("expected reason_code 'SQL_INJECTION', got %v", reasonCodes[0])
	}

	custom := audit["custom"].(map[string]interface{})
	if custom["matched_pattern"] != "SELECT.*FROM" {
		t.Errorf("expected matched_pattern 'SELECT.*FROM', got %v", custom["matched_pattern"])
	}
}

// TestEventDeserialization tests parsing events from Rust-generated JSON.
func TestEventDeserialization_RequestHeaders(t *testing.T) {
	rustJSON := `{
		"metadata": {
			"correlation_id": "req-123",
			"request_id": "internal-456",
			"client_ip": "192.168.1.1",
			"client_port": 54321,
			"server_name": "api.example.com",
			"protocol": "HTTP/2",
			"tls_version": "TLSv1.3",
			"tls_cipher": "TLS_AES_256_GCM_SHA384",
			"route_id": "api-route",
			"upstream_id": "backend-pool",
			"timestamp": "2024-01-15T10:30:00Z",
			"traceparent": "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
		},
		"method": "POST",
		"uri": "/api/users?include=profile",
		"headers": {
			"content-type": ["application/json"],
			"accept": ["application/json", "text/plain"],
			"x-request-id": ["abc123"]
		}
	}`

	var event RequestHeadersEvent
	if err := json.Unmarshal([]byte(rustJSON), &event); err != nil {
		t.Fatalf("failed to parse RequestHeadersEvent: %v", err)
	}

	if event.Metadata.CorrelationID != "req-123" {
		t.Errorf("expected correlation_id 'req-123', got %s", event.Metadata.CorrelationID)
	}
	if event.Metadata.ClientIP != "192.168.1.1" {
		t.Errorf("expected client_ip '192.168.1.1', got %s", event.Metadata.ClientIP)
	}
	if event.Metadata.ClientPort != 54321 {
		t.Errorf("expected client_port 54321, got %d", event.Metadata.ClientPort)
	}
	if event.Metadata.TLSVersion == nil || *event.Metadata.TLSVersion != "TLSv1.3" {
		t.Errorf("expected tls_version 'TLSv1.3', got %v", event.Metadata.TLSVersion)
	}
	if event.Method != "POST" {
		t.Errorf("expected method 'POST', got %s", event.Method)
	}
	if event.URI != "/api/users?include=profile" {
		t.Errorf("expected uri '/api/users?include=profile', got %s", event.URI)
	}
	if event.Headers["content-type"][0] != "application/json" {
		t.Errorf("expected content-type 'application/json', got %v", event.Headers["content-type"])
	}
	if len(event.Headers["accept"]) != 2 {
		t.Errorf("expected 2 accept values, got %d", len(event.Headers["accept"]))
	}
}

func TestEventDeserialization_RequestBodyChunk(t *testing.T) {
	bodyData := []byte(`{"name": "test"}`)
	encodedData := base64.StdEncoding.EncodeToString(bodyData)

	rustJSON := `{
		"correlation_id": "req-123",
		"data": "` + encodedData + `",
		"is_last": true,
		"total_size": 16,
		"chunk_index": 0,
		"bytes_received": 16
	}`

	var event RequestBodyChunkEvent
	if err := json.Unmarshal([]byte(rustJSON), &event); err != nil {
		t.Fatalf("failed to parse RequestBodyChunkEvent: %v", err)
	}

	if event.CorrelationID != "req-123" {
		t.Errorf("expected correlation_id 'req-123', got %s", event.CorrelationID)
	}

	decoded, err := event.DecodedData()
	if err != nil {
		t.Fatalf("failed to decode data: %v", err)
	}
	if string(decoded) != `{"name": "test"}` {
		t.Errorf("expected decoded data '{\"name\": \"test\"}', got %s", string(decoded))
	}

	if !event.IsLast {
		t.Error("expected is_last to be true")
	}
	if event.ChunkIndex != 0 {
		t.Errorf("expected chunk_index 0, got %d", event.ChunkIndex)
	}
}

func TestEventDeserialization_ResponseHeaders(t *testing.T) {
	rustJSON := `{
		"correlation_id": "req-123",
		"status": 200,
		"headers": {
			"content-type": ["application/json"],
			"cache-control": ["max-age=3600"]
		}
	}`

	var event ResponseHeadersEvent
	if err := json.Unmarshal([]byte(rustJSON), &event); err != nil {
		t.Fatalf("failed to parse ResponseHeadersEvent: %v", err)
	}

	if event.CorrelationID != "req-123" {
		t.Errorf("expected correlation_id 'req-123', got %s", event.CorrelationID)
	}
	if event.Status != 200 {
		t.Errorf("expected status 200, got %d", event.Status)
	}
	if event.Headers["content-type"][0] != "application/json" {
		t.Errorf("expected content-type 'application/json', got %v", event.Headers["content-type"])
	}
}

// TestWireFormatRoundTrip tests JSON round-trip compatibility.
func TestWireFormatRoundTrip(t *testing.T) {
	response := Block(403).
		WithBody("Access denied").
		WithTag("security").
		AddRequestHeader("X-Blocked", "true").
		Build()

	// Serialize to JSON (what we send to proxy)
	jsonBytes, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	// Parse back (simulating what Rust would receive)
	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Verify structure matches Rust expectations
	if parsed["version"].(float64) != 1 {
		t.Errorf("expected version 1, got %v", parsed["version"])
	}

	decision := parsed["decision"].(map[string]interface{})
	block := decision["block"].(map[string]interface{})
	if block["status"].(float64) != 403 {
		t.Errorf("expected status 403, got %v", block["status"])
	}
	if block["body"] != "Access denied" {
		t.Errorf("expected body 'Access denied', got %v", block["body"])
	}

	audit := parsed["audit"].(map[string]interface{})
	tags := audit["tags"].([]interface{})
	if tags[0] != "security" {
		t.Errorf("expected tag 'security', got %v", tags[0])
	}

	requestHeaders := parsed["request_headers"].([]interface{})
	h1 := requestHeaders[0].(map[string]interface{})
	setOp := h1["set"].(map[string]interface{})
	if setOp["name"] != "X-Blocked" {
		t.Errorf("expected header name 'X-Blocked', got %v", setOp["name"])
	}
}

// TestBodyMutationFormat tests body mutation format matches Rust BodyMutation.
func TestBodyMutationFormat(t *testing.T) {
	data := []byte("modified content")
	response := Allow().WithRequestBodyMutation(data, 0).Build()

	jsonBytes, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	mutation := parsed["request_body_mutation"].(map[string]interface{})
	if mutation["chunk_index"].(float64) != 0 {
		t.Errorf("expected chunk_index 0, got %v", mutation["chunk_index"])
	}

	expectedData := base64.StdEncoding.EncodeToString(data)
	if mutation["data"] != expectedData {
		t.Errorf("expected data '%s', got %v", expectedData, mutation["data"])
	}
}

func TestBodyMutationFormat_PassThrough(t *testing.T) {
	response := Allow().Build()

	jsonBytes, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// No mutation = pass through
	if parsed["request_body_mutation"] != nil {
		t.Errorf("expected request_body_mutation to be nil, got %v", parsed["request_body_mutation"])
	}
}

// TestReadWriteMessage tests length-prefixed message I/O.
func TestReadWriteMessage(t *testing.T) {
	message := map[string]interface{}{
		"version":    1,
		"event_type": "request_headers",
		"payload": map[string]interface{}{
			"method": "GET",
			"uri":    "/test",
		},
	}

	// Write message to buffer
	var buf bytes.Buffer
	if err := WriteMessage(&buf, message); err != nil {
		t.Fatalf("failed to write message: %v", err)
	}

	// Read message back
	reader := bytes.NewReader(buf.Bytes())
	read, err := ReadMessage(reader)
	if err != nil {
		t.Fatalf("failed to read message: %v", err)
	}

	if read["version"].(float64) != 1 {
		t.Errorf("expected version 1, got %v", read["version"])
	}
	if read["event_type"] != "request_headers" {
		t.Errorf("expected event_type 'request_headers', got %v", read["event_type"])
	}

	payload := read["payload"].(map[string]interface{})
	if payload["method"] != "GET" {
		t.Errorf("expected method 'GET', got %v", payload["method"])
	}
}

func TestReadMessage_EOF(t *testing.T) {
	// Empty reader
	reader := bytes.NewReader([]byte{})
	read, err := ReadMessage(reader)

	if err != nil {
		t.Errorf("expected nil error on EOF, got %v", err)
	}
	if read != nil {
		t.Errorf("expected nil message on EOF, got %v", read)
	}
}

func TestReadMessage_TooLarge(t *testing.T) {
	// Create a length prefix that exceeds max
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, uint32(MaxMessageSize+1))

	reader := bytes.NewReader(buf.Bytes())
	_, err := ReadMessage(reader)

	if err == nil {
		t.Error("expected error for message exceeding max size")
	}
}

func TestWriteMessage_TooLarge(t *testing.T) {
	// Create a message larger than max
	largeData := make([]byte, MaxMessageSize+1)
	message := map[string]interface{}{
		"data": string(largeData),
	}

	var buf bytes.Buffer
	err := WriteMessage(&buf, message)

	if err == nil {
		t.Error("expected error for message exceeding max size")
	}
}

// TestNewAllowResponse tests the default allow response constructor.
func TestNewAllowResponse(t *testing.T) {
	response := NewAllowResponse()

	if response.Version != ProtocolVersion {
		t.Errorf("expected version %d, got %d", ProtocolVersion, response.Version)
	}
	if response.Decision != "allow" {
		t.Errorf("expected decision 'allow', got %v", response.Decision)
	}
	if len(response.RequestHeaders) != 0 {
		t.Errorf("expected empty request headers, got %d", len(response.RequestHeaders))
	}
	if len(response.ResponseHeaders) != 0 {
		t.Errorf("expected empty response headers, got %d", len(response.ResponseHeaders))
	}
	if len(response.RoutingMetadata) != 0 {
		t.Errorf("expected empty routing metadata, got %d", len(response.RoutingMetadata))
	}
}

// TestResponseBodyChunkEvent tests response body chunk deserialization.
func TestEventDeserialization_ResponseBodyChunk(t *testing.T) {
	bodyData := []byte(`<html><body>Hello</body></html>`)
	encodedData := base64.StdEncoding.EncodeToString(bodyData)

	rustJSON := `{
		"correlation_id": "req-123",
		"data": "` + encodedData + `",
		"is_last": true,
		"total_size": 31,
		"chunk_index": 0,
		"bytes_received": 31
	}`

	var event ResponseBodyChunkEvent
	if err := json.Unmarshal([]byte(rustJSON), &event); err != nil {
		t.Fatalf("failed to parse ResponseBodyChunkEvent: %v", err)
	}

	if event.CorrelationID != "req-123" {
		t.Errorf("expected correlation_id 'req-123', got %s", event.CorrelationID)
	}

	decoded, err := event.DecodedData()
	if err != nil {
		t.Fatalf("failed to decode data: %v", err)
	}
	if string(decoded) != `<html><body>Hello</body></html>` {
		t.Errorf("unexpected decoded data: %s", string(decoded))
	}
}

// TestWebSocketFrameEvent tests websocket frame deserialization.
func TestEventDeserialization_WebSocketFrame(t *testing.T) {
	frameData := []byte("Hello WebSocket")
	encodedData := base64.StdEncoding.EncodeToString(frameData)

	rustJSON := `{
		"correlation_id": "ws-123",
		"opcode": 1,
		"data": "` + encodedData + `",
		"direction": "client_to_server",
		"frame_index": 0
	}`

	var event WebSocketFrameEvent
	if err := json.Unmarshal([]byte(rustJSON), &event); err != nil {
		t.Fatalf("failed to parse WebSocketFrameEvent: %v", err)
	}

	if event.CorrelationID != "ws-123" {
		t.Errorf("expected correlation_id 'ws-123', got %s", event.CorrelationID)
	}
	if event.Opcode != 1 {
		t.Errorf("expected opcode 1, got %d", event.Opcode)
	}
	if event.Direction != "client_to_server" {
		t.Errorf("expected direction 'client_to_server', got %s", event.Direction)
	}

	decoded, err := event.DecodedData()
	if err != nil {
		t.Fatalf("failed to decode data: %v", err)
	}
	if string(decoded) != "Hello WebSocket" {
		t.Errorf("unexpected decoded data: %s", string(decoded))
	}
}

// TestConfigureEvent tests configure event deserialization.
func TestEventDeserialization_Configure(t *testing.T) {
	rustJSON := `{
		"agent_id": "rate-limiter-1",
		"config": {
			"enabled": true,
			"rate_limit": 100,
			"blocked_paths": ["/admin", "/internal"]
		}
	}`

	var event ConfigureEvent
	if err := json.Unmarshal([]byte(rustJSON), &event); err != nil {
		t.Fatalf("failed to parse ConfigureEvent: %v", err)
	}

	if event.AgentID != "rate-limiter-1" {
		t.Errorf("expected agent_id 'rate-limiter-1', got %s", event.AgentID)
	}
	if event.Config["enabled"] != true {
		t.Errorf("expected enabled true, got %v", event.Config["enabled"])
	}
	if event.Config["rate_limit"].(float64) != 100 {
		t.Errorf("expected rate_limit 100, got %v", event.Config["rate_limit"])
	}
}

// TestRequestCompleteEvent tests request complete event deserialization.
func TestEventDeserialization_RequestComplete(t *testing.T) {
	rustJSON := `{
		"correlation_id": "req-123",
		"status": 200,
		"duration_ms": 150,
		"request_size": 1024,
		"response_size": 2048
	}`

	var event RequestCompleteEvent
	if err := json.Unmarshal([]byte(rustJSON), &event); err != nil {
		t.Fatalf("failed to parse RequestCompleteEvent: %v", err)
	}

	if event.CorrelationID != "req-123" {
		t.Errorf("expected correlation_id 'req-123', got %s", event.CorrelationID)
	}
	if event.Status != 200 {
		t.Errorf("expected status 200, got %d", event.Status)
	}
	if event.DurationMS != 150 {
		t.Errorf("expected duration_ms 150, got %d", event.DurationMS)
	}
	if event.RequestSize != 1024 {
		t.Errorf("expected request_size 1024, got %d", event.RequestSize)
	}
	if event.ResponseSize != 2048 {
		t.Errorf("expected response_size 2048, got %d", event.ResponseSize)
	}
}
