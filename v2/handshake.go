package v2

// ProtocolVersionV2 is the v2 protocol version.
const ProtocolVersionV2 = 2

// HandshakeRequest is the first message the proxy sends on a Unix socket
// connection (zentinel-agent-protocol UdsHandshakeRequest).
type HandshakeRequest struct {
	SupportedVersions []uint32 `json:"supported_versions"`
	ProxyID           string   `json:"proxy_id"`
	ProxyVersion      string   `json:"proxy_version"`
	// Config is the agent's `config { }` block. The proxy also sends it as a
	// Configure message right after the handshake, which is what reaches
	// Agent.OnConfigure; the runner does not apply this copy.
	Config             interface{} `json:"config"`
	SupportedEncodings []string    `json:"supported_encodings,omitempty"`
}

// HandshakeResponse is the agent's answer (UdsHandshakeResponse). Every
// field is required by the proxy.
type HandshakeResponse struct {
	ProtocolVersion uint32          `json:"protocol_version"`
	Capabilities    UdsCapabilities `json:"capabilities"`
	Success         bool            `json:"success"`
	Error           *string         `json:"error"`
	// Encoding is the payload encoding for the rest of the connection. This
	// SDK speaks JSON only.
	Encoding string `json:"encoding"`
}

// UdsCapabilities is the capability block of the handshake response.
type UdsCapabilities struct {
	AgentID         string      `json:"agent_id"`
	Name            string      `json:"name"`
	Version         string      `json:"version"`
	SupportedEvents []int32     `json:"supported_events"`
	Features        UdsFeatures `json:"features"`
	Limits          UdsLimits   `json:"limits"`
}

// UdsFeatures lists the optional protocol features an agent supports.
type UdsFeatures struct {
	StreamingBody      bool   `json:"streaming_body"`
	Websocket          bool   `json:"websocket"`
	Guardrails         bool   `json:"guardrails"`
	ConfigPush         bool   `json:"config_push"`
	MetricsExport      bool   `json:"metrics_export"`
	ConcurrentRequests uint32 `json:"concurrent_requests"`
	Cancellation       bool   `json:"cancellation"`
	FlowControl        bool   `json:"flow_control"`
	HealthReporting    bool   `json:"health_reporting"`
}

// UdsLimits are the agent's processing limits.
type UdsLimits struct {
	MaxBodySize        uint64 `json:"max_body_size"`
	MaxConcurrency     uint32 `json:"max_concurrency"`
	PreferredChunkSize uint64 `json:"preferred_chunk_size"`
}

// NewHandshakeResponse creates an accepting handshake response.
func NewHandshakeResponse(caps UdsCapabilities) *HandshakeResponse {
	return &HandshakeResponse{
		ProtocolVersion: ProtocolVersionV2,
		Capabilities:    caps,
		Success:         true,
		Encoding:        "json",
	}
}

// NewHandshakeResponseError creates a rejecting handshake response.
func NewHandshakeResponseError(caps UdsCapabilities, err string) *HandshakeResponse {
	return &HandshakeResponse{
		ProtocolVersion: ProtocolVersionV2,
		Capabilities:    caps,
		Success:         false,
		Error:           &err,
		Encoding:        "json",
	}
}

// supportsVersion reports whether the proxy offered protocol v2. An empty
// list is taken as v2, the only version the proxy speaks.
func (r *HandshakeRequest) supportsVersion() bool {
	if len(r.SupportedVersions) == 0 {
		return true
	}
	for _, v := range r.SupportedVersions {
		if v == ProtocolVersionV2 {
			return true
		}
	}
	return false
}
