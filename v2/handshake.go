package v2

import (
	"encoding/json"
)

// ProtocolVersionV2 is the v2 protocol version.
const ProtocolVersionV2 = 2

// HandshakeRequest is sent by the proxy to initiate the v2 handshake.
type HandshakeRequest struct {
	// ProtocolVersion must be 2 for v2 protocol.
	ProtocolVersion uint32 `json:"protocol_version"`

	// ClientName identifies the connecting proxy.
	ClientName string `json:"client_name"`

	// SupportedFeatures lists features the proxy supports.
	SupportedFeatures []string `json:"supported_features,omitempty"`
}

// HandshakeResponse is sent by the agent in response to HandshakeRequest.
type HandshakeResponse struct {
	// ProtocolVersion confirms the protocol version.
	ProtocolVersion uint32 `json:"protocol_version"`

	// AgentName identifies this agent.
	AgentName string `json:"agent_name"`

	// Capabilities describes what the agent can process.
	Capabilities *AgentCapabilities `json:"capabilities"`

	// Error is set if the handshake failed.
	Error string `json:"error,omitempty"`

	// Accepted indicates whether the handshake was accepted.
	Accepted bool `json:"accepted"`
}

// NewHandshakeRequest creates a new handshake request.
func NewHandshakeRequest(clientName string) *HandshakeRequest {
	return &HandshakeRequest{
		ProtocolVersion:   ProtocolVersionV2,
		ClientName:        clientName,
		SupportedFeatures: []string{},
	}
}

// WithFeature adds a supported feature to the handshake request.
func (r *HandshakeRequest) WithFeature(feature string) *HandshakeRequest {
	r.SupportedFeatures = append(r.SupportedFeatures, feature)
	return r
}

// WithFeatures adds multiple supported features to the handshake request.
func (r *HandshakeRequest) WithFeatures(features ...string) *HandshakeRequest {
	r.SupportedFeatures = append(r.SupportedFeatures, features...)
	return r
}

// NewHandshakeResponse creates an accepted handshake response.
func NewHandshakeResponse(agentName string, capabilities *AgentCapabilities) *HandshakeResponse {
	return &HandshakeResponse{
		ProtocolVersion: ProtocolVersionV2,
		AgentName:       agentName,
		Capabilities:    capabilities,
		Accepted:        true,
	}
}

// NewHandshakeResponseError creates a rejected handshake response.
func NewHandshakeResponseError(agentName string, err string) *HandshakeResponse {
	return &HandshakeResponse{
		ProtocolVersion: ProtocolVersionV2,
		AgentName:       agentName,
		Capabilities:    nil,
		Accepted:        false,
		Error:           err,
	}
}

// RegistrationRequest is sent by an agent initiating a reverse connection.
type RegistrationRequest struct {
	// ProtocolVersion must be 2 for v2 protocol.
	ProtocolVersion uint32 `json:"protocol_version"`

	// AgentID uniquely identifies this agent.
	AgentID string `json:"agent_id"`

	// Capabilities describes what the agent can process.
	Capabilities *AgentCapabilities `json:"capabilities"`

	// AuthToken is an optional authentication token.
	AuthToken string `json:"auth_token,omitempty"`

	// Metadata contains additional agent information.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// RegistrationResponse is sent by the proxy in response to RegistrationRequest.
type RegistrationResponse struct {
	// Accepted indicates whether the registration was accepted.
	Accepted bool `json:"accepted"`

	// Error is set if the registration was rejected.
	Error string `json:"error,omitempty"`

	// AssignedID is the proxy-assigned connection ID.
	AssignedID string `json:"assigned_id,omitempty"`

	// Config is optional configuration pushed by the proxy.
	Config map[string]interface{} `json:"config,omitempty"`
}

// NewRegistrationRequest creates a new registration request.
func NewRegistrationRequest(agentID string, capabilities *AgentCapabilities) *RegistrationRequest {
	return &RegistrationRequest{
		ProtocolVersion: ProtocolVersionV2,
		AgentID:         agentID,
		Capabilities:    capabilities,
		Metadata:        make(map[string]interface{}),
	}
}

// WithAuthToken sets the authentication token.
func (r *RegistrationRequest) WithAuthToken(token string) *RegistrationRequest {
	r.AuthToken = token
	return r
}

// WithMetadata sets a metadata key-value pair.
func (r *RegistrationRequest) WithMetadata(key string, value interface{}) *RegistrationRequest {
	if r.Metadata == nil {
		r.Metadata = make(map[string]interface{})
	}
	r.Metadata[key] = value
	return r
}

// NewRegistrationResponseAccepted creates an accepted registration response.
func NewRegistrationResponseAccepted(assignedID string) *RegistrationResponse {
	return &RegistrationResponse{
		Accepted:   true,
		AssignedID: assignedID,
	}
}

// NewRegistrationResponseRejected creates a rejected registration response.
func NewRegistrationResponseRejected(err string) *RegistrationResponse {
	return &RegistrationResponse{
		Accepted: false,
		Error:    err,
	}
}

// WithConfig adds configuration to the registration response.
func (r *RegistrationResponse) WithConfig(config map[string]interface{}) *RegistrationResponse {
	r.Config = config
	return r
}

// MarshalHandshakeRequest marshals a HandshakeRequest to JSON.
func MarshalHandshakeRequest(req *HandshakeRequest) ([]byte, error) {
	return json.Marshal(req)
}

// UnmarshalHandshakeRequest unmarshals JSON to a HandshakeRequest.
func UnmarshalHandshakeRequest(data []byte) (*HandshakeRequest, error) {
	var req HandshakeRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

// MarshalHandshakeResponse marshals a HandshakeResponse to JSON.
func MarshalHandshakeResponse(resp *HandshakeResponse) ([]byte, error) {
	return json.Marshal(resp)
}

// UnmarshalHandshakeResponse unmarshals JSON to a HandshakeResponse.
func UnmarshalHandshakeResponse(data []byte) (*HandshakeResponse, error) {
	var resp HandshakeResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// MarshalRegistrationRequest marshals a RegistrationRequest to JSON.
func MarshalRegistrationRequest(req *RegistrationRequest) ([]byte, error) {
	return json.Marshal(req)
}

// UnmarshalRegistrationRequest unmarshals JSON to a RegistrationRequest.
func UnmarshalRegistrationRequest(data []byte) (*RegistrationRequest, error) {
	var req RegistrationRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

// MarshalRegistrationResponse marshals a RegistrationResponse to JSON.
func MarshalRegistrationResponse(resp *RegistrationResponse) ([]byte, error) {
	return json.Marshal(resp)
}

// UnmarshalRegistrationResponse unmarshals JSON to a RegistrationResponse.
func UnmarshalRegistrationResponse(data []byte) (*RegistrationResponse, error) {
	var resp RegistrationResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
