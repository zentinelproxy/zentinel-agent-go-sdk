package sentinel

import (
	"testing"
)

func makeTestResponse(status int, headers map[string][]string, body []byte) *Response {
	event := &ResponseHeadersEvent{
		CorrelationID: "test-123",
		Status:        status,
		Headers:       headers,
	}
	if headers == nil {
		event.Headers = map[string][]string{}
	}
	return NewResponse(event, body)
}

func TestResponse_StatusCode(t *testing.T) {
	response := makeTestResponse(200, nil, nil)

	if response.StatusCode() != 200 {
		t.Errorf("expected StatusCode() 200, got %d", response.StatusCode())
	}
}

func TestResponse_IsSuccess(t *testing.T) {
	tests := []struct {
		status   int
		expected bool
	}{
		{200, true},
		{201, true},
		{299, true},
		{199, false},
		{300, false},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			response := makeTestResponse(tt.status, nil, nil)
			if response.IsSuccess() != tt.expected {
				t.Errorf("IsSuccess() for status %d: expected %v, got %v", tt.status, tt.expected, response.IsSuccess())
			}
		})
	}
}

func TestResponse_IsRedirect(t *testing.T) {
	tests := []struct {
		status   int
		expected bool
	}{
		{300, true},
		{301, true},
		{302, true},
		{399, true},
		{299, false},
		{400, false},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			response := makeTestResponse(tt.status, nil, nil)
			if response.IsRedirect() != tt.expected {
				t.Errorf("IsRedirect() for status %d: expected %v, got %v", tt.status, tt.expected, response.IsRedirect())
			}
		})
	}
}

func TestResponse_IsClientError(t *testing.T) {
	tests := []struct {
		status   int
		expected bool
	}{
		{400, true},
		{404, true},
		{499, true},
		{399, false},
		{500, false},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			response := makeTestResponse(tt.status, nil, nil)
			if response.IsClientError() != tt.expected {
				t.Errorf("IsClientError() for status %d: expected %v, got %v", tt.status, tt.expected, response.IsClientError())
			}
		})
	}
}

func TestResponse_IsServerError(t *testing.T) {
	tests := []struct {
		status   int
		expected bool
	}{
		{500, true},
		{502, true},
		{599, true},
		{499, false},
		{600, false},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			response := makeTestResponse(tt.status, nil, nil)
			if response.IsServerError() != tt.expected {
				t.Errorf("IsServerError() for status %d: expected %v, got %v", tt.status, tt.expected, response.IsServerError())
			}
		})
	}
}

func TestResponse_IsError(t *testing.T) {
	tests := []struct {
		status   int
		expected bool
	}{
		{400, true},
		{404, true},
		{500, true},
		{502, true},
		{200, false},
		{302, false},
		{399, false},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			response := makeTestResponse(tt.status, nil, nil)
			if response.IsError() != tt.expected {
				t.Errorf("IsError() for status %d: expected %v, got %v", tt.status, tt.expected, response.IsError())
			}
		})
	}
}

func TestResponse_Headers(t *testing.T) {
	headers := map[string][]string{
		"Content-Type": {"text/html"},
		"Location":     {"https://example.com"},
	}
	response := makeTestResponse(302, headers, nil)

	if response.ContentType() != "text/html" {
		t.Errorf("expected ContentType() 'text/html', got %s", response.ContentType())
	}
	if response.Location() != "https://example.com" {
		t.Errorf("expected Location() 'https://example.com', got %s", response.Location())
	}
	if !response.IsHTML() {
		t.Error("expected IsHTML() to return true")
	}
	if response.IsJSON() {
		t.Error("expected IsJSON() to return false")
	}
}

func TestResponse_HeaderCaseInsensitive(t *testing.T) {
	headers := map[string][]string{
		"Content-Type": {"application/json"},
	}
	response := makeTestResponse(200, headers, nil)

	if response.Header("content-type") != "application/json" {
		t.Errorf("expected Header('content-type') 'application/json', got %s", response.Header("content-type"))
	}
	if response.Header("CONTENT-TYPE") != "application/json" {
		t.Errorf("expected Header('CONTENT-TYPE') 'application/json', got %s", response.Header("CONTENT-TYPE"))
	}
}

func TestResponse_HeaderAll(t *testing.T) {
	headers := map[string][]string{
		"Set-Cookie": {"cookie1=value1", "cookie2=value2"},
	}
	response := makeTestResponse(200, headers, nil)

	cookies := response.HeaderAll("Set-Cookie")
	if len(cookies) != 2 {
		t.Fatalf("expected 2 Set-Cookie headers, got %d", len(cookies))
	}
	if cookies[0] != "cookie1=value1" || cookies[1] != "cookie2=value2" {
		t.Errorf("unexpected Set-Cookie values: %v", cookies)
	}
}

func TestResponse_HasHeader(t *testing.T) {
	headers := map[string][]string{
		"Content-Type": {"application/json"},
	}
	response := makeTestResponse(200, headers, nil)

	if !response.HasHeader("Content-Type") {
		t.Error("expected HasHeader('Content-Type') to return true")
	}
	if !response.HasHeader("content-type") {
		t.Error("expected HasHeader('content-type') to return true (case insensitive)")
	}
	if response.HasHeader("Missing") {
		t.Error("expected HasHeader('Missing') to return false")
	}
}

func TestResponse_ContentLength(t *testing.T) {
	headers := map[string][]string{
		"Content-Length": {"1024"},
	}
	response := makeTestResponse(200, headers, nil)

	if response.ContentLength() != 1024 {
		t.Errorf("expected ContentLength() 1024, got %d", response.ContentLength())
	}
}

func TestResponse_ContentLengthInvalid(t *testing.T) {
	headers := map[string][]string{
		"Content-Length": {"not-a-number"},
	}
	response := makeTestResponse(200, headers, nil)

	if response.ContentLength() != -1 {
		t.Errorf("expected ContentLength() -1 for invalid value, got %d", response.ContentLength())
	}
}

func TestResponse_ContentLengthMissing(t *testing.T) {
	response := makeTestResponse(200, nil, nil)

	if response.ContentLength() != -1 {
		t.Errorf("expected ContentLength() -1 for missing header, got %d", response.ContentLength())
	}
}

func TestResponse_IsJSON(t *testing.T) {
	headers := map[string][]string{
		"Content-Type": {"application/json"},
	}
	response := makeTestResponse(200, headers, nil)

	if !response.IsJSON() {
		t.Error("expected IsJSON() to return true")
	}

	// Test with charset
	headers2 := map[string][]string{
		"Content-Type": {"application/json; charset=utf-8"},
	}
	response2 := makeTestResponse(200, headers2, nil)

	if !response2.IsJSON() {
		t.Error("expected IsJSON() to return true for 'application/json; charset=utf-8'")
	}
}

func TestResponse_IsHTML(t *testing.T) {
	headers := map[string][]string{
		"Content-Type": {"text/html"},
	}
	response := makeTestResponse(200, headers, nil)

	if !response.IsHTML() {
		t.Error("expected IsHTML() to return true")
	}

	// Test with charset
	headers2 := map[string][]string{
		"Content-Type": {"text/html; charset=utf-8"},
	}
	response2 := makeTestResponse(200, headers2, nil)

	if !response2.IsHTML() {
		t.Error("expected IsHTML() to return true for 'text/html; charset=utf-8'")
	}
}

func TestResponse_Body(t *testing.T) {
	body := []byte("Hello, World!")
	response := makeTestResponse(200, nil, body)

	if string(response.Body()) != "Hello, World!" {
		t.Errorf("expected Body() 'Hello, World!', got %s", string(response.Body()))
	}
	if response.BodyString() != "Hello, World!" {
		t.Errorf("expected BodyString() 'Hello, World!', got %s", response.BodyString())
	}
}

func TestResponse_BodyJSON(t *testing.T) {
	body := []byte(`{"message": "success"}`)
	response := makeTestResponse(200, nil, body)

	var data map[string]string
	if err := response.BodyJSON(&data); err != nil {
		t.Fatalf("failed to parse JSON body: %v", err)
	}
	if data["message"] != "success" {
		t.Errorf("expected message 'success', got %s", data["message"])
	}
}

func TestResponse_WithBody(t *testing.T) {
	response := makeTestResponse(200, nil, nil)
	newBody := []byte("new body")

	responseWithBody := response.WithBody(newBody)

	if string(responseWithBody.Body()) != "new body" {
		t.Errorf("expected new body 'new body', got %s", string(responseWithBody.Body()))
	}
	// Original should be unchanged
	if len(response.Body()) != 0 {
		t.Error("original response body should be empty")
	}
}

func TestResponse_String(t *testing.T) {
	response := makeTestResponse(404, nil, nil)

	str := response.String()
	if str != "Response(404)" {
		t.Errorf("expected String() 'Response(404)', got %s", str)
	}
}

func TestResponse_CorrelationID(t *testing.T) {
	response := makeTestResponse(200, nil, nil)

	if response.CorrelationID() != "test-123" {
		t.Errorf("expected CorrelationID() 'test-123', got %s", response.CorrelationID())
	}
}

func TestResponse_Headers_Empty(t *testing.T) {
	response := makeTestResponse(200, nil, nil)

	headers := response.Headers()
	if headers == nil {
		t.Error("expected Headers() to return non-nil map")
	}
	if len(headers) != 0 {
		t.Errorf("expected empty headers map, got %d entries", len(headers))
	}
}

func TestResponse_HeaderAll_Missing(t *testing.T) {
	response := makeTestResponse(200, nil, nil)

	headers := response.HeaderAll("Missing")
	if headers == nil {
		t.Error("expected HeaderAll() to return non-nil slice")
	}
	if len(headers) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(headers))
	}
}
