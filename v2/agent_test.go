package v2

import (
	"context"
	"testing"

	sentinel "github.com/raskell-io/sentinel-agent-go-sdk"
)

// TestAgentV2Impl is a test agent implementing AgentV2.
type TestAgentV2Impl struct {
	BaseAgentV2
	onRequestCalled    bool
	onShutdownCalled   bool
	onDrainCalled      bool
	onStreamClosedID   string
	onCancelRequestID  uint64
}

func (a *TestAgentV2Impl) Name() string {
	return "test-agent-v2"
}

func (a *TestAgentV2Impl) Capabilities() *AgentCapabilities {
	return NewAgentCapabilities().
		HandleRequestHeaders().
		HandleRequestBody().
		WithMaxConcurrentRequests(100)
}

func (a *TestAgentV2Impl) OnRequest(ctx context.Context, request *sentinel.Request) *sentinel.Decision {
	a.onRequestCalled = true
	if request.PathStartsWith("/blocked") {
		return sentinel.Deny().WithBody("Blocked by v2 agent")
	}
	return sentinel.Allow()
}

func (a *TestAgentV2Impl) OnShutdown(ctx context.Context) {
	a.onShutdownCalled = true
}

func (a *TestAgentV2Impl) OnDrain(ctx context.Context) {
	a.onDrainCalled = true
}

func (a *TestAgentV2Impl) OnStreamClosed(ctx context.Context, streamID string) {
	a.onStreamClosedID = streamID
}

func (a *TestAgentV2Impl) OnCancel(ctx context.Context, requestID uint64) {
	a.onCancelRequestID = requestID
}

func TestBaseAgentV2_Defaults(t *testing.T) {
	agent := &TestAgentV2Impl{}
	ctx := context.Background()

	// Test default health check
	health := agent.HealthCheck(ctx)
	if !health.IsHealthy() {
		t.Error("expected default health to be healthy")
	}

	// Test default metrics
	metrics := agent.Metrics(ctx)
	if metrics == nil {
		t.Error("expected metrics to be non-nil")
	}
}

func TestTestAgentV2_Capabilities(t *testing.T) {
	agent := &TestAgentV2Impl{}
	caps := agent.Capabilities()

	if !caps.HandlesRequestHeaders {
		t.Error("expected HandlesRequestHeaders to be true")
	}
	if !caps.HandlesRequestBody {
		t.Error("expected HandlesRequestBody to be true")
	}
	if caps.MaxConcurrentRequests == nil || *caps.MaxConcurrentRequests != 100 {
		t.Errorf("expected MaxConcurrentRequests to be 100, got %v", caps.MaxConcurrentRequests)
	}
}

func TestTestAgentV2_OnRequest(t *testing.T) {
	agent := &TestAgentV2Impl{}
	ctx := context.Background()

	// Test blocked path
	event := &sentinel.RequestHeadersEvent{
		Metadata: sentinel.RequestMetadata{
			CorrelationID: "test",
			RequestID:     "req",
			ClientIP:      "127.0.0.1",
			ClientPort:    1234,
		},
		Method:  "GET",
		URI:     "/blocked/resource",
		Headers: map[string][]string{},
	}
	request := sentinel.NewRequest(event, nil)

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

	// Test allowed path
	event2 := &sentinel.RequestHeadersEvent{
		Metadata: sentinel.RequestMetadata{
			CorrelationID: "test2",
			RequestID:     "req2",
			ClientIP:      "127.0.0.1",
			ClientPort:    1234,
		},
		Method:  "GET",
		URI:     "/allowed",
		Headers: map[string][]string{},
	}
	request2 := sentinel.NewRequest(event2, nil)

	decision2 := agent.OnRequest(ctx, request2)
	response2 := decision2.Build()

	if response2.Decision != "allow" {
		t.Errorf("expected 'allow' decision, got %v", response2.Decision)
	}
}

func TestTestAgentV2_LifecycleHooks(t *testing.T) {
	agent := &TestAgentV2Impl{}
	ctx := context.Background()

	// Test OnShutdown
	agent.OnShutdown(ctx)
	if !agent.onShutdownCalled {
		t.Error("expected OnShutdown to be called")
	}

	// Test OnDrain
	agent.OnDrain(ctx)
	if !agent.onDrainCalled {
		t.Error("expected OnDrain to be called")
	}

	// Test OnStreamClosed
	agent.OnStreamClosed(ctx, "stream-123")
	if agent.onStreamClosedID != "stream-123" {
		t.Errorf("expected stream ID 'stream-123', got %s", agent.onStreamClosedID)
	}

	// Test OnCancel
	agent.OnCancel(ctx, 456)
	if agent.onCancelRequestID != 456 {
		t.Errorf("expected request ID 456, got %d", agent.onCancelRequestID)
	}
}

func TestBaseAgentV2_MetricsCollectorRef(t *testing.T) {
	base := NewBaseAgentV2()
	collector := base.MetricsCollectorRef()

	if collector == nil {
		t.Error("expected metrics collector to be non-nil")
	}

	// Record some metrics
	collector.RecordRequest(true, 10.0)
	collector.IncrementActive()

	// Verify via Metrics()
	ctx := context.Background()
	report := base.Metrics(ctx)

	if report.RequestsTotal != 1 {
		t.Errorf("expected RequestsTotal 1, got %d", report.RequestsTotal)
	}
	if report.RequestsActive != 1 {
		t.Errorf("expected RequestsActive 1, got %d", report.RequestsActive)
	}
}

func TestBaseAgentV2_SetCapabilities(t *testing.T) {
	base := NewBaseAgentV2()

	// Default capabilities
	caps := base.Capabilities()
	if !caps.HandlesRequestHeaders {
		t.Error("expected default to handle request headers")
	}

	// Set custom capabilities
	customCaps := NewAgentCapabilities().All().WithMaxConcurrentRequests(50)
	base.SetCapabilities(customCaps)

	caps = base.Capabilities()
	if !caps.HandlesResponseBody {
		t.Error("expected custom caps to handle response body")
	}
	if caps.MaxConcurrentRequests == nil || *caps.MaxConcurrentRequests != 50 {
		t.Errorf("expected MaxConcurrentRequests 50, got %v", caps.MaxConcurrentRequests)
	}
}

// ConfigurableV2TestConfig is a test config for ConfigurableAgentV2.
type ConfigurableV2TestConfig struct {
	Enabled   bool   `json:"enabled"`
	RateLimit int    `json:"rate_limit"`
	Name      string `json:"name"`
}

// ConfigurableV2TestAgent is a test configurable v2 agent.
type ConfigurableV2TestAgent struct {
	*ConfigurableAgentV2Base[ConfigurableV2TestConfig]
}

func NewConfigurableV2TestAgent() *ConfigurableV2TestAgent {
	return &ConfigurableV2TestAgent{
		ConfigurableAgentV2Base: NewConfigurableAgentV2(ConfigurableV2TestConfig{
			Enabled:   true,
			RateLimit: 100,
			Name:      "default",
		}),
	}
}

func (a *ConfigurableV2TestAgent) Name() string {
	return "configurable-v2-test-agent"
}

func TestConfigurableAgentV2_DefaultConfig(t *testing.T) {
	agent := NewConfigurableV2TestAgent()

	config := agent.Config()
	if !config.Enabled {
		t.Error("expected default Enabled to be true")
	}
	if config.RateLimit != 100 {
		t.Errorf("expected default RateLimit 100, got %d", config.RateLimit)
	}
}

func TestConfigurableAgentV2_Capabilities(t *testing.T) {
	agent := NewConfigurableV2TestAgent()

	// Default capabilities
	caps := agent.Capabilities()
	if caps == nil {
		t.Fatal("expected capabilities to be non-nil")
	}
	if !caps.HandlesRequestHeaders {
		t.Error("expected default to handle request headers")
	}

	// Set custom capabilities
	agent.SetCapabilities(NewAgentCapabilities().All())
	caps = agent.Capabilities()
	if !caps.HandlesResponseBody {
		t.Error("expected custom caps to handle response body")
	}
}

func TestConfigurableAgentV2_HealthAndMetrics(t *testing.T) {
	agent := NewConfigurableV2TestAgent()
	ctx := context.Background()

	health := agent.HealthCheck(ctx)
	if !health.IsHealthy() {
		t.Error("expected health to be healthy")
	}

	metrics := agent.Metrics(ctx)
	if metrics == nil {
		t.Error("expected metrics to be non-nil")
	}
}

func TestConfigurableAgentV2_LifecycleHooks(t *testing.T) {
	agent := NewConfigurableV2TestAgent()
	ctx := context.Background()

	// These should not panic
	agent.OnShutdown(ctx)
	agent.OnDrain(ctx)
	agent.OnStreamClosed(ctx, "stream-1")
	agent.OnCancel(ctx, 123)
}
