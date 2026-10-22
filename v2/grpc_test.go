package v2

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	pb "github.com/zentinelproxy/zentinel-agent-go-sdk/v2/proto"
)

func startGRPC(t *testing.T, agent AgentV2) pb.AgentServiceV2Client {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	server := grpc.NewServer()
	RegisterGRPCService(server, NewAgentHandlerV2(agent))
	go func() { _ = server.Serve(lis) }()
	t.Cleanup(server.Stop)
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return pb.NewAgentServiceV2Client(conn)
}

// TestGRPC_ProxySequence drives the stream the way the proxy does:
// handshake, ConfigureEvent + Ping (the Pong acknowledges), then events.
func TestGRPC_ProxySequence(t *testing.T) {
	client := startGRPC(t, &wireAgent{BaseAgentV2: *NewBaseAgentV2()})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stream, err := client.ProcessStream(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// The proxy waits for response headers before sending its handshake.
	if _, err := stream.Header(); err != nil {
		t.Fatalf("no response headers: %v", err)
	}

	send := func(m *pb.ProxyToAgent) {
		t.Helper()
		if err := stream.Send(m); err != nil {
			t.Fatal(err)
		}
	}
	send(&pb.ProxyToAgent{Message: &pb.ProxyToAgent_Handshake{Handshake: &pb.HandshakeRequest{SupportedVersions: []uint32{2, 1}, ProxyId: "zentinel-proxy", ConfigJson: `{"greeting":"grpc"}`}}})
	in, err := stream.Recv()
	if err != nil {
		t.Fatal(err)
	}
	hs := in.GetHandshake()
	if !hs.GetSuccess() || hs.GetProtocolVersion() != 2 || hs.GetCapabilities().GetAgentId() != "wire" {
		t.Fatalf("bad handshake: %v", in)
	}

	send(&pb.ProxyToAgent{Message: &pb.ProxyToAgent_Configure{Configure: &pb.ConfigureEvent{ConfigJson: `{"greeting":"grpc"}`, IsInitial: true}}})
	send(&pb.ProxyToAgent{Message: &pb.ProxyToAgent_Ping{Ping: &pb.Ping{Sequence: 41, TimestampMs: 5}}})
	in, err = stream.Recv()
	if err != nil || in.GetPong().GetSequence() != 41 || in.GetPong().GetPingTimestampMs() != 5 {
		t.Fatalf("bad pong: %v %v", in, err)
	}

	send(&pb.ProxyToAgent{Message: &pb.ProxyToAgent_RequestHeaders{RequestHeaders: &pb.RequestHeadersEvent{
		Metadata: &pb.RequestMetadata{CorrelationId: "g1"}, Method: "GET", Uri: "/x",
		Headers: []*pb.Header{{Name: "host", Value: "a"}},
	}}})
	in, err = stream.Recv()
	if err != nil {
		t.Fatal(err)
	}
	resp := in.GetResponse()
	if resp.GetCorrelationId() != "g1" || resp.GetAllow() == nil || len(resp.GetRequestHeaders()) != 1 ||
		resp.GetRequestHeaders()[0].GetSet().GetValue() != "grpc" {
		t.Fatalf("bad response: %v", in)
	}

	send(&pb.ProxyToAgent{Message: &pb.ProxyToAgent_RequestHeaders{RequestHeaders: &pb.RequestHeadersEvent{
		Metadata: &pb.RequestMetadata{CorrelationId: "g2"}, Method: "POST", Uri: "/b",
	}}})
	if _, err := stream.Recv(); err != nil {
		t.Fatal(err)
	}
	send(&pb.ProxyToAgent{Message: &pb.ProxyToAgent_RequestBodyChunk{RequestBodyChunk: &pb.BodyChunkEvent{CorrelationId: "g2", Data: []byte("so evil"), IsLast: true}}})
	in, err = stream.Recv()
	if err != nil {
		t.Fatal(err)
	}
	block := in.GetResponse().GetBlock()
	if in.GetResponse().GetCorrelationId() != "g2" || block.GetStatus() != 403 || block.GetBody() != "body:so evil" {
		t.Fatalf("bad body block: %v", in)
	}
}

func TestGRPC_ConfigureRejectedEndsStream(t *testing.T) {
	client := startGRPC(t, &wireAgent{BaseAgentV2: *NewBaseAgentV2(), reject: true})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stream, err := client.ProcessStream(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_ = stream.Send(&pb.ProxyToAgent{Message: &pb.ProxyToAgent_Handshake{Handshake: &pb.HandshakeRequest{SupportedVersions: []uint32{2}}}})
	if _, err := stream.Recv(); err != nil {
		t.Fatal(err)
	}
	_ = stream.Send(&pb.ProxyToAgent{Message: &pb.ProxyToAgent_Configure{Configure: &pb.ConfigureEvent{ConfigJson: `{}`}}})
	_ = stream.Send(&pb.ProxyToAgent{Message: &pb.ProxyToAgent_Ping{Ping: &pb.Ping{Sequence: 1}}})
	if in, err := stream.Recv(); err == nil {
		t.Fatalf("stream continued after a rejected configuration: %v", in)
	}
}
