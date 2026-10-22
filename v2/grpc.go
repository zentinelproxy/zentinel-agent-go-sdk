package v2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	zentinel "github.com/zentinelproxy/zentinel-agent-go-sdk"
	pb "github.com/zentinelproxy/zentinel-agent-go-sdk/v2/proto"
)

// grpcService implements zentinel.agent.v2.AgentServiceV2 with the protobuf
// codec, as the proxy's gRPC client expects.
type grpcService struct {
	pb.UnimplementedAgentServiceV2Server
	handler *AgentHandlerV2
}

// RegisterGRPCService registers the agent service for handler on s. The
// runner's gRPC transport does this; use it to serve an agent from your own
// grpc.Server.
func RegisterGRPCService(s grpc.ServiceRegistrar, handler *AgentHandlerV2) {
	pb.RegisterAgentServiceV2Server(s, &grpcService{handler: handler})
}

func nowMs() uint64 { return uint64(time.Now().UnixMilli()) }

func (s *grpcService) handshakeResponse(req *pb.HandshakeRequest) *pb.AgentToProxy {
	agent := s.handler.Agent()
	resp := &pb.HandshakeResponse{
		ProtocolVersion: ProtocolVersionV2,
		Capabilities:    agent.Capabilities().toProto(agent.Name()),
		Success:         true,
	}
	hs := HandshakeRequest{SupportedVersions: req.GetSupportedVersions()}
	if !hs.supportsVersion() {
		reason := fmt.Sprintf("agent speaks protocol v2; proxy offered %v", req.GetSupportedVersions())
		resp.Success = false
		resp.Error = &reason
	}
	return &pb.AgentToProxy{Message: &pb.AgentToProxy_Handshake{Handshake: resp}}
}

// ProcessStream is the bidirectional stream the proxy opens per connection:
// handshake, then a ConfigureEvent and a Ping (the Pong acknowledges the
// configuration), then events.
func (s *grpcService) ProcessStream(stream grpc.BidiStreamingServer[pb.ProxyToAgent, pb.AgentToProxy]) error {
	ctx := stream.Context()
	// The proxy's client (tonic) does not send its handshake until the
	// response headers have arrived, and grpc-go holds them back until the
	// first message: without this, each side waits for the other until the
	// proxy's connect timeout.
	if err := stream.SendHeader(metadata.MD{}); err != nil {
		return err
	}
	var sendMu sync.Mutex
	send := func(m *pb.AgentToProxy) error {
		sendMu.Lock()
		defer sendMu.Unlock()
		return stream.Send(m)
	}
	var inflight sync.WaitGroup
	defer inflight.Wait()

	for {
		in, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}

		switch m := in.GetMessage().(type) {
		case *pb.ProxyToAgent_Handshake:
			resp := s.handshakeResponse(m.Handshake)
			if err := send(resp); err != nil {
				return err
			}
			if !resp.GetHandshake().GetSuccess() {
				return nil
			}
			log.Info().Str("proxy", m.Handshake.GetProxyId()).Msg("Handshake completed (gRPC)")
		case *pb.ProxyToAgent_Configure:
			// Handled in stream order, so the Ping that follows is answered
			// only once the configuration is applied. A rejection ends the
			// stream, which is how the proxy learns of it.
			if err := s.configure(ctx, m.Configure); err != nil {
				log.Error().Err(err).Msg("Configuration rejected")
				return status.Error(codes.FailedPrecondition, err.Error())
			}
			log.Info().Msg("Configuration applied")
		case *pb.ProxyToAgent_Ping:
			pong := &pb.Pong{Sequence: m.Ping.GetSequence(), PingTimestampMs: m.Ping.GetTimestampMs(), TimestampMs: nowMs()}
			if err := send(&pb.AgentToProxy{Message: &pb.AgentToProxy_Pong{Pong: pong}}); err != nil {
				return err
			}
		case *pb.ProxyToAgent_Cancel:
			s.handler.Cancel(ctx, m.Cancel.GetCorrelationId())
		case nil:
		default:
			inflight.Add(1)
			go func(in *pb.ProxyToAgent) {
				defer inflight.Done()
				out := s.event(ctx, in)
				if out == nil {
					return
				}
				if err := send(out); err != nil {
					log.Error().Err(err).Msg("Failed to send agent response")
				}
			}(in)
		}
	}
}

func (s *grpcService) configure(ctx context.Context, ev *pb.ConfigureEvent) error {
	config := map[string]interface{}{}
	if raw := ev.GetConfigJson(); raw != "" {
		if err := json.Unmarshal([]byte(raw), &config); err != nil {
			return fmt.Errorf("invalid configuration JSON: %w", err)
		}
	}
	return s.handler.Configure(ctx, config)
}

// event runs one event through the handler and returns the response
// message, or nil for messages that take none.
func (s *grpcService) event(ctx context.Context, in *pb.ProxyToAgent) *pb.AgentToProxy {
	var cid string
	var resp zentinel.AgentResponse
	switch m := in.GetMessage().(type) {
	case *pb.ProxyToAgent_RequestHeaders:
		ev := requestHeadersFromProto(m.RequestHeaders)
		cid = ev.Metadata.CorrelationID
		resp = s.handler.RequestHeaders(ctx, ev)
	case *pb.ProxyToAgent_RequestBodyChunk:
		cid = m.RequestBodyChunk.GetCorrelationId()
		resp = s.handler.RequestBodyChunk(ctx, cid, m.RequestBodyChunk.GetData(), m.RequestBodyChunk.GetIsLast())
	case *pb.ProxyToAgent_ResponseHeaders:
		cid = m.ResponseHeaders.GetCorrelationId()
		resp = s.handler.ResponseHeaders(ctx, &zentinel.ResponseHeadersEvent{
			CorrelationID: cid,
			Status:        int(m.ResponseHeaders.GetStatusCode()),
			Headers:       headersFromProto(m.ResponseHeaders.GetHeaders()),
		})
	case *pb.ProxyToAgent_ResponseBodyChunk:
		cid = m.ResponseBodyChunk.GetCorrelationId()
		resp = s.handler.ResponseBodyChunk(ctx, cid, m.ResponseBodyChunk.GetData(), m.ResponseBodyChunk.GetIsLast())
	case *pb.ProxyToAgent_RequestComplete:
		cid = m.RequestComplete.GetCorrelationId()
		ev := &zentinel.RequestCompleteEvent{
			CorrelationID: cid,
			Status:        int(m.RequestComplete.GetStatusCode()),
			DurationMS:    int(m.RequestComplete.GetDurationMs()),
			Error:         m.RequestComplete.Error,
		}
		resp = s.handler.RequestComplete(ctx, ev)
	case *pb.ProxyToAgent_WebsocketFrame:
		cid = m.WebsocketFrame.GetCorrelationId()
		resp = zentinel.Allow().Build()
	case *pb.ProxyToAgent_Guardrail:
		cid = m.Guardrail.GetCorrelationId()
		resp = zentinel.Allow().Build()
	default:
		return nil
	}
	return &pb.AgentToProxy{Message: &pb.AgentToProxy_Response{Response: responseToProto(cid, resp)}}
}

// ProcessEvent handles a single event without a stream.
func (s *grpcService) ProcessEvent(ctx context.Context, in *pb.ProxyToAgent) (*pb.AgentToProxy, error) {
	switch m := in.GetMessage().(type) {
	case *pb.ProxyToAgent_Handshake:
		return s.handshakeResponse(m.Handshake), nil
	case *pb.ProxyToAgent_Ping:
		pong := &pb.Pong{Sequence: m.Ping.GetSequence(), PingTimestampMs: m.Ping.GetTimestampMs(), TimestampMs: nowMs()}
		return &pb.AgentToProxy{Message: &pb.AgentToProxy_Pong{Pong: pong}}, nil
	case *pb.ProxyToAgent_Configure:
		if err := s.configure(ctx, m.Configure); err != nil {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		return &pb.AgentToProxy{}, nil
	}
	if out := s.event(ctx, in); out != nil {
		return out, nil
	}
	return nil, status.Error(codes.InvalidArgument, "unsupported message")
}

// ControlStream accepts the control stream and drains it; this SDK sends no
// control messages yet.
func (s *grpcService) ControlStream(stream grpc.BidiStreamingServer[pb.AgentControl, pb.ProxyControl]) error {
	for {
		if _, err := stream.Recv(); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

func headersFromProto(headers []*pb.Header) map[string][]string {
	out := make(map[string][]string, len(headers))
	for _, h := range headers {
		out[h.GetName()] = append(out[h.GetName()], h.GetValue())
	}
	return out
}

func requestHeadersFromProto(ev *pb.RequestHeadersEvent) *zentinel.RequestHeadersEvent {
	md := ev.GetMetadata()
	var ts *string
	if md.GetTimestampMs() != 0 {
		t := time.UnixMilli(int64(md.GetTimestampMs())).UTC().Format(time.RFC3339Nano)
		ts = &t
	}
	return &zentinel.RequestHeadersEvent{
		Metadata: zentinel.RequestMetadata{
			CorrelationID: md.GetCorrelationId(),
			RequestID:     md.GetRequestId(),
			ClientIP:      md.GetClientIp(),
			ClientPort:    int(md.GetClientPort()),
			ServerName:    md.ServerName,
			Protocol:      md.GetProtocol(),
			TLSVersion:    md.TlsVersion,
			RouteID:       md.RouteId,
			UpstreamID:    md.UpstreamId,
			Timestamp:     ts,
			Traceparent:   md.Traceparent,
		},
		Method:  ev.GetMethod(),
		URI:     ev.GetUri(),
		Headers: headersFromProto(ev.GetHeaders()),
	}
}

// jsonDecision is the decision of an AgentResponse in its JSON form, which
// is what zentinel.Decision builds.
type jsonDecision struct {
	Block *struct {
		Status  uint32            `json:"status"`
		Body    *string           `json:"body"`
		Headers map[string]string `json:"headers"`
	} `json:"block"`
	Redirect *struct {
		URL    string `json:"url"`
		Status uint32 `json:"status"`
	} `json:"redirect"`
	Challenge *struct {
		ChallengeType string                 `json:"challenge_type"`
		Params        map[string]interface{} `json:"params"`
	} `json:"challenge"`
}

func stringify(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}

func headerOpsToProto(ops []zentinel.HeaderOp) []*pb.HeaderOp {
	out := make([]*pb.HeaderOp, 0, len(ops))
	for _, op := range ops {
		value := ""
		if op.Value != nil {
			value = *op.Value
		}
		h := &pb.Header{Name: op.Name, Value: value}
		switch op.Operation {
		case "add":
			out = append(out, &pb.HeaderOp{Operation: &pb.HeaderOp_Add{Add: h}})
		case "remove":
			out = append(out, &pb.HeaderOp{Operation: &pb.HeaderOp_Remove{Remove: op.Name}})
		default:
			out = append(out, &pb.HeaderOp{Operation: &pb.HeaderOp_Set{Set: h}})
		}
	}
	return out
}

func responseToProto(correlationID string, resp zentinel.AgentResponse) *pb.AgentResponse {
	out := &pb.AgentResponse{
		CorrelationId:   correlationID,
		RequestHeaders:  headerOpsToProto(resp.RequestHeaders),
		ResponseHeaders: headerOpsToProto(resp.ResponseHeaders),
		NeedsMore:       resp.NeedsMore,
		Decision:        &pb.AgentResponse_Allow{Allow: &pb.AllowDecision{}},
	}

	if resp.Decision != "allow" {
		var d jsonDecision
		if raw, err := json.Marshal(resp.Decision); err == nil {
			_ = json.Unmarshal(raw, &d)
		}
		switch {
		case d.Block != nil:
			block := &pb.BlockDecision{Status: d.Block.Status, Body: d.Block.Body}
			for name, value := range d.Block.Headers {
				block.Headers = append(block.Headers, &pb.Header{Name: name, Value: value})
			}
			out.Decision = &pb.AgentResponse_Block{Block: block}
		case d.Redirect != nil:
			out.Decision = &pb.AgentResponse_Redirect{Redirect: &pb.RedirectDecision{Url: d.Redirect.URL, Status: d.Redirect.Status}}
		case d.Challenge != nil:
			params := map[string]string{}
			for k, v := range d.Challenge.Params {
				params[k] = stringify(v)
			}
			out.Decision = &pb.AgentResponse_Challenge{Challenge: &pb.ChallengeDecision{ChallengeType: d.Challenge.ChallengeType, Params: params}}
		}
	}

	a := resp.Audit
	if len(a.Tags) > 0 || len(a.RuleIDs) > 0 || a.Confidence != nil || len(a.ReasonCodes) > 0 || len(a.Custom) > 0 {
		audit := &pb.AuditMetadata{Tags: a.Tags, RuleIds: a.RuleIDs, ReasonCodes: a.ReasonCodes}
		if a.Confidence != nil {
			c := float32(*a.Confidence)
			audit.Confidence = &c
		}
		if len(a.Custom) > 0 {
			audit.Custom = make(map[string]string, len(a.Custom))
			for k, v := range a.Custom {
				audit.Custom[k] = stringify(v)
			}
		}
		out.Audit = audit
	}
	return out
}
