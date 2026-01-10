package sentinel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
)

// RunnerConfig contains configuration for the agent runner.
type RunnerConfig struct {
	SocketPath string
	Name       string
	JSONLogs   bool
	LogLevel   string
}

// DefaultRunnerConfig returns the default runner configuration.
func DefaultRunnerConfig() RunnerConfig {
	return RunnerConfig{
		SocketPath: "/tmp/sentinel-agent.sock",
		Name:       "agent",
		JSONLogs:   false,
		LogLevel:   "info",
	}
}

// AgentHandler handles protocol events and routes them to the agent.
type AgentHandler struct {
	agent          Agent
	requests       map[string]*Request
	requestBodies  map[string][]byte
	responseBodies map[string][]byte
	responseEvents map[string]*ResponseHeadersEvent
	mu             sync.RWMutex
}

// NewAgentHandler creates a new handler for the given agent.
func NewAgentHandler(agent Agent) *AgentHandler {
	return &AgentHandler{
		agent:          agent,
		requests:       make(map[string]*Request),
		requestBodies:  make(map[string][]byte),
		responseBodies: make(map[string][]byte),
		responseEvents: make(map[string]*ResponseHeadersEvent),
	}
}

// HandleEvent handles an incoming protocol event.
func (h *AgentHandler) HandleEvent(ctx context.Context, event map[string]interface{}) (interface{}, error) {
	eventType, _ := event["event_type"].(string)
	payload, _ := event["payload"].(map[string]interface{})

	switch EventType(eventType) {
	case EventTypeConfigure:
		return h.handleConfigure(ctx, payload)
	case EventTypeRequestHeaders:
		return h.handleRequestHeaders(ctx, payload)
	case EventTypeRequestBodyChunk:
		return h.handleRequestBodyChunk(ctx, payload)
	case EventTypeResponseHeaders:
		return h.handleResponseHeaders(ctx, payload)
	case EventTypeResponseBodyChunk:
		return h.handleResponseBodyChunk(ctx, payload)
	case EventTypeRequestComplete:
		return h.handleRequestComplete(ctx, payload)
	case EventTypeGuardrailInspect:
		return h.handleGuardrailInspect(ctx, payload)
	default:
		log.Warn().Str("event_type", eventType).Msg("Unknown event type")
		return Allow().Build(), nil
	}
}

func (h *AgentHandler) handleConfigure(ctx context.Context, payload map[string]interface{}) (interface{}, error) {
	agentID, _ := payload["agent_id"].(string)
	config, _ := payload["config"].(map[string]interface{})

	if err := h.agent.OnConfigure(ctx, config); err != nil {
		log.Error().Err(err).Msg("Configuration failed")
		return map[string]interface{}{"success": false, "error": err.Error()}, nil
	}

	log.Info().Str("agent_id", agentID).Msg("Agent configured")
	return map[string]interface{}{"success": true}, nil
}

func (h *AgentHandler) handleRequestHeaders(ctx context.Context, payload map[string]interface{}) (interface{}, error) {
	// Parse the event
	jsonBytes, _ := json.Marshal(payload)
	var event RequestHeadersEvent
	if err := json.Unmarshal(jsonBytes, &event); err != nil {
		log.Error().Err(err).Msg("Failed to parse request headers event")
		return Allow().Build(), nil
	}

	request := NewRequest(&event, nil)
	correlationID := event.Metadata.CorrelationID

	// Cache request for response correlation
	h.mu.Lock()
	h.requests[correlationID] = request
	h.requestBodies[correlationID] = []byte{}
	h.mu.Unlock()

	decision := h.agent.OnRequest(ctx, request)
	return decision.Build(), nil
}

func (h *AgentHandler) handleRequestBodyChunk(ctx context.Context, payload map[string]interface{}) (interface{}, error) {
	jsonBytes, _ := json.Marshal(payload)
	var event RequestBodyChunkEvent
	if err := json.Unmarshal(jsonBytes, &event); err != nil {
		log.Error().Err(err).Msg("Failed to parse request body chunk event")
		return Allow().Build(), nil
	}

	correlationID := event.CorrelationID
	data, _ := event.DecodedData()

	// Accumulate body chunks
	h.mu.Lock()
	h.requestBodies[correlationID] = append(h.requestBodies[correlationID], data...)
	body := h.requestBodies[correlationID]
	request := h.requests[correlationID]
	h.mu.Unlock()

	// Only call handler on last chunk
	if event.IsLast && request != nil {
		requestWithBody := request.WithBody(body)
		decision := h.agent.OnRequestBody(ctx, requestWithBody)
		return decision.Build(), nil
	}

	// For non-final chunks, return allow with needs_more
	return Allow().NeedsMoreData().Build(), nil
}

func (h *AgentHandler) handleResponseHeaders(ctx context.Context, payload map[string]interface{}) (interface{}, error) {
	jsonBytes, _ := json.Marshal(payload)
	var event ResponseHeadersEvent
	if err := json.Unmarshal(jsonBytes, &event); err != nil {
		log.Error().Err(err).Msg("Failed to parse response headers event")
		return Allow().Build(), nil
	}

	correlationID := event.CorrelationID

	// Get cached request
	h.mu.RLock()
	request := h.requests[correlationID]
	h.mu.RUnlock()

	if request == nil {
		log.Warn().Str("correlation_id", correlationID).Msg("No cached request for correlation_id")
		return Allow().Build(), nil
	}

	response := NewResponse(&event, nil)

	// Cache response event for body processing
	h.mu.Lock()
	h.responseEvents[correlationID] = &event
	h.responseBodies[correlationID] = []byte{}
	h.mu.Unlock()

	decision := h.agent.OnResponse(ctx, request, response)
	return decision.Build(), nil
}

func (h *AgentHandler) handleResponseBodyChunk(ctx context.Context, payload map[string]interface{}) (interface{}, error) {
	jsonBytes, _ := json.Marshal(payload)
	var event ResponseBodyChunkEvent
	if err := json.Unmarshal(jsonBytes, &event); err != nil {
		log.Error().Err(err).Msg("Failed to parse response body chunk event")
		return Allow().Build(), nil
	}

	correlationID := event.CorrelationID
	data, _ := event.DecodedData()

	// Accumulate body chunks
	h.mu.Lock()
	h.responseBodies[correlationID] = append(h.responseBodies[correlationID], data...)
	body := h.responseBodies[correlationID]
	request := h.requests[correlationID]
	responseEvent := h.responseEvents[correlationID]
	h.mu.Unlock()

	// Only call handler on last chunk
	if event.IsLast && request != nil && responseEvent != nil {
		response := NewResponse(responseEvent, body)
		decision := h.agent.OnResponseBody(ctx, request, response)
		return decision.Build(), nil
	}

	return Allow().NeedsMoreData().Build(), nil
}

func (h *AgentHandler) handleRequestComplete(ctx context.Context, payload map[string]interface{}) (interface{}, error) {
	jsonBytes, _ := json.Marshal(payload)
	var event RequestCompleteEvent
	if err := json.Unmarshal(jsonBytes, &event); err != nil {
		log.Error().Err(err).Msg("Failed to parse request complete event")
		return map[string]interface{}{"success": true}, nil
	}

	correlationID := event.CorrelationID

	// Get and cleanup cached request
	h.mu.Lock()
	request := h.requests[correlationID]
	delete(h.requests, correlationID)
	delete(h.requestBodies, correlationID)
	delete(h.responseBodies, correlationID)
	delete(h.responseEvents, correlationID)
	h.mu.Unlock()

	if request != nil {
		h.agent.OnRequestComplete(ctx, request, event.Status, event.DurationMS)
	}

	return map[string]interface{}{"success": true}, nil
}

func (h *AgentHandler) handleGuardrailInspect(ctx context.Context, payload map[string]interface{}) (interface{}, error) {
	jsonBytes, _ := json.Marshal(payload)
	var event GuardrailInspectEvent
	if err := json.Unmarshal(jsonBytes, &event); err != nil {
		log.Error().Err(err).Msg("Failed to parse guardrail inspect event")
		return NewGuardrailResponse(), nil
	}

	response := h.agent.OnGuardrailInspect(ctx, &event)
	return response, nil
}

// AgentRunner runs an agent server.
type AgentRunner struct {
	agent    Agent
	config   RunnerConfig
	listener net.Listener
	shutdown chan struct{}
}

// NewAgentRunner creates a new runner for the given agent.
func NewAgentRunner(agent Agent) *AgentRunner {
	config := DefaultRunnerConfig()
	config.Name = agent.Name()
	return &AgentRunner{
		agent:    agent,
		config:   config,
		shutdown: make(chan struct{}),
	}
}

// WithName sets the agent name for logging.
func (r *AgentRunner) WithName(name string) *AgentRunner {
	r.config.Name = name
	return r
}

// WithSocket sets the Unix socket path.
func (r *AgentRunner) WithSocket(path string) *AgentRunner {
	r.config.SocketPath = path
	return r
}

// WithJSONLogs enables JSON log format.
func (r *AgentRunner) WithJSONLogs() *AgentRunner {
	r.config.JSONLogs = true
	return r
}

// WithLogLevel sets the log level.
func (r *AgentRunner) WithLogLevel(level string) *AgentRunner {
	r.config.LogLevel = level
	return r
}

// WithConfig sets the full runner configuration.
func (r *AgentRunner) WithConfig(config RunnerConfig) *AgentRunner {
	r.config = config
	return r
}

func (r *AgentRunner) setupLogging() {
	// Parse log level
	level, err := zerolog.ParseLevel(r.config.LogLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	if r.config.JSONLogs {
		log.Logger = zerolog.New(os.Stdout).With().
			Timestamp().
			Str("agent", r.config.Name).
			Logger()
	} else {
		log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout}).With().
			Timestamp().
			Str("agent", r.config.Name).
			Logger()
	}
}

func (r *AgentRunner) handleConnection(conn net.Conn) {
	defer conn.Close()

	handler := NewAgentHandler(r.agent)
	ctx := context.Background()

	for {
		select {
		case <-r.shutdown:
			return
		default:
		}

		msg, err := ReadMessage(conn)
		if err != nil {
			if err != io.EOF {
				log.Error().Err(err).Msg("Failed to read message")
			}
			return
		}
		if msg == nil {
			return
		}

		response, err := handler.HandleEvent(ctx, msg)
		if err != nil {
			log.Error().Err(err).Msg("Failed to handle event")
			response = Allow().Build()
		}

		if err := WriteMessage(conn, response); err != nil {
			log.Error().Err(err).Msg("Failed to write response")
			return
		}
	}
}

// Run starts the agent server.
func (r *AgentRunner) Run() error {
	r.setupLogging()

	// Clean up existing socket
	if _, err := os.Stat(r.config.SocketPath); err == nil {
		if err := os.Remove(r.config.SocketPath); err != nil {
			return fmt.Errorf("failed to remove existing socket: %w", err)
		}
	}

	// Create listener
	listener, err := net.Listen("unix", r.config.SocketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on socket: %w", err)
	}
	r.listener = listener

	// Set socket permissions
	if err := os.Chmod(r.config.SocketPath, 0660); err != nil {
		log.Warn().Err(err).Msg("Failed to set socket permissions")
	}

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Info().Msg("Shutdown signal received")
		close(r.shutdown)
		r.listener.Close()
	}()

	log.Info().
		Str("socket", r.config.SocketPath).
		Str("name", r.config.Name).
		Msg("Agent listening")

	// Accept connections
	for {
		conn, err := r.listener.Accept()
		if err != nil {
			select {
			case <-r.shutdown:
				break
			default:
				log.Error().Err(err).Msg("Failed to accept connection")
				continue
			}
			break
		}

		go r.handleConnection(conn)
	}

	// Cleanup
	os.Remove(r.config.SocketPath)
	log.Info().Msg("Agent shutdown complete")

	return nil
}

// ParseArgs parses command line arguments and returns a RunnerConfig.
func ParseArgs() RunnerConfig {
	config := DefaultRunnerConfig()

	pflag.StringVar(&config.SocketPath, "socket", config.SocketPath, "Unix socket path")
	pflag.BoolVar(&config.JSONLogs, "json-logs", config.JSONLogs, "Enable JSON log format")
	pflag.StringVar(&config.LogLevel, "log-level", config.LogLevel, "Log level (debug, info, warn, error)")
	pflag.Parse()

	return config
}

// RunAgent is a convenience function to run an agent from main.
// It parses command line arguments and runs the agent.
//
// Example:
//
//	func main() {
//	    sentinel.RunAgent(&MyAgent{})
//	}
func RunAgent(agent Agent) {
	config := ParseArgs()
	config.Name = agent.Name()

	runner := NewAgentRunner(agent).WithConfig(config)

	if err := runner.Run(); err != nil {
		log.Fatal().Err(err).Msg("Agent failed")
	}
}
