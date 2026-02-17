package zentinel

import (
	"encoding/json"
	"strconv"
	"strings"
)

// Response is an ergonomic wrapper around HTTP response data.
type Response struct {
	event *ResponseHeadersEvent
	body  []byte
}

// NewResponse creates a Response from a ResponseHeadersEvent.
func NewResponse(event *ResponseHeadersEvent, body []byte) *Response {
	return &Response{
		event: event,
		body:  body,
	}
}

// CorrelationID returns the correlation ID for request tracing.
func (r *Response) CorrelationID() string {
	return r.event.CorrelationID
}

// StatusCode returns the HTTP status code.
func (r *Response) StatusCode() int {
	return r.event.Status
}

// IsSuccess checks if the status code indicates success (2xx).
func (r *Response) IsSuccess() bool {
	return r.event.Status >= 200 && r.event.Status < 300
}

// IsRedirect checks if the status code indicates redirect (3xx).
func (r *Response) IsRedirect() bool {
	return r.event.Status >= 300 && r.event.Status < 400
}

// IsClientError checks if the status code indicates client error (4xx).
func (r *Response) IsClientError() bool {
	return r.event.Status >= 400 && r.event.Status < 500
}

// IsServerError checks if the status code indicates server error (5xx).
func (r *Response) IsServerError() bool {
	return r.event.Status >= 500 && r.event.Status < 600
}

// IsError checks if the status code indicates any error (4xx or 5xx).
func (r *Response) IsError() bool {
	return r.event.Status >= 400
}

// Headers returns all headers as a map.
func (r *Response) Headers() map[string][]string {
	return r.event.Headers
}

// Header returns a single header value (case-insensitive).
func (r *Response) Header(name string) string {
	nameLower := strings.ToLower(name)
	for key, values := range r.event.Headers {
		if strings.ToLower(key) == nameLower && len(values) > 0 {
			return values[0]
		}
	}
	return ""
}

// HeaderAll returns all values for a header (case-insensitive).
func (r *Response) HeaderAll(name string) []string {
	nameLower := strings.ToLower(name)
	for key, values := range r.event.Headers {
		if strings.ToLower(key) == nameLower {
			return values
		}
	}
	return []string{}
}

// HasHeader checks if a header exists (case-insensitive).
func (r *Response) HasHeader(name string) bool {
	nameLower := strings.ToLower(name)
	for key := range r.event.Headers {
		if strings.ToLower(key) == nameLower {
			return true
		}
	}
	return false
}

// ContentType returns the Content-Type header value.
func (r *Response) ContentType() string {
	return r.Header("content-type")
}

// Location returns the Location header value (for redirects).
func (r *Response) Location() string {
	return r.Header("location")
}

// ContentLength returns the Content-Length header value as an integer.
// Returns -1 if the header is not present or invalid.
func (r *Response) ContentLength() int {
	value := r.Header("content-length")
	if value == "" {
		return -1
	}
	length, err := strconv.Atoi(value)
	if err != nil {
		return -1
	}
	return length
}

// IsJSON checks if the content type indicates JSON.
func (r *Response) IsJSON() bool {
	ct := r.ContentType()
	return strings.Contains(strings.ToLower(ct), "application/json")
}

// IsHTML checks if the content type indicates HTML.
func (r *Response) IsHTML() bool {
	ct := r.ContentType()
	return strings.Contains(strings.ToLower(ct), "text/html")
}

// Body returns the raw body bytes.
func (r *Response) Body() []byte {
	return r.body
}

// BodyString returns the body as a UTF-8 string.
func (r *Response) BodyString() string {
	return string(r.body)
}

// BodyJSON parses the body as JSON into the given destination.
func (r *Response) BodyJSON(dest interface{}) error {
	return json.Unmarshal(r.body, dest)
}

// WithBody creates a new Response with the given body.
func (r *Response) WithBody(body []byte) *Response {
	return &Response{
		event: r.event,
		body:  body,
	}
}

// String returns a string representation of the response.
func (r *Response) String() string {
	return "Response(" + strconv.Itoa(r.StatusCode()) + ")"
}
