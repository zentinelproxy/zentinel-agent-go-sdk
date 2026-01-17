package v2

import (
	"testing"
)

func TestHandshakeRequest(t *testing.T) {
	req := NewHandshakeRequest("proxy-1").
		WithFeature("streaming").
		WithFeatures("cancellation", "metrics")

	if req.ProtocolVersion != ProtocolVersionV2 {
		t.Errorf("expected protocol version %d, got %d", ProtocolVersionV2, req.ProtocolVersion)
	}
	if req.ClientName != "proxy-1" {
		t.Errorf("expected client name 'proxy-1', got %s", req.ClientName)
	}
	if len(req.SupportedFeatures) != 3 {
		t.Errorf("expected 3 features, got %d", len(req.SupportedFeatures))
	}
}

func TestHandshakeResponse_Accepted(t *testing.T) {
	caps := NewAgentCapabilities().HandleRequestHeaders().HandleRequestBody()
	resp := NewHandshakeResponse("my-agent", caps)

	if !resp.Accepted {
		t.Error("expected response to be accepted")
	}
	if resp.ProtocolVersion != ProtocolVersionV2 {
		t.Errorf("expected protocol version %d, got %d", ProtocolVersionV2, resp.ProtocolVersion)
	}
	if resp.AgentName != "my-agent" {
		t.Errorf("expected agent name 'my-agent', got %s", resp.AgentName)
	}
	if resp.Capabilities == nil {
		t.Error("expected capabilities to be set")
	}
	if !resp.Capabilities.HandlesRequestHeaders {
		t.Error("expected HandlesRequestHeaders to be true")
	}
	if !resp.Capabilities.HandlesRequestBody {
		t.Error("expected HandlesRequestBody to be true")
	}
	if resp.Error != "" {
		t.Errorf("expected no error, got %s", resp.Error)
	}
}

func TestHandshakeResponse_Error(t *testing.T) {
	resp := NewHandshakeResponseError("my-agent", "unsupported version")

	if resp.Accepted {
		t.Error("expected response to be rejected")
	}
	if resp.Error != "unsupported version" {
		t.Errorf("expected error 'unsupported version', got %s", resp.Error)
	}
	if resp.Capabilities != nil {
		t.Error("expected capabilities to be nil on error")
	}
}

func TestRegistrationRequest(t *testing.T) {
	caps := NewAgentCapabilities().All()
	req := NewRegistrationRequest("waf-agent", caps).
		WithAuthToken("secret-token").
		WithMetadata("region", "us-east").
		WithMetadata("version", "1.0.0")

	if req.ProtocolVersion != ProtocolVersionV2 {
		t.Errorf("expected protocol version %d, got %d", ProtocolVersionV2, req.ProtocolVersion)
	}
	if req.AgentID != "waf-agent" {
		t.Errorf("expected agent ID 'waf-agent', got %s", req.AgentID)
	}
	if req.AuthToken != "secret-token" {
		t.Errorf("expected auth token 'secret-token', got %s", req.AuthToken)
	}
	if req.Metadata["region"] != "us-east" {
		t.Errorf("expected region 'us-east', got %v", req.Metadata["region"])
	}
	if req.Metadata["version"] != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %v", req.Metadata["version"])
	}
}

func TestRegistrationResponse_Accepted(t *testing.T) {
	config := map[string]interface{}{
		"rate_limit": 1000,
	}
	resp := NewRegistrationResponseAccepted("conn-123").
		WithConfig(config)

	if !resp.Accepted {
		t.Error("expected response to be accepted")
	}
	if resp.AssignedID != "conn-123" {
		t.Errorf("expected assigned ID 'conn-123', got %s", resp.AssignedID)
	}
	if resp.Config["rate_limit"] != 1000 {
		t.Errorf("expected rate_limit 1000, got %v", resp.Config["rate_limit"])
	}
}

func TestRegistrationResponse_Rejected(t *testing.T) {
	resp := NewRegistrationResponseRejected("authentication failed")

	if resp.Accepted {
		t.Error("expected response to be rejected")
	}
	if resp.Error != "authentication failed" {
		t.Errorf("expected error 'authentication failed', got %s", resp.Error)
	}
}

func TestHandshakeMarshalUnmarshal(t *testing.T) {
	// Test HandshakeRequest
	req := NewHandshakeRequest("test-proxy").WithFeature("feature1")
	reqBytes, err := MarshalHandshakeRequest(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	parsedReq, err := UnmarshalHandshakeRequest(reqBytes)
	if err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	if parsedReq.ClientName != req.ClientName {
		t.Errorf("client name mismatch: %s vs %s", parsedReq.ClientName, req.ClientName)
	}
	if parsedReq.ProtocolVersion != req.ProtocolVersion {
		t.Errorf("protocol version mismatch: %d vs %d", parsedReq.ProtocolVersion, req.ProtocolVersion)
	}

	// Test HandshakeResponse
	caps := NewAgentCapabilities().HandleRequestBody()
	resp := NewHandshakeResponse("test-agent", caps)
	respBytes, err := MarshalHandshakeResponse(resp)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	parsedResp, err := UnmarshalHandshakeResponse(respBytes)
	if err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if parsedResp.AgentName != resp.AgentName {
		t.Errorf("agent name mismatch: %s vs %s", parsedResp.AgentName, resp.AgentName)
	}
	if !parsedResp.Accepted {
		t.Error("expected accepted to be true")
	}
}

func TestRegistrationMarshalUnmarshal(t *testing.T) {
	// Test RegistrationRequest
	caps := NewAgentCapabilities()
	req := NewRegistrationRequest("test-agent", caps).WithAuthToken("token")
	reqBytes, err := MarshalRegistrationRequest(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	parsedReq, err := UnmarshalRegistrationRequest(reqBytes)
	if err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	if parsedReq.AgentID != req.AgentID {
		t.Errorf("agent ID mismatch: %s vs %s", parsedReq.AgentID, req.AgentID)
	}
	if parsedReq.AuthToken != req.AuthToken {
		t.Errorf("auth token mismatch: %s vs %s", parsedReq.AuthToken, req.AuthToken)
	}

	// Test RegistrationResponse
	resp := NewRegistrationResponseAccepted("conn-456")
	respBytes, err := MarshalRegistrationResponse(resp)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	parsedResp, err := UnmarshalRegistrationResponse(respBytes)
	if err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if parsedResp.AssignedID != resp.AssignedID {
		t.Errorf("assigned ID mismatch: %s vs %s", parsedResp.AssignedID, resp.AssignedID)
	}
	if !parsedResp.Accepted {
		t.Error("expected accepted to be true")
	}
}
