package zentinel

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestDecision_Allow(t *testing.T) {
	decision := Allow()
	response := decision.Build()

	if response.Decision != "allow" {
		t.Errorf("expected decision 'allow', got %v", response.Decision)
	}
	if response.Version != ProtocolVersion {
		t.Errorf("expected version %d, got %d", ProtocolVersion, response.Version)
	}
}

func TestDecision_Deny(t *testing.T) {
	decision := Deny()
	response := decision.Build()

	expected := map[string]interface{}{
		"block": map[string]interface{}{
			"status": 403,
		},
	}

	decisionMap, ok := response.Decision.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map decision, got %T", response.Decision)
	}

	block, ok := decisionMap["block"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected block map, got %T", decisionMap["block"])
	}

	if block["status"] != expected["block"].(map[string]interface{})["status"] {
		t.Errorf("expected status 403, got %v", block["status"])
	}
}

func TestDecision_BlockWithStatus(t *testing.T) {
	decision := Block(500)
	response := decision.Build()

	decisionMap, ok := response.Decision.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map decision, got %T", response.Decision)
	}

	block := decisionMap["block"].(map[string]interface{})
	if block["status"] != 500 {
		t.Errorf("expected status 500, got %v", block["status"])
	}
}

func TestDecision_BlockWithBody(t *testing.T) {
	decision := Deny().WithBody("Access denied")
	response := decision.Build()

	decisionMap := response.Decision.(map[string]interface{})
	block := decisionMap["block"].(map[string]interface{})

	if block["body"] != "Access denied" {
		t.Errorf("expected body 'Access denied', got %v", block["body"])
	}
}

func TestDecision_Redirect(t *testing.T) {
	decision := Redirect("/login", 302)
	response := decision.Build()

	decisionMap := response.Decision.(map[string]interface{})
	redirect := decisionMap["redirect"].(map[string]interface{})

	if redirect["url"] != "/login" {
		t.Errorf("expected url '/login', got %v", redirect["url"])
	}
	if redirect["status"] != 302 {
		t.Errorf("expected status 302, got %v", redirect["status"])
	}
}

func TestDecision_RedirectPermanent(t *testing.T) {
	decision := RedirectPermanent("/new-path")
	response := decision.Build()

	decisionMap := response.Decision.(map[string]interface{})
	redirect := decisionMap["redirect"].(map[string]interface{})

	if redirect["url"] != "/new-path" {
		t.Errorf("expected url '/new-path', got %v", redirect["url"])
	}
	if redirect["status"] != 301 {
		t.Errorf("expected status 301, got %v", redirect["status"])
	}
}

func TestDecision_Unauthorized(t *testing.T) {
	decision := Unauthorized()
	response := decision.Build()

	decisionMap := response.Decision.(map[string]interface{})
	block := decisionMap["block"].(map[string]interface{})

	if block["status"] != 401 {
		t.Errorf("expected status 401, got %v", block["status"])
	}
}

func TestDecision_RateLimited(t *testing.T) {
	decision := RateLimited()
	response := decision.Build()

	decisionMap := response.Decision.(map[string]interface{})
	block := decisionMap["block"].(map[string]interface{})

	if block["status"] != 429 {
		t.Errorf("expected status 429, got %v", block["status"])
	}
}

func TestDecision_Challenge(t *testing.T) {
	params := map[string]interface{}{"site_key": "abc123"}
	decision := Challenge("captcha", params)
	response := decision.Build()

	decisionMap := response.Decision.(map[string]interface{})
	challenge := decisionMap["challenge"].(map[string]interface{})

	if challenge["challenge_type"] != "captcha" {
		t.Errorf("expected challenge_type 'captcha', got %v", challenge["challenge_type"])
	}
	if challenge["params"].(map[string]interface{})["site_key"] != "abc123" {
		t.Errorf("expected site_key 'abc123', got %v", challenge["params"])
	}
}

func TestDecision_AddRequestHeader(t *testing.T) {
	decision := Allow().AddRequestHeader("X-Test", "value")
	response := decision.Build()

	if len(response.RequestHeaders) != 1 {
		t.Fatalf("expected 1 request header, got %d", len(response.RequestHeaders))
	}

	header := response.RequestHeaders[0]
	if header.Name != "X-Test" {
		t.Errorf("expected header name 'X-Test', got %s", header.Name)
	}
	if *header.Value != "value" {
		t.Errorf("expected header value 'value', got %s", *header.Value)
	}
}

func TestDecision_AddResponseHeader(t *testing.T) {
	decision := Allow().AddResponseHeader("X-Test", "value")
	response := decision.Build()

	if len(response.ResponseHeaders) != 1 {
		t.Fatalf("expected 1 response header, got %d", len(response.ResponseHeaders))
	}

	header := response.ResponseHeaders[0]
	if header.Name != "X-Test" {
		t.Errorf("expected header name 'X-Test', got %s", header.Name)
	}
}

func TestDecision_RemoveHeader(t *testing.T) {
	decision := Allow().RemoveRequestHeader("X-Remove")
	response := decision.Build()

	if len(response.RequestHeaders) != 1 {
		t.Fatalf("expected 1 request header, got %d", len(response.RequestHeaders))
	}

	header := response.RequestHeaders[0]
	if header.Operation != "remove" {
		t.Errorf("expected operation 'remove', got %s", header.Operation)
	}
	if header.Name != "X-Remove" {
		t.Errorf("expected header name 'X-Remove', got %s", header.Name)
	}
}

func TestDecision_AuditTags(t *testing.T) {
	decision := Deny().WithTag("security").WithTags("blocked", "test")
	response := decision.Build()

	expected := []string{"security", "blocked", "test"}
	if len(response.Audit.Tags) != len(expected) {
		t.Fatalf("expected %d tags, got %d", len(expected), len(response.Audit.Tags))
	}
	for i, tag := range expected {
		if response.Audit.Tags[i] != tag {
			t.Errorf("expected tag %s at index %d, got %s", tag, i, response.Audit.Tags[i])
		}
	}
}

func TestDecision_AuditMetadata(t *testing.T) {
	decision := Deny().WithMetadata("client_ip", "1.2.3.4")
	response := decision.Build()

	if response.Audit.Custom["client_ip"] != "1.2.3.4" {
		t.Errorf("expected client_ip '1.2.3.4', got %v", response.Audit.Custom["client_ip"])
	}
}

func TestDecision_Chaining(t *testing.T) {
	decision := Deny().
		WithBody("Blocked").
		WithTag("security").
		WithRuleID("RULE_001").
		WithConfidence(0.95).
		AddResponseHeader("X-Blocked", "true")

	response := decision.Build()

	decisionMap := response.Decision.(map[string]interface{})
	block := decisionMap["block"].(map[string]interface{})

	if block["body"] != "Blocked" {
		t.Errorf("expected body 'Blocked', got %v", block["body"])
	}
	if len(response.Audit.Tags) != 1 || response.Audit.Tags[0] != "security" {
		t.Errorf("expected tags ['security'], got %v", response.Audit.Tags)
	}
	if len(response.Audit.RuleIDs) != 1 || response.Audit.RuleIDs[0] != "RULE_001" {
		t.Errorf("expected rule_ids ['RULE_001'], got %v", response.Audit.RuleIDs)
	}
	if response.Audit.Confidence == nil || *response.Audit.Confidence != 0.95 {
		t.Errorf("expected confidence 0.95, got %v", response.Audit.Confidence)
	}
	if len(response.ResponseHeaders) != 1 {
		t.Errorf("expected 1 response header, got %d", len(response.ResponseHeaders))
	}
}

func TestDecision_NeedsMoreData(t *testing.T) {
	decision := Allow().NeedsMoreData()
	response := decision.Build()

	if !response.NeedsMore {
		t.Error("expected needs_more to be true")
	}
}

func TestDecision_WithRequestBodyMutation(t *testing.T) {
	data := []byte("modified content")
	decision := Allow().WithRequestBodyMutation(data, 0)
	response := decision.Build()

	if response.RequestBodyMutation == nil {
		t.Fatal("expected request_body_mutation to be set")
	}

	mutation := response.RequestBodyMutation
	if mutation["chunk_index"] != 0 {
		t.Errorf("expected chunk_index 0, got %v", mutation["chunk_index"])
	}

	expectedData := base64.StdEncoding.EncodeToString(data)
	if *mutation["data"].(*string) != expectedData {
		t.Errorf("expected data '%s', got %v", expectedData, mutation["data"])
	}
}

func TestDecision_WithJSONBody(t *testing.T) {
	decision := Deny().WithJSONBody(map[string]string{"error": "forbidden"})
	response := decision.Build()

	decisionMap := response.Decision.(map[string]interface{})
	block := decisionMap["block"].(map[string]interface{})

	var body map[string]string
	if err := json.Unmarshal([]byte(block["body"].(string)), &body); err != nil {
		t.Fatalf("failed to parse JSON body: %v", err)
	}

	if body["error"] != "forbidden" {
		t.Errorf("expected error 'forbidden', got %s", body["error"])
	}

	headers := block["headers"].(map[string]string)
	if headers["Content-Type"] != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %s", headers["Content-Type"])
	}
}

func TestDecision_WithBlockHeader(t *testing.T) {
	decision := Deny().WithBody("Forbidden").WithBlockHeader("X-Reason", "policy")
	response := decision.Build()

	decisionMap := response.Decision.(map[string]interface{})
	block := decisionMap["block"].(map[string]interface{})
	headers := block["headers"].(map[string]string)

	if headers["X-Reason"] != "policy" {
		t.Errorf("expected X-Reason 'policy', got %s", headers["X-Reason"])
	}
}

func TestDecision_WithRoutingMetadata(t *testing.T) {
	decision := Allow().WithRoutingMetadata("upstream", "backend-v2")
	response := decision.Build()

	if response.RoutingMetadata["upstream"] != "backend-v2" {
		t.Errorf("expected upstream 'backend-v2', got %s", response.RoutingMetadata["upstream"])
	}
}

func TestDecision_WithReasonCode(t *testing.T) {
	decision := Deny().WithReasonCode("IP_BLOCKED")
	response := decision.Build()

	if len(response.Audit.ReasonCodes) != 1 || response.Audit.ReasonCodes[0] != "IP_BLOCKED" {
		t.Errorf("expected reason_codes ['IP_BLOCKED'], got %v", response.Audit.ReasonCodes)
	}
}
