package v2

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
)

// TransportType specifies the transport mechanism.
type TransportType string

const (
	// TransportUDS uses Unix Domain Sockets (recommended for co-located agents).
	TransportUDS TransportType = "uds"

	// TransportGRPC uses gRPC over HTTP/2 (for remote agents).
	TransportGRPC TransportType = "grpc"

	// TransportReverse uses reverse connections (agent connects to proxy).
	TransportReverse TransportType = "reverse"
)

// RunnerConfigV2 contains configuration for the v2 agent runner.
type RunnerConfigV2 struct {
	// Name is the agent name for logging.
	Name string

	// Transport specifies the transport type.
	Transport TransportType

	// SocketPath is the Unix socket path (for UDS transport).
	SocketPath string

	// GRPCAddress is the gRPC server address (for gRPC transport).
	GRPCAddress string

	// ReverseAddress is the proxy address to connect to (for reverse transport).
	ReverseAddress string

	// TLSConfig for gRPC transport.
	TLSConfig *tls.Config

	// JSONLogs enables JSON log format.
	JSONLogs bool

	// LogLevel sets the log level.
	LogLevel string

	// ShutdownTimeout is the maximum time to wait for graceful shutdown.
	ShutdownTimeout time.Duration

	// DrainTimeout is the maximum time to wait for requests to drain.
	DrainTimeout time.Duration

	// HealthCheckInterval is how often to run health checks.
	HealthCheckInterval time.Duration

	// ReverseReconnectInterval is how often to attempt reconnection (for reverse transport).
	ReverseReconnectInterval time.Duration

	// AuthToken for reverse connection authentication.
	AuthToken string
}

// DefaultRunnerConfigV2 returns the default v2 runner configuration.
func DefaultRunnerConfigV2() RunnerConfigV2 {
	return RunnerConfigV2{
		Name:                     "agent",
		Transport:                TransportUDS,
		SocketPath:               "/tmp/sentinel-agent.sock",
		GRPCAddress:              "localhost:50051",
		ReverseAddress:           "",
		TLSConfig:                nil,
		JSONLogs:                 false,
		LogLevel:                 "info",
		ShutdownTimeout:          30 * time.Second,
		DrainTimeout:             10 * time.Second,
		HealthCheckInterval:      10 * time.Second,
		ReverseReconnectInterval: 5 * time.Second,
		AuthToken:                "",
	}
}

// AgentRunnerV2 runs an agent server with v2 protocol support.
type AgentRunnerV2 struct {
	agent    AgentV2
	config   RunnerConfigV2
	handler  *AgentHandlerV2
	listener net.Listener
	shutdown chan struct{}
	draining bool
	mu       sync.RWMutex
	wg       sync.WaitGroup
}

// NewAgentRunnerV2 creates a new v2 runner for the given agent.
func NewAgentRunnerV2(agent AgentV2) *AgentRunnerV2 {
	config := DefaultRunnerConfigV2()
	config.Name = agent.Name()

	return &AgentRunnerV2{
		agent:    agent,
		config:   config,
		handler:  NewAgentHandlerV2(agent),
		shutdown: make(chan struct{}),
	}
}

// WithName sets the agent name for logging.
func (r *AgentRunnerV2) WithName(name string) *AgentRunnerV2 {
	r.config.Name = name
	return r
}

// WithSocket configures UDS transport with the given socket path.
func (r *AgentRunnerV2) WithSocket(path string) *AgentRunnerV2 {
	r.config.Transport = TransportUDS
	r.config.SocketPath = path
	return r
}

// WithGRPC configures gRPC transport with the given address.
func (r *AgentRunnerV2) WithGRPC(address string) *AgentRunnerV2 {
	r.config.Transport = TransportGRPC
	r.config.GRPCAddress = address
	return r
}

// WithGRPCTLS configures gRPC transport with TLS.
func (r *AgentRunnerV2) WithGRPCTLS(address string, tlsConfig *tls.Config) *AgentRunnerV2 {
	r.config.Transport = TransportGRPC
	r.config.GRPCAddress = address
	r.config.TLSConfig = tlsConfig
	return r
}

// WithReverse configures reverse connection transport.
func (r *AgentRunnerV2) WithReverse(proxyAddress string) *AgentRunnerV2 {
	r.config.Transport = TransportReverse
	r.config.ReverseAddress = proxyAddress
	return r
}

// WithReverseAuth configures reverse connection with authentication.
func (r *AgentRunnerV2) WithReverseAuth(proxyAddress, authToken string) *AgentRunnerV2 {
	r.config.Transport = TransportReverse
	r.config.ReverseAddress = proxyAddress
	r.config.AuthToken = authToken
	return r
}

// WithJSONLogs enables JSON log format.
func (r *AgentRunnerV2) WithJSONLogs() *AgentRunnerV2 {
	r.config.JSONLogs = true
	return r
}

// WithLogLevel sets the log level.
func (r *AgentRunnerV2) WithLogLevel(level string) *AgentRunnerV2 {
	r.config.LogLevel = level
	return r
}

// WithShutdownTimeout sets the shutdown timeout.
func (r *AgentRunnerV2) WithShutdownTimeout(timeout time.Duration) *AgentRunnerV2 {
	r.config.ShutdownTimeout = timeout
	return r
}

// WithDrainTimeout sets the drain timeout.
func (r *AgentRunnerV2) WithDrainTimeout(timeout time.Duration) *AgentRunnerV2 {
	r.config.DrainTimeout = timeout
	return r
}

// WithConfig sets the full runner configuration.
func (r *AgentRunnerV2) WithConfig(config RunnerConfigV2) *AgentRunnerV2 {
	r.config = config
	return r
}

func (r *AgentRunnerV2) setupLogging() {
	level, err := zerolog.ParseLevel(r.config.LogLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	if r.config.JSONLogs {
		log.Logger = zerolog.New(os.Stdout).With().
			Timestamp().
			Str("agent", r.config.Name).
			Str("protocol", "v2").
			Logger()
	} else {
		log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout}).With().
			Timestamp().
			Str("agent", r.config.Name).
			Logger()
	}
}

// Run starts the agent server.
func (r *AgentRunnerV2) Run() error {
	r.setupLogging()

	log.Info().
		Str("transport", string(r.config.Transport)).
		Str("name", r.config.Name).
		Msg("Starting agent with v2 protocol")

	switch r.config.Transport {
	case TransportUDS:
		return r.runUDS()
	case TransportGRPC:
		return r.runGRPC()
	case TransportReverse:
		return r.runReverse()
	default:
		return fmt.Errorf("unsupported transport: %s", r.config.Transport)
	}
}

func (r *AgentRunnerV2) runUDS() error {
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
	r.setupSignalHandling()

	log.Info().Str("socket", r.config.SocketPath).Msg("Agent listening (UDS)")

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

		r.wg.Add(1)
		go r.handleUDSConnection(conn)
	}

	// Wait for connections to drain
	r.waitForDrain()

	// Cleanup
	os.Remove(r.config.SocketPath)
	log.Info().Msg("Agent shutdown complete")

	return nil
}

func (r *AgentRunnerV2) handleUDSConnection(conn net.Conn) {
	defer r.wg.Done()
	defer conn.Close()

	streamID := fmt.Sprintf("uds-%s", conn.RemoteAddr().String())
	ctx := context.Background()

	// Perform handshake
	if err := r.performHandshake(conn); err != nil {
		log.Error().Err(err).Msg("Handshake failed")
		return
	}

	log.Debug().Str("stream_id", streamID).Msg("Connection established")

	for {
		select {
		case <-r.shutdown:
			r.agent.OnStreamClosed(ctx, streamID)
			return
		default:
		}

		// Check if draining
		r.mu.RLock()
		draining := r.draining
		r.mu.RUnlock()
		if draining {
			r.agent.OnStreamClosed(ctx, streamID)
			return
		}

		msg, err := ReadMessageV2(conn)
		if err != nil {
			if err != io.EOF {
				log.Error().Err(err).Msg("Failed to read message")
			}
			r.agent.OnStreamClosed(ctx, streamID)
			return
		}
		if msg == nil {
			r.agent.OnStreamClosed(ctx, streamID)
			return
		}

		response, err := r.handler.HandleMessage(ctx, msg)
		if err != nil {
			log.Error().Err(err).Msg("Failed to handle message")
			continue
		}

		// Some messages (like cancel) don't have responses
		if response == nil {
			continue
		}

		if err := WriteMessageV2(conn, response); err != nil {
			log.Error().Err(err).Msg("Failed to write response")
			r.agent.OnStreamClosed(ctx, streamID)
			return
		}
	}
}

func (r *AgentRunnerV2) performHandshake(conn net.Conn) error {
	// Read handshake request
	msg, err := ReadMessageV2(conn)
	if err != nil {
		return fmt.Errorf("failed to read handshake: %w", err)
	}
	if msg == nil {
		return fmt.Errorf("connection closed during handshake")
	}

	if msg.Type != MsgTypeHandshakeRequest {
		return fmt.Errorf("expected handshake request, got %s", msg.TypeName())
	}

	// Handle handshake
	response, err := r.handler.HandleMessage(context.Background(), msg)
	if err != nil {
		return fmt.Errorf("handshake handling failed: %w", err)
	}

	// Send response
	if err := WriteMessageV2(conn, response); err != nil {
		return fmt.Errorf("failed to send handshake response: %w", err)
	}

	return nil
}

func (r *AgentRunnerV2) runGRPC() error {
	// Note: Full gRPC implementation would require protobuf definitions
	// and generated code. This is a placeholder that shows the structure.
	log.Error().Msg("gRPC transport requires protobuf support - use UDS for now")
	return fmt.Errorf("gRPC transport not yet implemented - use UDS transport")
}

func (r *AgentRunnerV2) runReverse() error {
	if r.config.ReverseAddress == "" {
		return fmt.Errorf("reverse address not configured")
	}

	r.setupSignalHandling()

	log.Info().Str("address", r.config.ReverseAddress).Msg("Connecting to proxy (reverse)")

	for {
		select {
		case <-r.shutdown:
			log.Info().Msg("Agent shutdown complete")
			return nil
		default:
		}

		// Connect to proxy
		conn, err := r.connectReverse()
		if err != nil {
			log.Error().Err(err).Msg("Failed to connect to proxy")
			time.Sleep(r.config.ReverseReconnectInterval)
			continue
		}

		// Handle connection
		r.wg.Add(1)
		r.handleReverseConnection(conn)
		r.wg.Done()

		// Reconnect after disconnection
		select {
		case <-r.shutdown:
			log.Info().Msg("Agent shutdown complete")
			return nil
		default:
			log.Info().Msg("Connection lost, reconnecting...")
			time.Sleep(r.config.ReverseReconnectInterval)
		}
	}
}

func (r *AgentRunnerV2) connectReverse() (net.Conn, error) {
	var conn net.Conn
	var err error

	// Determine if UDS or TCP
	if r.config.ReverseAddress[0] == '/' {
		conn, err = net.Dial("unix", r.config.ReverseAddress)
	} else {
		conn, err = net.Dial("tcp", r.config.ReverseAddress)
	}
	if err != nil {
		return nil, err
	}

	// Send registration
	reg := NewRegistrationRequest(r.config.Name, r.agent.Capabilities())
	if r.config.AuthToken != "" {
		reg.WithAuthToken(r.config.AuthToken)
	}

	regMsg, err := NewV2Message(MsgTypeRegistration, reg)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create registration message: %w", err)
	}

	if err := WriteMessageV2(conn, regMsg); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to send registration: %w", err)
	}

	// Read registration response
	respMsg, err := ReadMessageV2(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to read registration response: %w", err)
	}

	if respMsg.Type != MsgTypeRegistrationAck {
		conn.Close()
		return nil, fmt.Errorf("expected registration ack, got %s", respMsg.TypeName())
	}

	var resp RegistrationResponse
	if err := respMsg.ParsePayload(&resp); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to parse registration response: %w", err)
	}

	if !resp.Accepted {
		conn.Close()
		return nil, fmt.Errorf("registration rejected: %s", resp.Error)
	}

	log.Info().Str("assigned_id", resp.AssignedID).Msg("Registered with proxy")

	return conn, nil
}

func (r *AgentRunnerV2) handleReverseConnection(conn net.Conn) {
	defer conn.Close()

	streamID := fmt.Sprintf("reverse-%s", conn.RemoteAddr().String())
	ctx := context.Background()

	log.Debug().Str("stream_id", streamID).Msg("Reverse connection established")

	for {
		select {
		case <-r.shutdown:
			r.agent.OnStreamClosed(ctx, streamID)
			return
		default:
		}

		msg, err := ReadMessageV2(conn)
		if err != nil {
			if err != io.EOF {
				log.Error().Err(err).Msg("Failed to read message")
			}
			r.agent.OnStreamClosed(ctx, streamID)
			return
		}
		if msg == nil {
			r.agent.OnStreamClosed(ctx, streamID)
			return
		}

		response, err := r.handler.HandleMessage(ctx, msg)
		if err != nil {
			log.Error().Err(err).Msg("Failed to handle message")
			continue
		}

		if response == nil {
			continue
		}

		if err := WriteMessageV2(conn, response); err != nil {
			log.Error().Err(err).Msg("Failed to write response")
			r.agent.OnStreamClosed(ctx, streamID)
			return
		}
	}
}

func (r *AgentRunnerV2) setupSignalHandling() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Info().Str("signal", sig.String()).Msg("Shutdown signal received")

		// Start drain
		r.mu.Lock()
		r.draining = true
		r.mu.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), r.config.DrainTimeout)
		defer cancel()
		r.agent.OnDrain(ctx)

		// Close listener to stop accepting new connections
		close(r.shutdown)
		if r.listener != nil {
			r.listener.Close()
		}
	}()
}

func (r *AgentRunnerV2) waitForDrain() {
	// Wait for drain timeout or all connections to close
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Info().Msg("All connections drained")
	case <-time.After(r.config.DrainTimeout):
		log.Warn().Msg("Drain timeout reached, forcing shutdown")
	}

	// Call shutdown hook
	ctx, cancel := context.WithTimeout(context.Background(), r.config.ShutdownTimeout)
	defer cancel()
	r.agent.OnShutdown(ctx)
}

// ParseArgsV2 parses command line arguments and returns a v2 RunnerConfig.
func ParseArgsV2() RunnerConfigV2 {
	config := DefaultRunnerConfigV2()

	pflag.StringVar(&config.SocketPath, "socket", config.SocketPath, "Unix socket path (for UDS transport)")
	pflag.StringVar(&config.GRPCAddress, "grpc", "", "gRPC server address (enables gRPC transport)")
	pflag.StringVar(&config.ReverseAddress, "reverse", "", "Proxy address for reverse connection")
	pflag.BoolVar(&config.JSONLogs, "json-logs", config.JSONLogs, "Enable JSON log format")
	pflag.StringVar(&config.LogLevel, "log-level", config.LogLevel, "Log level (debug, info, warn, error)")
	pflag.DurationVar(&config.ShutdownTimeout, "shutdown-timeout", config.ShutdownTimeout, "Shutdown timeout")
	pflag.DurationVar(&config.DrainTimeout, "drain-timeout", config.DrainTimeout, "Drain timeout")
	pflag.StringVar(&config.AuthToken, "auth-token", "", "Authentication token for reverse connections")
	pflag.Parse()

	// Determine transport based on flags
	if config.ReverseAddress != "" {
		config.Transport = TransportReverse
	} else if config.GRPCAddress != "" {
		config.Transport = TransportGRPC
	} else {
		config.Transport = TransportUDS
	}

	return config
}

// RunAgentV2 is a convenience function to run an agent from main.
// It parses command line arguments and runs the agent with v2 protocol.
//
// Example:
//
//	func main() {
//	    v2.RunAgentV2(&MyAgent{})
//	}
func RunAgentV2(agent AgentV2) {
	config := ParseArgsV2()
	config.Name = agent.Name()

	runner := NewAgentRunnerV2(agent).WithConfig(config)

	if err := runner.Run(); err != nil {
		log.Fatal().Err(err).Msg("Agent failed")
	}
}
