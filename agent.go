package sentinel

import (
	"context"
	"encoding/json"
	"sync"
)

// Agent is the interface for Sentinel agents.
//
// Implement this interface to create a custom agent that can process
// HTTP requests and responses in the Sentinel proxy pipeline.
//
// Example:
//
//	type MyAgent struct{}
//
//	func (a *MyAgent) Name() string { return "my-agent" }
//
//	func (a *MyAgent) OnRequest(ctx context.Context, req *Request) *Decision {
//	    if req.PathStartsWith("/blocked") {
//	        return Deny().WithBody("Blocked")
//	    }
//	    return Allow()
//	}
type Agent interface {
	// Name returns the agent name for logging.
	Name() string

	// OnConfigure handles configuration from the proxy.
	// Called once when the agent connects to the proxy.
	// Return an error to reject the configuration and prevent proxy startup.
	OnConfigure(ctx context.Context, config map[string]interface{}) error

	// OnRequest processes incoming request headers.
	// Called when request headers are received from the client.
	OnRequest(ctx context.Context, request *Request) *Decision

	// OnRequestBody processes request body.
	// Called when request body is available (if body inspection enabled).
	OnRequestBody(ctx context.Context, request *Request) *Decision

	// OnResponse processes response headers from upstream.
	// Called when response headers are received from the upstream server.
	OnResponse(ctx context.Context, request *Request, response *Response) *Decision

	// OnResponseBody processes response body.
	// Called when response body is available (if body inspection enabled).
	OnResponseBody(ctx context.Context, request *Request, response *Response) *Decision

	// OnRequestComplete is called when request processing is complete.
	// Override for logging, metrics, or cleanup.
	OnRequestComplete(ctx context.Context, request *Request, status int, durationMS int)
}

// BaseAgent provides default implementations for all Agent methods.
// Embed this in your agent struct to only implement the methods you need.
//
// Example:
//
//	type MyAgent struct {
//	    sentinel.BaseAgent
//	}
//
//	func (a *MyAgent) Name() string { return "my-agent" }
//
//	func (a *MyAgent) OnRequest(ctx context.Context, req *Request) *Decision {
//	    // Your custom logic here
//	    return sentinel.Allow()
//	}
type BaseAgent struct{}

// Name returns a default agent name. Override this in your agent.
func (a *BaseAgent) Name() string {
	return "agent"
}

// OnConfigure provides a default no-op configuration handler.
func (a *BaseAgent) OnConfigure(ctx context.Context, config map[string]interface{}) error {
	return nil
}

// OnRequest provides a default allow decision.
func (a *BaseAgent) OnRequest(ctx context.Context, request *Request) *Decision {
	return Allow()
}

// OnRequestBody provides a default allow decision.
func (a *BaseAgent) OnRequestBody(ctx context.Context, request *Request) *Decision {
	return Allow()
}

// OnResponse provides a default allow decision.
func (a *BaseAgent) OnResponse(ctx context.Context, request *Request, response *Response) *Decision {
	return Allow()
}

// OnResponseBody provides a default allow decision.
func (a *BaseAgent) OnResponseBody(ctx context.Context, request *Request, response *Response) *Decision {
	return Allow()
}

// OnRequestComplete provides a default no-op handler.
func (a *BaseAgent) OnRequestComplete(ctx context.Context, request *Request, status int, durationMS int) {
}

// ConfigurableAgent is an agent with typed configuration support.
//
// Example:
//
//	type MyConfig struct {
//	    RateLimit int  `json:"rate_limit"`
//	    Enabled   bool `json:"enabled"`
//	}
//
//	type MyAgent struct {
//	    *sentinel.ConfigurableAgentBase[MyConfig]
//	}
//
//	func NewMyAgent() *MyAgent {
//	    return &MyAgent{
//	        ConfigurableAgentBase: sentinel.NewConfigurableAgent(MyConfig{
//	            RateLimit: 100,
//	            Enabled:   true,
//	        }),
//	    }
//	}
//
//	func (a *MyAgent) Name() string { return "my-agent" }
//
//	func (a *MyAgent) OnRequest(ctx context.Context, req *Request) *Decision {
//	    cfg := a.Config()
//	    if !cfg.Enabled {
//	        return sentinel.Allow()
//	    }
//	    // Use cfg.RateLimit...
//	    return sentinel.Allow()
//	}
type ConfigurableAgent[T any] interface {
	Agent

	// Config returns the current configuration.
	Config() T

	// SetConfig updates the configuration.
	SetConfig(config T)

	// ParseConfig parses a configuration map into the typed config.
	ParseConfig(configMap map[string]interface{}) (T, error)

	// OnConfigApplied is called after configuration is applied.
	OnConfigApplied(ctx context.Context, config T)
}

// ConfigurableAgentBase provides a base implementation for ConfigurableAgent.
type ConfigurableAgentBase[T any] struct {
	BaseAgent
	config T
	mu     sync.RWMutex
}

// NewConfigurableAgent creates a new ConfigurableAgentBase with default config.
func NewConfigurableAgent[T any](defaultConfig T) *ConfigurableAgentBase[T] {
	return &ConfigurableAgentBase[T]{
		config: defaultConfig,
	}
}

// Config returns the current configuration (thread-safe).
func (a *ConfigurableAgentBase[T]) Config() T {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.config
}

// SetConfig updates the configuration (thread-safe).
func (a *ConfigurableAgentBase[T]) SetConfig(config T) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.config = config
}

// ParseConfig parses a configuration map into the typed config using JSON.
func (a *ConfigurableAgentBase[T]) ParseConfig(configMap map[string]interface{}) (T, error) {
	var config T
	// Convert map to JSON then back to struct
	jsonBytes, err := json.Marshal(configMap)
	if err != nil {
		return config, err
	}
	err = json.Unmarshal(jsonBytes, &config)
	return config, err
}

// OnConfigApplied is called after configuration is applied.
// Override this in your agent for post-configuration setup.
func (a *ConfigurableAgentBase[T]) OnConfigApplied(ctx context.Context, config T) {
}

// OnConfigure handles configuration from the proxy.
// It parses the config, stores it, and calls OnConfigApplied.
func (a *ConfigurableAgentBase[T]) OnConfigure(ctx context.Context, configMap map[string]interface{}) error {
	config, err := a.ParseConfig(configMap)
	if err != nil {
		return err
	}
	a.SetConfig(config)
	a.OnConfigApplied(ctx, config)
	return nil
}
