package v2

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	grpcencoding "google.golang.org/grpc/encoding"
	"google.golang.org/grpc/status"
)

// agentGRPCService implements the AgentServiceV2 gRPC service using JSON marshaling.
// This allows the service to work without protoc-generated code by using a manual
// grpc.ServiceDesc with a JSON codec.
type agentGRPCService struct {
	runner   *AgentRunnerV2
	streamID atomic.Uint64
}

// jsonMessage is a raw JSON container used as the gRPC message type.
// It implements the gRPC codec interface by holding raw JSON bytes.
type jsonMessage struct {
	Data json.RawMessage
}

// jsonCodec implements grpc encoding.Codec for JSON marshaling.
// This is registered as the "json" codec and used by the gRPC server
// to marshal/unmarshal messages as JSON instead of protobuf.
type jsonCodec struct{}

// Ensure jsonCodec satisfies the grpc encoding.Codec interface.
var _ grpcencoding.Codec = jsonCodec{}

func (jsonCodec) Marshal(v interface{}) ([]byte, error) {
	msg, ok := v.(*jsonMessage)
	if !ok {
		return json.Marshal(v)
	}
	if msg.Data == nil {
		return []byte("{}"), nil
	}
	return []byte(msg.Data), nil
}

func (jsonCodec) Unmarshal(data []byte, v interface{}) error {
	msg, ok := v.(*jsonMessage)
	if !ok {
		return json.Unmarshal(data, v)
	}
	msg.Data = make(json.RawMessage, len(data))
	copy(msg.Data, data)
	return nil
}

func (jsonCodec) Name() string {
	return "json"
}

// agentServiceDesc is the manually-constructed grpc.ServiceDesc for AgentServiceV2.
// This matches the proto service definition:
//
//	service AgentServiceV2 {
//	  rpc ProcessStream(stream ProxyToAgent) returns (stream AgentToProxy);
//	  rpc ControlStream(stream AgentControl) returns (stream ProxyControl);
//	  rpc ProcessEvent(ProxyToAgent) returns (AgentToProxy);
//	}
var agentServiceDesc = grpc.ServiceDesc{
	ServiceName: "zentinel.agent.v2.AgentServiceV2",
	HandlerType: (*agentGRPCService)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ProcessEvent",
			Handler:    processEventHandler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "ProcessStream",
			Handler:       processStreamHandler,
			ServerStreams:  true,
			ClientStreams:  true,
		},
		{
			StreamName:    "ControlStream",
			Handler:       controlStreamHandler,
			ServerStreams:  true,
			ClientStreams:  true,
		},
	},
	Metadata: "agent_v2.proto",
}

// registerAgentService registers the AgentServiceV2 gRPC service on the given server.
func registerAgentService(s *grpc.Server, runner *AgentRunnerV2) {
	svc := &agentGRPCService{
		runner: runner,
	}
	s.RegisterService(&agentServiceDesc, svc)
}

// processEventHandler handles the unary ProcessEvent RPC.
func processEventHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	svc := srv.(*agentGRPCService)

	in := &jsonMessage{}
	if err := dec(in); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to decode request: %v", err)
	}

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return svc.processEvent(ctx, req.(*jsonMessage))
	}

	if interceptor == nil {
		return handler(ctx, in)
	}

	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/zentinel.agent.v2.AgentServiceV2/ProcessEvent",
	}
	return interceptor(ctx, in, info, handler)
}

// processEvent handles a single unary ProxyToAgent message.
func (s *agentGRPCService) processEvent(ctx context.Context, in *jsonMessage) (*jsonMessage, error) {
	// Try to handle configure events directly
	if s.handleConfigureEvent(ctx, in.Data) {
		return &jsonMessage{Data: json.RawMessage(`{}`)}, nil
	}

	// Convert gRPC message to V2Message
	v2Msg, err := grpcProxyToV2Message(in.Data)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to convert message: %v", err)
	}

	if v2Msg == nil {
		return &jsonMessage{Data: json.RawMessage(`{}`)}, nil
	}

	// Process through the existing handler
	response, err := s.runner.handler.HandleMessage(ctx, v2Msg)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "handler error: %v", err)
	}

	if response == nil {
		return &jsonMessage{Data: json.RawMessage(`{}`)}, nil
	}

	// Convert response back to gRPC format
	respData, err := v2MessageToGRPCResponse(response)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert response: %v", err)
	}

	return &jsonMessage{Data: respData}, nil
}

// processStreamHandler handles the bidirectional streaming ProcessStream RPC.
func processStreamHandler(srv interface{}, stream grpc.ServerStream) error {
	svc := srv.(*agentGRPCService)
	return svc.processStream(stream)
}

// processStream implements bidirectional streaming for ProcessStream.
// It receives ProxyToAgent messages, processes them through the handler,
// and sends AgentToProxy responses back.
func (s *agentGRPCService) processStream(stream grpc.ServerStream) error {
	s.runner.wg.Add(1)
	defer s.runner.wg.Done()

	id := s.streamID.Add(1)
	streamID := fmt.Sprintf("grpc-stream-%d", id)
	ctx := stream.Context()

	log.Debug().Str("stream_id", streamID).Msg("gRPC ProcessStream started")
	defer func() {
		s.runner.agent.OnStreamClosed(ctx, streamID)
		log.Debug().Str("stream_id", streamID).Msg("gRPC ProcessStream ended")
	}()

	// Use a mutex for sending on the stream (gRPC streams are not safe for concurrent sends)
	var sendMu sync.Mutex

	for {
		// Check for shutdown
		select {
		case <-s.runner.shutdown:
			return status.Error(codes.Unavailable, "server shutting down")
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Check if draining
		s.runner.mu.RLock()
		draining := s.runner.draining
		s.runner.mu.RUnlock()
		if draining {
			return status.Error(codes.Unavailable, "server draining")
		}

		// Receive next message
		in := &jsonMessage{}
		if err := stream.RecvMsg(in); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		// Try to handle configure events directly (they bypass the V2Message handler)
		if handled := s.handleConfigureEvent(ctx, in.Data); handled {
			continue
		}

		// Convert and process through the V2Message handler
		v2Msg, err := grpcProxyToV2Message(in.Data)
		if err != nil {
			log.Error().Err(err).Str("stream_id", streamID).Msg("Failed to convert gRPC message")
			continue
		}

		if v2Msg == nil {
			continue
		}

		response, err := s.runner.handler.HandleMessage(ctx, v2Msg)
		if err != nil {
			log.Error().Err(err).Str("stream_id", streamID).Msg("Failed to handle message")
			continue
		}

		// Some messages (like cancel) don't have responses
		if response == nil {
			continue
		}

		respData, err := v2MessageToGRPCResponse(response)
		if err != nil {
			log.Error().Err(err).Str("stream_id", streamID).Msg("Failed to convert response")
			continue
		}

		sendMu.Lock()
		sendErr := stream.SendMsg(&jsonMessage{Data: respData})
		sendMu.Unlock()

		if sendErr != nil {
			log.Error().Err(sendErr).Str("stream_id", streamID).Msg("Failed to send response")
			return sendErr
		}
	}
}

// handleConfigureEvent checks if the raw gRPC message is a configure event and handles it
// directly by calling the agent's OnConfigure method. Returns true if handled.
func (s *agentGRPCService) handleConfigureEvent(ctx context.Context, data json.RawMessage) bool {
	var msg struct {
		Configure *grpcConfigureEvent `json:"configure,omitempty"`
	}
	if err := json.Unmarshal(data, &msg); err != nil || msg.Configure == nil {
		return false
	}

	var config map[string]interface{}
	if msg.Configure.ConfigJSON != "" {
		if err := json.Unmarshal([]byte(msg.Configure.ConfigJSON), &config); err != nil {
			log.Error().Err(err).Msg("Failed to parse configure event config JSON")
			config = map[string]interface{}{}
		}
	}

	if err := s.runner.agent.OnConfigure(ctx, config); err != nil {
		log.Error().Err(err).Msg("Agent OnConfigure failed")
	} else {
		log.Debug().Msg("Agent configuration applied via gRPC")
	}

	return true
}

// controlStreamHandler handles the bidirectional streaming ControlStream RPC.
func controlStreamHandler(srv interface{}, stream grpc.ServerStream) error {
	svc := srv.(*agentGRPCService)
	return svc.controlStream(stream)
}

// controlStream implements the ControlStream RPC for health, metrics, and config updates.
func (s *agentGRPCService) controlStream(stream grpc.ServerStream) error {
	s.runner.wg.Add(1)
	defer s.runner.wg.Done()

	id := s.streamID.Add(1)
	streamID := fmt.Sprintf("grpc-control-%d", id)
	ctx := stream.Context()

	log.Debug().Str("stream_id", streamID).Msg("gRPC ControlStream started")
	defer log.Debug().Str("stream_id", streamID).Msg("gRPC ControlStream ended")

	var sendMu sync.Mutex

	for {
		select {
		case <-s.runner.shutdown:
			return status.Error(codes.Unavailable, "server shutting down")
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		in := &jsonMessage{}
		if err := stream.RecvMsg(in); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		// Parse the control message
		var controlMsg struct {
			Health     json.RawMessage `json:"health,omitempty"`
			Metrics    json.RawMessage `json:"metrics,omitempty"`
			ConfigUpdate json.RawMessage `json:"config_update,omitempty"`
			Log        json.RawMessage `json:"log,omitempty"`
		}
		if err := json.Unmarshal(in.Data, &controlMsg); err != nil {
			log.Error().Err(err).Str("stream_id", streamID).Msg("Failed to parse control message")
			continue
		}

		// Handle health request - respond with current health
		if controlMsg.Health != nil {
			health := s.runner.agent.HealthCheck(ctx)
			state := int32(1)
			switch health.State {
			case HealthStateDegraded:
				state = 2
			case HealthStateUnhealthy:
				state = 4
			}
			resp := map[string]interface{}{
				"configure": map[string]interface{}{
					"config_json":  "{}",
					"is_initial":   false,
					"timestamp_ms": time.Now().UnixMilli(),
				},
			}
			_ = state // health state tracked but control stream responds with ProxyControl
			respData, err := json.Marshal(resp)
			if err != nil {
				log.Error().Err(err).Msg("Failed to marshal control response")
				continue
			}

			sendMu.Lock()
			sendErr := stream.SendMsg(&jsonMessage{Data: respData})
			sendMu.Unlock()
			if sendErr != nil {
				return sendErr
			}
		}

		// Handle metrics - respond with acknowledgment
		if controlMsg.Metrics != nil {
			// Metrics are fire-and-forget from the agent side
			log.Debug().Str("stream_id", streamID).Msg("Received metrics report via control stream")
		}
	}
}
