package zentinel

import (
	"context"
	"testing"
)

// TestAgent is a simple test agent that uses BaseAgent.
type TestAgent struct {
	BaseAgent
}

func (a *TestAgent) Name() string {
	return "test-agent"
}

func TestBaseAgent_DefaultHandlers(t *testing.T) {
	agent := &TestAgent{}
	ctx := context.Background()

	// Create test request
	event := &RequestHeadersEvent{
		Metadata: RequestMetadata{
			CorrelationID: "test",
			RequestID:     "req",
			ClientIP:      "127.0.0.1",
			ClientPort:    1234,
		},
		Method:  "GET",
		URI:     "/test",
		Headers: map[string][]string{},
	}
	request := NewRequest(event, nil)

	// Test OnRequest default
	decision := agent.OnRequest(ctx, request)
	response := decision.Build()
	if response.Decision != "allow" {
		t.Errorf("expected default OnRequest to return 'allow', got %v", response.Decision)
	}

	// Test OnRequestBody default
	decision = agent.OnRequestBody(ctx, request)
	response = decision.Build()
	if response.Decision != "allow" {
		t.Errorf("expected default OnRequestBody to return 'allow', got %v", response.Decision)
	}

	// Test OnResponse default
	respEvent := &ResponseHeadersEvent{
		CorrelationID: "test",
		Status:        200,
		Headers:       map[string][]string{},
	}
	resp := NewResponse(respEvent, nil)
	decision = agent.OnResponse(ctx, request, resp)
	response = decision.Build()
	if response.Decision != "allow" {
		t.Errorf("expected default OnResponse to return 'allow', got %v", response.Decision)
	}

	// Test OnResponseBody default
	decision = agent.OnResponseBody(ctx, request, resp)
	response = decision.Build()
	if response.Decision != "allow" {
		t.Errorf("expected default OnResponseBody to return 'allow', got %v", response.Decision)
	}
}

func TestBaseAgent_OnConfigure(t *testing.T) {
	agent := &TestAgent{}
	ctx := context.Background()

	config := map[string]interface{}{
		"key": "value",
	}

	err := agent.OnConfigure(ctx, config)
	if err != nil {
		t.Errorf("expected OnConfigure to return nil error, got %v", err)
	}
}

func TestBaseAgent_OnRequestComplete(t *testing.T) {
	agent := &TestAgent{}
	ctx := context.Background()

	event := &RequestHeadersEvent{
		Metadata: RequestMetadata{
			CorrelationID: "test",
			RequestID:     "req",
			ClientIP:      "127.0.0.1",
			ClientPort:    1234,
		},
		Method:  "GET",
		URI:     "/test",
		Headers: map[string][]string{},
	}
	request := NewRequest(event, nil)

	// Should not panic
	agent.OnRequestComplete(ctx, request, 200, 100)
}

func TestBaseAgent_Name(t *testing.T) {
	agent := &BaseAgent{}

	if agent.Name() != "agent" {
		t.Errorf("expected BaseAgent.Name() 'agent', got %s", agent.Name())
	}
}

// CustomAgent is a test agent with custom handlers.
type CustomAgent struct {
	BaseAgent
	onRequestCalled bool
}

func (a *CustomAgent) Name() string {
	return "custom-agent"
}

func (a *CustomAgent) OnRequest(ctx context.Context, request *Request) *Decision {
	a.onRequestCalled = true
	if request.PathStartsWith("/blocked") {
		return Deny().WithBody("Blocked")
	}
	return Allow()
}

func TestCustomAgent_OnRequest(t *testing.T) {
	agent := &CustomAgent{}
	ctx := context.Background()

	// Test blocked path
	event := &RequestHeadersEvent{
		Metadata: RequestMetadata{
			CorrelationID: "test",
			RequestID:     "req",
			ClientIP:      "127.0.0.1",
			ClientPort:    1234,
		},
		Method:  "GET",
		URI:     "/blocked/resource",
		Headers: map[string][]string{},
	}
	request := NewRequest(event, nil)

	decision := agent.OnRequest(ctx, request)
	response := decision.Build()

	if !agent.onRequestCalled {
		t.Error("expected OnRequest to be called")
	}

	decisionMap, ok := response.Decision.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map decision, got %T", response.Decision)
	}

	block := decisionMap["block"].(map[string]interface{})
	if block["status"] != 403 {
		t.Errorf("expected status 403, got %v", block["status"])
	}
	if block["body"] != "Blocked" {
		t.Errorf("expected body 'Blocked', got %v", block["body"])
	}

	// Test allowed path
	event2 := &RequestHeadersEvent{
		Metadata: RequestMetadata{
			CorrelationID: "test2",
			RequestID:     "req2",
			ClientIP:      "127.0.0.1",
			ClientPort:    1234,
		},
		Method:  "GET",
		URI:     "/allowed/resource",
		Headers: map[string][]string{},
	}
	request2 := NewRequest(event2, nil)

	decision2 := agent.OnRequest(ctx, request2)
	response2 := decision2.Build()

	if response2.Decision != "allow" {
		t.Errorf("expected 'allow' decision for allowed path, got %v", response2.Decision)
	}
}

// TestConfig is a configuration struct for testing.
type TestConfig struct {
	Enabled   bool   `json:"enabled"`
	RateLimit int    `json:"rate_limit"`
	Name      string `json:"name"`
}

// ConfigurableTestAgent is a test agent with configuration.
type ConfigurableTestAgent struct {
	*ConfigurableAgentBase[TestConfig]
	configAppliedCalled bool
}

func NewConfigurableTestAgent() *ConfigurableTestAgent {
	return &ConfigurableTestAgent{
		ConfigurableAgentBase: NewConfigurableAgent(TestConfig{
			Enabled:   true,
			RateLimit: 100,
			Name:      "default",
		}),
	}
}

func (a *ConfigurableTestAgent) Name() string {
	return "configurable-test-agent"
}

// OnConfigure overrides the base implementation to call our custom OnConfigApplied.
// In Go, embedded struct methods don't support virtual dispatch, so we need to override.
func (a *ConfigurableTestAgent) OnConfigure(ctx context.Context, configMap map[string]interface{}) error {
	config, err := a.ParseConfig(configMap)
	if err != nil {
		return err
	}
	a.SetConfig(config)
	a.onConfigApplied(ctx, config)
	return nil
}

func (a *ConfigurableTestAgent) onConfigApplied(ctx context.Context, config TestConfig) {
	a.configAppliedCalled = true
}

func TestConfigurableAgent_DefaultConfig(t *testing.T) {
	agent := NewConfigurableTestAgent()

	config := agent.Config()
	if !config.Enabled {
		t.Error("expected default Enabled to be true")
	}
	if config.RateLimit != 100 {
		t.Errorf("expected default RateLimit 100, got %d", config.RateLimit)
	}
	if config.Name != "default" {
		t.Errorf("expected default Name 'default', got %s", config.Name)
	}
}

func TestConfigurableAgent_OnConfigure(t *testing.T) {
	agent := NewConfigurableTestAgent()
	ctx := context.Background()

	configMap := map[string]interface{}{
		"enabled":    false,
		"rate_limit": 200,
		"name":       "custom",
	}

	err := agent.OnConfigure(ctx, configMap)
	if err != nil {
		t.Fatalf("OnConfigure failed: %v", err)
	}

	config := agent.Config()
	if config.Enabled {
		t.Error("expected Enabled to be false after configure")
	}
	if config.RateLimit != 200 {
		t.Errorf("expected RateLimit 200 after configure, got %d", config.RateLimit)
	}
	if config.Name != "custom" {
		t.Errorf("expected Name 'custom' after configure, got %s", config.Name)
	}

	if !agent.configAppliedCalled {
		t.Error("expected OnConfigApplied to be called")
	}
}

func TestConfigurableAgent_SetConfig(t *testing.T) {
	agent := NewConfigurableTestAgent()

	newConfig := TestConfig{
		Enabled:   false,
		RateLimit: 50,
		Name:      "set-directly",
	}
	agent.SetConfig(newConfig)

	config := agent.Config()
	if config.RateLimit != 50 {
		t.Errorf("expected RateLimit 50 after SetConfig, got %d", config.RateLimit)
	}
}

func TestConfigurableAgent_ParseConfig(t *testing.T) {
	agent := NewConfigurableTestAgent()

	configMap := map[string]interface{}{
		"enabled":    true,
		"rate_limit": 300,
		"name":       "parsed",
	}

	config, err := agent.ParseConfig(configMap)
	if err != nil {
		t.Fatalf("ParseConfig failed: %v", err)
	}

	if !config.Enabled {
		t.Error("expected parsed Enabled to be true")
	}
	if config.RateLimit != 300 {
		t.Errorf("expected parsed RateLimit 300, got %d", config.RateLimit)
	}
	if config.Name != "parsed" {
		t.Errorf("expected parsed Name 'parsed', got %s", config.Name)
	}
}

func TestConfigurableAgent_ParseConfig_InvalidJSON(t *testing.T) {
	agent := NewConfigurableTestAgent()

	// Using invalid types that can't be parsed
	configMap := map[string]interface{}{
		"enabled":    "not-a-bool", // This might work due to Go's JSON behavior
		"rate_limit": "not-a-number",
	}

	_, err := agent.ParseConfig(configMap)
	if err == nil {
		t.Error("expected ParseConfig to fail with invalid types")
	}
}

// AgentWithResponse tests an agent that modifies responses.
type AgentWithResponse struct {
	BaseAgent
}

func (a *AgentWithResponse) Name() string {
	return "response-agent"
}

func (a *AgentWithResponse) OnResponse(ctx context.Context, request *Request, response *Response) *Decision {
	if response.IsHTML() {
		return Allow().
			AddResponseHeader("X-Content-Type-Options", "nosniff").
			AddResponseHeader("X-Frame-Options", "DENY")
	}
	return Allow()
}

func TestAgentWithResponse_OnResponse(t *testing.T) {
	agent := &AgentWithResponse{}
	ctx := context.Background()

	// Create request
	reqEvent := &RequestHeadersEvent{
		Metadata: RequestMetadata{
			CorrelationID: "test",
			RequestID:     "req",
			ClientIP:      "127.0.0.1",
			ClientPort:    1234,
		},
		Method:  "GET",
		URI:     "/page",
		Headers: map[string][]string{},
	}
	request := NewRequest(reqEvent, nil)

	// Create HTML response
	respEvent := &ResponseHeadersEvent{
		CorrelationID: "test",
		Status:        200,
		Headers: map[string][]string{
			"Content-Type": {"text/html"},
		},
	}
	response := NewResponse(respEvent, nil)

	decision := agent.OnResponse(ctx, request, response)
	agentResponse := decision.Build()

	if len(agentResponse.ResponseHeaders) != 2 {
		t.Fatalf("expected 2 response headers, got %d", len(agentResponse.ResponseHeaders))
	}

	// Create JSON response
	respEvent2 := &ResponseHeadersEvent{
		CorrelationID: "test2",
		Status:        200,
		Headers: map[string][]string{
			"Content-Type": {"application/json"},
		},
	}
	response2 := NewResponse(respEvent2, nil)

	decision2 := agent.OnResponse(ctx, request, response2)
	agentResponse2 := decision2.Build()

	if len(agentResponse2.ResponseHeaders) != 0 {
		t.Errorf("expected 0 response headers for JSON, got %d", len(agentResponse2.ResponseHeaders))
	}
}
