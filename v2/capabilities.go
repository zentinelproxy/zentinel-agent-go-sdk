package v2

import (
	zentinel "github.com/zentinelproxy/zentinel-agent-go-sdk"
	pb "github.com/zentinelproxy/zentinel-agent-go-sdk/v2/proto"
)

// AgentCapabilities describes what processing capabilities an agent supports.
// Use NewAgentCapabilities() and the builder methods to construct capabilities.
//
// Example:
//
//	caps := v2.NewAgentCapabilities().
//	    HandleRequestHeaders().
//	    HandleRequestBody().
//	    HandleResponseHeaders().
//	    WithMaxConcurrentRequests(100)
type AgentCapabilities struct {
	// HandlesRequestHeaders indicates the agent processes request headers.
	HandlesRequestHeaders bool `json:"handles_request_headers"`

	// HandlesRequestBody indicates the agent processes request body chunks.
	HandlesRequestBody bool `json:"handles_request_body"`

	// HandlesResponseHeaders indicates the agent processes response headers.
	HandlesResponseHeaders bool `json:"handles_response_headers"`

	// HandlesResponseBody indicates the agent processes response body chunks.
	HandlesResponseBody bool `json:"handles_response_body"`

	// SupportsStreaming indicates the agent supports streaming body processing.
	SupportsStreaming bool `json:"supports_streaming"`

	// SupportsCancellation indicates the agent handles request cancellation.
	SupportsCancellation bool `json:"supports_cancellation"`

	// MaxConcurrentRequests limits concurrent in-flight requests.
	// nil means no limit.
	MaxConcurrentRequests *uint32 `json:"max_concurrent_requests,omitempty"`

	// SupportedFeatures lists additional features the agent supports.
	SupportedFeatures []string `json:"supported_features,omitempty"`

	// AgentID identifies the agent in the handshake. Empty means the
	// agent's Name().
	AgentID string `json:"agent_id,omitempty"`

	// Version is the agent's version, reported in the handshake. Empty
	// means the SDK version.
	Version string `json:"version,omitempty"`

	// MaxBodySize is the largest body the agent wants to see (default 10 MB).
	MaxBodySize uint64 `json:"max_body_size,omitempty"`
}

// NewAgentCapabilities creates a new AgentCapabilities with default values.
// By default, only request header handling is enabled.
func NewAgentCapabilities() *AgentCapabilities {
	return &AgentCapabilities{
		HandlesRequestHeaders:  true,
		HandlesRequestBody:     false,
		HandlesResponseHeaders: false,
		HandlesResponseBody:    false,
		SupportsStreaming:      false,
		SupportsCancellation:   true,
		MaxConcurrentRequests:  nil,
		SupportedFeatures:      []string{},
	}
}

// WithAgentID sets the agent ID reported in the handshake.
func (c *AgentCapabilities) WithAgentID(id string) *AgentCapabilities {
	c.AgentID = id
	return c
}

// WithVersion sets the agent version reported in the handshake.
func (c *AgentCapabilities) WithVersion(version string) *AgentCapabilities {
	c.Version = version
	return c
}

// HandleRequestHeaders enables request header processing.
func (c *AgentCapabilities) HandleRequestHeaders() *AgentCapabilities {
	c.HandlesRequestHeaders = true
	return c
}

// HandleRequestBody enables request body processing.
func (c *AgentCapabilities) HandleRequestBody() *AgentCapabilities {
	c.HandlesRequestBody = true
	return c
}

// HandleResponseHeaders enables response header processing.
func (c *AgentCapabilities) HandleResponseHeaders() *AgentCapabilities {
	c.HandlesResponseHeaders = true
	return c
}

// HandleResponseBody enables response body processing.
func (c *AgentCapabilities) HandleResponseBody() *AgentCapabilities {
	c.HandlesResponseBody = true
	return c
}

// WithStreaming enables streaming body processing.
func (c *AgentCapabilities) WithStreaming() *AgentCapabilities {
	c.SupportsStreaming = true
	return c
}

// WithCancellation enables request cancellation support.
func (c *AgentCapabilities) WithCancellation() *AgentCapabilities {
	c.SupportsCancellation = true
	return c
}

// WithoutCancellation disables request cancellation support.
func (c *AgentCapabilities) WithoutCancellation() *AgentCapabilities {
	c.SupportsCancellation = false
	return c
}

// WithMaxConcurrentRequests sets the maximum concurrent requests limit.
func (c *AgentCapabilities) WithMaxConcurrentRequests(max uint32) *AgentCapabilities {
	c.MaxConcurrentRequests = &max
	return c
}

// WithFeature adds a supported feature.
func (c *AgentCapabilities) WithFeature(feature string) *AgentCapabilities {
	c.SupportedFeatures = append(c.SupportedFeatures, feature)
	return c
}

// WithFeatures adds multiple supported features.
func (c *AgentCapabilities) WithFeatures(features ...string) *AgentCapabilities {
	c.SupportedFeatures = append(c.SupportedFeatures, features...)
	return c
}

// All enables all processing capabilities.
func (c *AgentCapabilities) All() *AgentCapabilities {
	return c.
		HandleRequestHeaders().
		HandleRequestBody().
		HandleResponseHeaders().
		HandleResponseBody().
		WithStreaming().
		WithCancellation()
}

// Clone creates a deep copy of the capabilities.
func (c *AgentCapabilities) Clone() *AgentCapabilities {
	clone := &AgentCapabilities{
		HandlesRequestHeaders:  c.HandlesRequestHeaders,
		HandlesRequestBody:     c.HandlesRequestBody,
		HandlesResponseHeaders: c.HandlesResponseHeaders,
		HandlesResponseBody:    c.HandlesResponseBody,
		SupportsStreaming:      c.SupportsStreaming,
		SupportsCancellation:   c.SupportsCancellation,
		AgentID:                c.AgentID,
		Version:                c.Version,
		MaxBodySize:            c.MaxBodySize,
	}
	if c.MaxConcurrentRequests != nil {
		max := *c.MaxConcurrentRequests
		clone.MaxConcurrentRequests = &max
	}
	if c.SupportedFeatures != nil {
		clone.SupportedFeatures = make([]string, len(c.SupportedFeatures))
		copy(clone.SupportedFeatures, c.SupportedFeatures)
	}
	return clone
}

// HasFeature checks if a specific feature is supported.
func (c *AgentCapabilities) HasFeature(feature string) bool {
	for _, f := range c.SupportedFeatures {
		if f == feature {
			return true
		}
	}
	return false
}

// Event type numbers of the handshake's supported_events (the proto's
// EventType enum).
const (
	eventRequestHeaders    int32 = 1
	eventRequestBodyChunk  int32 = 2
	eventResponseHeaders   int32 = 3
	eventResponseBodyChunk int32 = 4
	eventRequestComplete   int32 = 5
)

const (
	defaultMaxBodySize        = 10 * 1024 * 1024
	defaultMaxConcurrency     = 100
	defaultPreferredChunkSize = 64 * 1024
)

func (c *AgentCapabilities) supportedEvents() []int32 {
	events := []int32{}
	if c.HandlesRequestHeaders {
		events = append(events, eventRequestHeaders)
	}
	if c.HandlesRequestBody {
		events = append(events, eventRequestBodyChunk)
	}
	if c.HandlesResponseHeaders {
		events = append(events, eventResponseHeaders)
	}
	if c.HandlesResponseBody {
		events = append(events, eventResponseBodyChunk)
	}
	return append(events, eventRequestComplete)
}

func (c *AgentCapabilities) identity(name string) (id, version string) {
	id, version = c.AgentID, c.Version
	if id == "" {
		id = name
	}
	if version == "" {
		version = zentinel.Version
	}
	return id, version
}

func (c *AgentCapabilities) maxConcurrency() uint32 {
	if c.MaxConcurrentRequests != nil {
		return *c.MaxConcurrentRequests
	}
	return defaultMaxConcurrency
}

func (c *AgentCapabilities) maxBodySize() uint64 {
	if c.MaxBodySize != 0 {
		return c.MaxBodySize
	}
	return defaultMaxBodySize
}

// ToUds converts the capabilities to the handshake form of the Unix socket
// transport, for an agent called name.
func (c *AgentCapabilities) ToUds(name string) UdsCapabilities {
	id, version := c.identity(name)
	return UdsCapabilities{
		AgentID:         id,
		Name:            name,
		Version:         version,
		SupportedEvents: c.supportedEvents(),
		Features: UdsFeatures{
			StreamingBody:      c.SupportsStreaming,
			ConfigPush:         true,
			ConcurrentRequests: c.maxConcurrency(),
			Cancellation:       c.SupportsCancellation,
		},
		Limits: UdsLimits{
			MaxBodySize:        c.maxBodySize(),
			MaxConcurrency:     c.maxConcurrency(),
			PreferredChunkSize: defaultPreferredChunkSize,
		},
	}
}

// toProto converts the capabilities to the gRPC handshake form.
func (c *AgentCapabilities) toProto(name string) *pb.AgentCapabilities {
	id, version := c.identity(name)
	return &pb.AgentCapabilities{
		ProtocolVersion: ProtocolVersionV2,
		AgentId:         id,
		Name:            name,
		Version:         version,
		SupportedEvents: c.supportedEvents(),
		Features: &pb.AgentFeatures{
			StreamingBody:      c.SupportsStreaming,
			ConfigPush:         true,
			ConcurrentRequests: c.maxConcurrency(),
			Cancellation:       c.SupportsCancellation,
		},
		Limits: &pb.AgentLimits{
			MaxBodySize:        c.maxBodySize(),
			MaxConcurrency:     c.maxConcurrency(),
			PreferredChunkSize: defaultPreferredChunkSize,
		},
	}
}
