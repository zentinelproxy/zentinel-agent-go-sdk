package zentinel

import (
	"testing"
)

func makeTestRequest(method, uri string, headers map[string][]string, body []byte) *Request {
	event := &RequestHeadersEvent{
		Metadata: RequestMetadata{
			CorrelationID: "test-123",
			RequestID:     "req-456",
			ClientIP:      "127.0.0.1",
			ClientPort:    12345,
			Protocol:      "HTTP/1.1",
		},
		Method:  method,
		URI:     uri,
		Headers: headers,
	}
	if headers == nil {
		event.Headers = map[string][]string{}
	}
	return NewRequest(event, body)
}

func TestRequest_Method(t *testing.T) {
	request := makeTestRequest("POST", "/test", nil, nil)

	if request.Method() != "POST" {
		t.Errorf("expected method 'POST', got %s", request.Method())
	}
	if !request.IsPost() {
		t.Error("expected IsPost() to return true")
	}
	if request.IsGet() {
		t.Error("expected IsGet() to return false")
	}
}

func TestRequest_MethodChecks(t *testing.T) {
	tests := []struct {
		method string
		check  func(*Request) bool
	}{
		{"GET", (*Request).IsGet},
		{"POST", (*Request).IsPost},
		{"PUT", (*Request).IsPut},
		{"DELETE", (*Request).IsDelete},
		{"PATCH", (*Request).IsPatch},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			request := makeTestRequest(tt.method, "/test", nil, nil)
			if !tt.check(request) {
				t.Errorf("expected Is%s() to return true for method %s", tt.method, tt.method)
			}
		})
	}
}

func TestRequest_Path(t *testing.T) {
	request := makeTestRequest("GET", "/api/users?page=1", nil, nil)

	if request.Path() != "/api/users?page=1" {
		t.Errorf("expected path '/api/users?page=1', got %s", request.Path())
	}
	if request.PathOnly() != "/api/users" {
		t.Errorf("expected path_only '/api/users', got %s", request.PathOnly())
	}
	if request.QueryString() != "page=1" {
		t.Errorf("expected query_string 'page=1', got %s", request.QueryString())
	}
}

func TestRequest_PathStartsWith(t *testing.T) {
	request := makeTestRequest("GET", "/api/users", nil, nil)

	if !request.PathStartsWith("/api") {
		t.Error("expected PathStartsWith('/api') to return true")
	}
	if request.PathStartsWith("/admin") {
		t.Error("expected PathStartsWith('/admin') to return false")
	}
}

func TestRequest_PathEquals(t *testing.T) {
	request := makeTestRequest("GET", "/health", nil, nil)

	if !request.PathEquals("/health") {
		t.Error("expected PathEquals('/health') to return true")
	}
	if request.PathEquals("/healthz") {
		t.Error("expected PathEquals('/healthz') to return false")
	}
}

func TestRequest_QueryParams(t *testing.T) {
	request := makeTestRequest("GET", "/search?q=test&tag=a&tag=b", nil, nil)

	if request.Query("q") != "test" {
		t.Errorf("expected query('q') 'test', got %s", request.Query("q"))
	}

	tags := request.QueryAll("tag")
	if len(tags) != 2 || tags[0] != "a" || tags[1] != "b" {
		t.Errorf("expected query_all('tag') ['a', 'b'], got %v", tags)
	}

	if request.Query("missing") != "" {
		t.Errorf("expected query('missing') to be empty, got %s", request.Query("missing"))
	}
}

func TestRequest_Headers(t *testing.T) {
	headers := map[string][]string{
		"Content-Type": {"application/json"},
		"X-Custom":     {"value1", "value2"},
	}
	request := makeTestRequest("GET", "/test", headers, nil)

	// Case-insensitive header access
	if request.Header("content-type") != "application/json" {
		t.Errorf("expected header('content-type') 'application/json', got %s", request.Header("content-type"))
	}
	if request.Header("Content-Type") != "application/json" {
		t.Errorf("expected header('Content-Type') 'application/json', got %s", request.Header("Content-Type"))
	}

	// Multiple values
	customs := request.HeaderAll("X-Custom")
	if len(customs) != 2 || customs[0] != "value1" || customs[1] != "value2" {
		t.Errorf("expected header_all('X-Custom') ['value1', 'value2'], got %v", customs)
	}

	// HasHeader
	if !request.HasHeader("Content-Type") {
		t.Error("expected HasHeader('Content-Type') to return true")
	}
	if request.HasHeader("Missing") {
		t.Error("expected HasHeader('Missing') to return false")
	}
}

func TestRequest_CommonHeaders(t *testing.T) {
	headers := map[string][]string{
		"Host":           {"example.com"},
		"User-Agent":     {"TestAgent/1.0"},
		"Content-Type":   {"application/json"},
		"Authorization":  {"Bearer token"},
		"Content-Length": {"100"},
	}
	request := makeTestRequest("GET", "/test", headers, nil)

	if request.Host() != "example.com" {
		t.Errorf("expected Host() 'example.com', got %s", request.Host())
	}
	if request.UserAgent() != "TestAgent/1.0" {
		t.Errorf("expected UserAgent() 'TestAgent/1.0', got %s", request.UserAgent())
	}
	if request.ContentType() != "application/json" {
		t.Errorf("expected ContentType() 'application/json', got %s", request.ContentType())
	}
	if request.Authorization() != "Bearer token" {
		t.Errorf("expected Authorization() 'Bearer token', got %s", request.Authorization())
	}
	if request.ContentLength() != 100 {
		t.Errorf("expected ContentLength() 100, got %d", request.ContentLength())
	}
}

func TestRequest_ContentLengthInvalid(t *testing.T) {
	headers := map[string][]string{
		"Content-Length": {"not-a-number"},
	}
	request := makeTestRequest("GET", "/test", headers, nil)

	if request.ContentLength() != -1 {
		t.Errorf("expected ContentLength() -1 for invalid value, got %d", request.ContentLength())
	}
}

func TestRequest_ContentLengthMissing(t *testing.T) {
	request := makeTestRequest("GET", "/test", nil, nil)

	if request.ContentLength() != -1 {
		t.Errorf("expected ContentLength() -1 for missing header, got %d", request.ContentLength())
	}
}

func TestRequest_Body(t *testing.T) {
	body := []byte(`{"key": "value"}`)
	request := makeTestRequest("POST", "/test", nil, body)

	if string(request.Body()) != `{"key": "value"}` {
		t.Errorf("expected Body() '{\"key\": \"value\"}', got %s", string(request.Body()))
	}
	if request.BodyString() != `{"key": "value"}` {
		t.Errorf("expected BodyString() '{\"key\": \"value\"}', got %s", request.BodyString())
	}

	var data map[string]string
	if err := request.BodyJSON(&data); err != nil {
		t.Fatalf("failed to parse JSON body: %v", err)
	}
	if data["key"] != "value" {
		t.Errorf("expected body JSON key 'value', got %s", data["key"])
	}
}

func TestRequest_IsJSON(t *testing.T) {
	headers := map[string][]string{
		"Content-Type": {"application/json"},
	}
	request := makeTestRequest("POST", "/test", headers, nil)

	if !request.IsJSON() {
		t.Error("expected IsJSON() to return true")
	}

	request2 := makeTestRequest("GET", "/test", nil, nil)
	if request2.IsJSON() {
		t.Error("expected IsJSON() to return false when Content-Type is not set")
	}
}

func TestRequest_Metadata(t *testing.T) {
	request := makeTestRequest("GET", "/test", nil, nil)

	if request.CorrelationID() != "test-123" {
		t.Errorf("expected CorrelationID() 'test-123', got %s", request.CorrelationID())
	}
	if request.ClientIP() != "127.0.0.1" {
		t.Errorf("expected ClientIP() '127.0.0.1', got %s", request.ClientIP())
	}
}

func TestRequest_WithBody(t *testing.T) {
	request := makeTestRequest("POST", "/test", nil, nil)
	newBody := []byte("new body")

	requestWithBody := request.WithBody(newBody)

	if string(requestWithBody.Body()) != "new body" {
		t.Errorf("expected new body 'new body', got %s", string(requestWithBody.Body()))
	}
	// Original should be unchanged
	if len(request.Body()) != 0 {
		t.Error("original request body should be empty")
	}
}

func TestRequest_String(t *testing.T) {
	request := makeTestRequest("GET", "/api/users", nil, nil)

	str := request.String()
	if str != "Request(GET /api/users)" {
		t.Errorf("expected String() 'Request(GET /api/users)', got %s", str)
	}
}

func TestRequest_URI(t *testing.T) {
	request := makeTestRequest("GET", "/api/users?page=1", nil, nil)

	if request.URI() != "/api/users?page=1" {
		t.Errorf("expected URI() '/api/users?page=1', got %s", request.URI())
	}
}

func TestRequest_Headers_Empty(t *testing.T) {
	request := makeTestRequest("GET", "/test", nil, nil)

	headers := request.Headers()
	if headers == nil {
		t.Error("expected Headers() to return non-nil map")
	}
	if len(headers) != 0 {
		t.Errorf("expected empty headers map, got %d entries", len(headers))
	}
}

func TestRequest_Metadata_Full(t *testing.T) {
	request := makeTestRequest("GET", "/test", nil, nil)

	metadata := request.Metadata()
	if metadata.CorrelationID != "test-123" {
		t.Errorf("expected Metadata().CorrelationID 'test-123', got %s", metadata.CorrelationID)
	}
	if metadata.RequestID != "req-456" {
		t.Errorf("expected Metadata().RequestID 'req-456', got %s", metadata.RequestID)
	}
	if metadata.ClientIP != "127.0.0.1" {
		t.Errorf("expected Metadata().ClientIP '127.0.0.1', got %s", metadata.ClientIP)
	}
	if metadata.ClientPort != 12345 {
		t.Errorf("expected Metadata().ClientPort 12345, got %d", metadata.ClientPort)
	}
}
