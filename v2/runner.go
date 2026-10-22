package v2

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	zentinel "github.com/zentinelproxy/zentinel-agent-go-sdk"
)

// TransportType specifies the transport mechanism.
type TransportType string

const (
	// TransportUDS uses a Unix domain socket (recommended for co-located agents).
	TransportUDS TransportType = "uds"

	// TransportGRPC uses gRPC over HTTP/2 (for remote agents).
	TransportGRPC TransportType = "grpc"
)

// RunnerConfigV2 contains configuration for the v2 agent runner.
type RunnerConfigV2 struct {
	// Name is the agent name for logging.
	Name string

	// Transport specifies the transport type.
	Transport TransportType

	// SocketPath is the Unix socket path (for UDS transport).
	SocketPath string

	// GRPCAddress is the gRPC listen address, host:port (for gRPC transport).
	GRPCAddress string

	// TLSConfig for gRPC transport.
	TLSConfig *tls.Config

	// JSONLogs enables JSON log format.
	JSONLogs bool

	// LogLevel sets the log level.
	LogLevel string

	// ShutdownTimeout is the maximum time OnShutdown may take.
	ShutdownTimeout time.Duration

	// DrainTimeout is the maximum time to wait for in-flight requests on
	// shutdown.
	DrainTimeout time.Duration
}

// DefaultRunnerConfigV2 returns the default v2 runner configuration.
func DefaultRunnerConfigV2() RunnerConfigV2 {
	return RunnerConfigV2{
		Name:            "agent",
		Transport:       TransportUDS,
		SocketPath:      "/tmp/zentinel-agent.sock",
		GRPCAddress:     "127.0.0.1:50051",
		LogLevel:        "info",
		ShutdownTimeout: 30 * time.Second,
		DrainTimeout:    5 * time.Second,
	}
}

// AgentRunnerV2 runs an agent server speaking agent protocol v2.
type AgentRunnerV2 struct {
	agent    AgentV2
	config   RunnerConfigV2
	handler  *AgentHandlerV2
	shutdown chan struct{}
	stopOnce sync.Once

	mu    sync.Mutex
	conns map[net.Conn]struct{}
	wg    sync.WaitGroup
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
		conns:    make(map[net.Conn]struct{}),
	}
}

// WithName sets the agent name for logging.
func (r *AgentRunnerV2) WithName(name string) *AgentRunnerV2 {
	r.config.Name = name
	return r
}

// WithSocket configures the Unix socket transport with the given path.
func (r *AgentRunnerV2) WithSocket(path string) *AgentRunnerV2 {
	r.config.Transport = TransportUDS
	r.config.SocketPath = path
	return r
}

// WithGRPC configures the gRPC transport on host:port.
func (r *AgentRunnerV2) WithGRPC(address string) *AgentRunnerV2 {
	r.config.Transport = TransportGRPC
	r.config.GRPCAddress = address
	return r
}

// WithGRPCTLS configures the gRPC transport with TLS.
func (r *AgentRunnerV2) WithGRPCTLS(address string, tlsConfig *tls.Config) *AgentRunnerV2 {
	r.config.Transport = TransportGRPC
	r.config.GRPCAddress = address
	r.config.TLSConfig = tlsConfig
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

// Handler returns the handler that routes protocol events to the agent.
func (r *AgentRunnerV2) Handler() *AgentHandlerV2 { return r.handler }

// Stop shuts the runner down, as SIGINT or SIGTERM do.
func (r *AgentRunnerV2) Stop() {
	r.stopOnce.Do(func() { close(r.shutdown) })
}

func (r *AgentRunnerV2) setupLogging() {
	level, err := zerolog.ParseLevel(r.config.LogLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	if r.config.JSONLogs {
		log.Logger = zerolog.New(os.Stdout).With().Timestamp().
			Str("agent", r.config.Name).Str("protocol", "v2").Logger()
	} else {
		log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout}).With().Timestamp().
			Str("agent", r.config.Name).Logger()
	}
}

// Run starts the agent server and blocks until it is stopped.
func (r *AgentRunnerV2) Run() error {
	r.setupLogging()
	log.Info().Str("transport", string(r.config.Transport)).Str("name", r.config.Name).
		Msg("Starting agent (protocol v2)")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)
	go func() {
		select {
		case sig := <-sigChan:
			log.Info().Str("signal", sig.String()).Msg("Shutdown signal received")
			r.Stop()
		case <-r.shutdown:
		}
	}()

	var err error
	switch r.config.Transport {
	case TransportUDS:
		err = r.runUDS()
	case TransportGRPC:
		err = r.runGRPC()
	default:
		err = fmt.Errorf("unsupported transport: %s", r.config.Transport)
	}
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), r.config.ShutdownTimeout)
	defer cancel()
	r.agent.OnShutdown(ctx)
	log.Info().Msg("Agent shutdown complete")
	return nil
}

func (r *AgentRunnerV2) runUDS() error {
	if _, err := os.Stat(r.config.SocketPath); err == nil {
		if err := os.Remove(r.config.SocketPath); err != nil {
			return fmt.Errorf("failed to remove existing socket: %w", err)
		}
	}
	listener, err := net.Listen("unix", r.config.SocketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on socket: %w", err)
	}
	defer os.Remove(r.config.SocketPath)
	if err := os.Chmod(r.config.SocketPath, 0o660); err != nil {
		log.Warn().Err(err).Msg("Failed to set socket permissions")
	}
	log.Info().Str("socket", r.config.SocketPath).Msg("Agent listening (UDS)")

	go func() {
		<-r.shutdown
		listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-r.shutdown:
				r.drain()
				return nil
			default:
			}
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			log.Error().Err(err).Msg("Failed to accept connection")
			continue
		}
		r.mu.Lock()
		r.conns[conn] = struct{}{}
		r.mu.Unlock()
		r.wg.Add(1)
		go func() {
			defer r.wg.Done()
			defer func() {
				r.mu.Lock()
				delete(r.conns, conn)
				r.mu.Unlock()
				conn.Close()
			}()
			streamID := fmt.Sprintf("uds-%p", conn)
			if err := ServeUDSConn(context.Background(), conn, r.handler); err != nil {
				select {
				case <-r.shutdown:
				default:
					log.Warn().Err(err).Str("stream_id", streamID).Msg("Connection ended")
				}
			}
			r.agent.OnStreamClosed(context.Background(), streamID)
		}()
	}
}

// drain tells the agent, waits for connections to finish for up to
// DrainTimeout, then closes the rest.
func (r *AgentRunnerV2) drain() {
	ctx, cancel := context.WithTimeout(context.Background(), r.config.DrainTimeout)
	defer cancel()
	r.agent.OnDrain(ctx)

	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()
	// The proxy keeps pooled connections open indefinitely, so idle ones are
	// closed rather than waited for.
	r.mu.Lock()
	for conn := range r.conns {
		if uc, ok := conn.(*net.UnixConn); ok {
			_ = uc.CloseRead()
		} else {
			_ = conn.Close()
		}
	}
	r.mu.Unlock()
	select {
	case <-done:
	case <-time.After(r.config.DrainTimeout):
		log.Warn().Msg("Drain timeout reached, closing connections")
		r.mu.Lock()
		for conn := range r.conns {
			conn.Close()
		}
		r.mu.Unlock()
	}
}

func (r *AgentRunnerV2) runGRPC() error {
	lis, err := net.Listen("tcp", r.config.GRPCAddress)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", r.config.GRPCAddress, err)
	}

	var opts []grpc.ServerOption
	if r.config.TLSConfig != nil {
		opts = append(opts, grpc.Creds(credentials.NewTLS(r.config.TLSConfig)))
	}
	server := grpc.NewServer(opts...)
	RegisterGRPCService(server, r.handler)
	log.Info().Str("address", r.config.GRPCAddress).Msg("Agent listening (gRPC)")

	go func() {
		<-r.shutdown
		ctx, cancel := context.WithTimeout(context.Background(), r.config.DrainTimeout)
		defer cancel()
		r.agent.OnDrain(ctx)
		stopped := make(chan struct{})
		go func() {
			server.GracefulStop()
			close(stopped)
		}()
		select {
		case <-stopped:
		case <-ctx.Done():
			server.Stop()
		}
	}()

	if err := server.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		return fmt.Errorf("gRPC server error: %w", err)
	}
	return nil
}

// ParseArgsV2 parses command line arguments and returns a v2 RunnerConfig.
func ParseArgsV2() RunnerConfigV2 {
	config := DefaultRunnerConfigV2()
	grpcAddress := ""

	pflag.StringVar(&config.SocketPath, "socket", config.SocketPath, "Unix socket path (UDS transport)")
	pflag.StringVar(&grpcAddress, "grpc", "", "gRPC listen address, host:port (selects the gRPC transport)")
	pflag.BoolVar(&config.JSONLogs, "json-logs", config.JSONLogs, "Enable JSON log format")
	pflag.StringVar(&config.LogLevel, "log-level", config.LogLevel, "Log level (debug, info, warn, error)")
	pflag.DurationVar(&config.ShutdownTimeout, "shutdown-timeout", config.ShutdownTimeout, "Shutdown timeout")
	pflag.DurationVar(&config.DrainTimeout, "drain-timeout", config.DrainTimeout, "Drain timeout")
	pflag.Parse()

	if grpcAddress != "" {
		config.Transport = TransportGRPC
		config.GRPCAddress = grpcAddress
	}
	return config
}

// RunAgentV2 runs an agent from main: it parses --socket / --grpc and the
// logging flags, and exits the process on error.
//
//	func main() {
//	    v2.RunAgentV2(&MyAgent{})
//	}
func RunAgentV2(agent AgentV2) {
	config := ParseArgsV2()
	config.Name = agent.Name()
	if err := NewAgentRunnerV2(agent).WithConfig(config).Run(); err != nil {
		log.Fatal().Err(err).Msg("Agent failed")
	}
}

// RunAgent runs an agent written against the root package's Agent interface
// (with v2 defaults for capabilities, health and lifecycle hooks), like
// RunAgentV2.
func RunAgent(agent zentinel.Agent) {
	RunAgentV2(Upgrade(agent))
}
