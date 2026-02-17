package zentinel

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
)

// Request is an ergonomic wrapper around HTTP request data.
type Request struct {
	event       *RequestHeadersEvent
	body        []byte
	parsedURL   *url.URL
	queryParams url.Values
}

// NewRequest creates a Request from a RequestHeadersEvent.
func NewRequest(event *RequestHeadersEvent, body []byte) *Request {
	parsedURL, _ := url.Parse(event.URI)
	return &Request{
		event:     event,
		body:      body,
		parsedURL: parsedURL,
	}
}

// Metadata returns the request metadata.
func (r *Request) Metadata() *RequestMetadata {
	return &r.event.Metadata
}

// CorrelationID returns the correlation ID for request tracing.
func (r *Request) CorrelationID() string {
	return r.event.Metadata.CorrelationID
}

// ClientIP returns the client IP address.
func (r *Request) ClientIP() string {
	return r.event.Metadata.ClientIP
}

// Method returns the HTTP method.
func (r *Request) Method() string {
	return r.event.Method
}

// IsGet checks if this is a GET request.
func (r *Request) IsGet() bool {
	return strings.EqualFold(r.event.Method, "GET")
}

// IsPost checks if this is a POST request.
func (r *Request) IsPost() bool {
	return strings.EqualFold(r.event.Method, "POST")
}

// IsPut checks if this is a PUT request.
func (r *Request) IsPut() bool {
	return strings.EqualFold(r.event.Method, "PUT")
}

// IsDelete checks if this is a DELETE request.
func (r *Request) IsDelete() bool {
	return strings.EqualFold(r.event.Method, "DELETE")
}

// IsPatch checks if this is a PATCH request.
func (r *Request) IsPatch() bool {
	return strings.EqualFold(r.event.Method, "PATCH")
}

// URI returns the full URI including query string.
func (r *Request) URI() string {
	return r.event.URI
}

// Path returns the full path including query string.
func (r *Request) Path() string {
	return r.event.URI
}

// PathOnly returns just the path without query string.
func (r *Request) PathOnly() string {
	if r.parsedURL != nil {
		return r.parsedURL.Path
	}
	return r.event.URI
}

// QueryString returns the raw query string.
func (r *Request) QueryString() string {
	if r.parsedURL != nil {
		return r.parsedURL.RawQuery
	}
	return ""
}

// getQueryParams parses and caches query parameters.
func (r *Request) getQueryParams() url.Values {
	if r.queryParams == nil {
		if r.parsedURL != nil {
			r.queryParams = r.parsedURL.Query()
		} else {
			r.queryParams = url.Values{}
		}
	}
	return r.queryParams
}

// Query returns a single query parameter value.
func (r *Request) Query(name string) string {
	return r.getQueryParams().Get(name)
}

// QueryAll returns all values for a query parameter.
func (r *Request) QueryAll(name string) []string {
	values := r.getQueryParams()[name]
	if values == nil {
		return []string{}
	}
	return values
}

// PathStartsWith checks if the path starts with the given prefix.
func (r *Request) PathStartsWith(prefix string) bool {
	return strings.HasPrefix(r.PathOnly(), prefix)
}

// PathEquals checks if the path exactly matches.
func (r *Request) PathEquals(path string) bool {
	return r.PathOnly() == path
}

// Headers returns all headers as a map.
func (r *Request) Headers() map[string][]string {
	return r.event.Headers
}

// Header returns a single header value (case-insensitive).
func (r *Request) Header(name string) string {
	nameLower := strings.ToLower(name)
	for key, values := range r.event.Headers {
		if strings.ToLower(key) == nameLower && len(values) > 0 {
			return values[0]
		}
	}
	return ""
}

// HeaderAll returns all values for a header (case-insensitive).
func (r *Request) HeaderAll(name string) []string {
	nameLower := strings.ToLower(name)
	for key, values := range r.event.Headers {
		if strings.ToLower(key) == nameLower {
			return values
		}
	}
	return []string{}
}

// HasHeader checks if a header exists (case-insensitive).
func (r *Request) HasHeader(name string) bool {
	nameLower := strings.ToLower(name)
	for key := range r.event.Headers {
		if strings.ToLower(key) == nameLower {
			return true
		}
	}
	return false
}

// Host returns the Host header value.
func (r *Request) Host() string {
	return r.Header("host")
}

// UserAgent returns the User-Agent header value.
func (r *Request) UserAgent() string {
	return r.Header("user-agent")
}

// ContentType returns the Content-Type header value.
func (r *Request) ContentType() string {
	return r.Header("content-type")
}

// Authorization returns the Authorization header value.
func (r *Request) Authorization() string {
	return r.Header("authorization")
}

// ContentLength returns the Content-Length header value as an integer.
// Returns -1 if the header is not present or invalid.
func (r *Request) ContentLength() int {
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
func (r *Request) IsJSON() bool {
	ct := r.ContentType()
	return strings.Contains(strings.ToLower(ct), "application/json")
}

// Body returns the raw body bytes.
func (r *Request) Body() []byte {
	return r.body
}

// BodyString returns the body as a UTF-8 string.
func (r *Request) BodyString() string {
	return string(r.body)
}

// BodyJSON parses the body as JSON into the given destination.
func (r *Request) BodyJSON(dest interface{}) error {
	return json.Unmarshal(r.body, dest)
}

// WithBody creates a new Request with the given body.
func (r *Request) WithBody(body []byte) *Request {
	return &Request{
		event:       r.event,
		body:        body,
		parsedURL:   r.parsedURL,
		queryParams: r.queryParams,
	}
}

// String returns a string representation of the request.
func (r *Request) String() string {
	return "Request(" + r.Method() + " " + r.Path() + ")"
}
