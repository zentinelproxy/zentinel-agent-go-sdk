package v2

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
