package v2

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	zentinel "github.com/zentinelproxy/zentinel-agent-go-sdk"
)

// wireAgent is a small agent for driving the transports in tests.
type wireAgent struct {
	BaseAgentV2
	mu     sync.Mutex
	config map[string]interface{}
	reject bool
}

func (a *wireAgent) Name() string { return "wire" }

func (a *wireAgent) OnConfigure(ctx context.Context, config map[string]interface{}) error {
	if a.reject {
		return errors.New("no thanks")
	}
	a.mu.Lock()
	a.config = config
	a.mu.Unlock()
	return nil
}

func (a *wireAgent) OnRequest(ctx context.Context, req *zentinel.Request) *zentinel.Decision {
	if req.PathStartsWith("/block") {
		return zentinel.Deny().WithBody("nope")
	}
	a.mu.Lock()
	greeting, _ := a.config["greeting"].(string)
	a.mu.Unlock()
	return zentinel.Allow().AddRequestHeader("X-Greeting", greeting)
}

func (a *wireAgent) OnRequestBody(ctx context.Context, req *zentinel.Request) *zentinel.Decision {
	if strings.Contains(req.BodyString(), "evil") {
		return zentinel.Deny().WithBody("body:" + req.BodyString())
	}
	return zentinel.Allow()
}

// fakeProxy speaks the proxy's side of the Unix socket protocol.
type fakeProxy struct {
	t    *testing.T
	conn net.Conn
}

func startUDS(t *testing.T, agent AgentV2) *fakeProxy {
	t.Helper()
	proxySide, agentSide := net.Pipe()
	go func() { _ = ServeUDSConn(context.Background(), agentSide, NewAgentHandlerV2(agent)) }()
	t.Cleanup(func() { proxySide.Close() })
	return &fakeProxy{t: t, conn: proxySide}
}

func (p *fakeProxy) send(msgType byte, payload string) {
	p.t.Helper()
	if err := WriteMessageV2(p.conn, &V2Message{Type: msgType, Payload: []byte(payload)}); err != nil {
		p.t.Fatalf("send: %v", err)
	}
}

func (p *fakeProxy) recv() (*V2Message, map[string]interface{}) {
	p.t.Helper()
	_ = p.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	msg, err := ReadMessageV2(p.conn)
	if err != nil || msg == nil {
		p.t.Fatalf("recv: %v", err)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(msg.Payload, &body); err != nil {
		p.t.Fatalf("payload is not a JSON object: %s", msg.Payload)
	}
	return msg, body
}

// handshake sends the request the proxy sends (zentinel 0.6.50+) and
// checks the response has every field the proxy's UdsHandshakeResponse
// requires, none null.
func (p *fakeProxy) handshake() {
	p.t.Helper()
	p.send(MsgTypeHandshakeRequest, `{"supported_versions":[2],"proxy_id":"zentinel-proxy","proxy_version":"0.6.50","config":{"greeting":"hi"},"supported_encodings":["json"]}`)
	msg, resp := p.recv()
	if msg.Type != MsgTypeHandshakeResponse {
		p.t.Fatalf("got %s, want HandshakeResponse", msg.TypeName())
	}
	if resp["protocol_version"] != float64(2) || resp["success"] != true || resp["encoding"] != "json" {
		p.t.Fatalf("bad handshake response: %s", msg.Payload)
	}
	if _, ok := resp["error"]; !ok {
		p.t.Fatalf("handshake response lacks error: %s", msg.Payload)
	}
	caps, ok := resp["capabilities"].(map[string]interface{})
	if !ok {
		p.t.Fatalf("capabilities is not an object: %s", msg.Payload)
	}
	for _, field := range []string{"agent_id", "name", "version", "supported_events", "features", "limits"} {
		if caps[field] == nil {
			p.t.Fatalf("capabilities.%s missing or null: %s", field, msg.Payload)
		}
	}
	for _, field := range []string{"streaming_body", "websocket", "guardrails", "config_push", "metrics_export", "concurrent_requests", "cancellation", "flow_control", "health_reporting"} {
		if caps["features"].(map[string]interface{})[field] == nil {
			p.t.Fatalf("features.%s missing: %s", field, msg.Payload)
		}
	}
	for _, field := range []string{"max_body_size", "max_concurrency", "preferred_chunk_size"} {
		if caps["limits"].(map[string]interface{})[field] == nil {
			p.t.Fatalf("limits.%s missing: %s", field, msg.Payload)
		}
	}
}

func correlationOf(t *testing.T, resp map[string]interface{}) string {
	t.Helper()
	audit, _ := resp["audit"].(map[string]interface{})
	custom, _ := audit["custom"].(map[string]interface{})
	cid, _ := custom["correlation_id"].(string)
	return cid
}

func TestUDS_HandshakeConfigureAndEvents(t *testing.T) {
	agent := &wireAgent{BaseAgentV2: *NewBaseAgentV2()}
	p := startUDS(t, agent)
	p.handshake()

	// The Configure message the proxy sends after the handshake (#603).
	p.send(MsgTypeConfigure, `{"config":{"greeting":"configured","nested":{"x":1}},"config_version":null,"correlation_id":"zentinel-configure-wire-0"}`)
	msg, resp := p.recv()
	if msg.Type != MsgTypeAgentResponse || resp["decision"] != "allow" || resp["version"] != float64(2) {
		t.Fatalf("bad configure ack: %s", msg.Payload)
	}
	if cid := correlationOf(t, resp); cid != "zentinel-configure-wire-0" {
		t.Fatalf("configure ack correlation id %q", cid)
	}

	p.send(MsgTypePing, `{"sequence":7}`)
	if msg, _ := p.recv(); msg.Type != MsgTypePong || !bytes.Equal(msg.Payload, []byte(`{"sequence":7}`)) {
		t.Fatalf("bad pong: %s %s", msg.TypeName(), msg.Payload)
	}

	p.send(MsgTypeRequestHeaders, `{"metadata":{"correlation_id":"c1","request_id":"r1","client_ip":"127.0.0.1","client_port":1,"protocol":"HTTP/1.1","timestamp":"now"},"method":"GET","uri":"/x","headers":{"host":["a"]},"correlation_id":"c1"}`)
	msg, resp = p.recv()
	if cid := correlationOf(t, resp); cid != "c1" {
		t.Fatalf("response correlation id %q: %s", cid, msg.Payload)
	}
	ops, _ := resp["request_headers"].([]interface{})
	if len(ops) != 1 || !strings.Contains(string(msg.Payload), `{"set":{"name":"X-Greeting","value":"configured"}}`) {
		t.Fatalf("header op not applied from configuration: %s", msg.Payload)
	}

	p.send(MsgTypeRequestHeaders, `{"metadata":{"correlation_id":"c2","request_id":"r2","client_ip":"127.0.0.1","client_port":1,"protocol":"HTTP/1.1","timestamp":"now"},"method":"GET","uri":"/block/me","headers":{}}`)
	msg, resp = p.recv()
	if !strings.Contains(string(msg.Payload), `"decision":{"block":{"body":"nope","status":403}}`) || correlationOf(t, resp) != "c2" {
		t.Fatalf("bad block: %s", msg.Payload)
	}

	// The body arrives base64-encoded, in chunks.
	p.send(MsgTypeRequestHeaders, `{"metadata":{"correlation_id":"c3","request_id":"r3","client_ip":"127.0.0.1","client_port":1,"protocol":"HTTP/1.1","timestamp":"now"},"method":"POST","uri":"/b","headers":{}}`)
	p.recv()
	first := base64.StdEncoding.EncodeToString([]byte("some "))
	last := base64.StdEncoding.EncodeToString([]byte("evil"))
	p.send(MsgTypeRequestBodyChunk, `{"correlation_id":"c3","data":"`+first+`","is_last":false,"total_size":null,"chunk_index":0,"bytes_received":5}`)
	if _, resp := p.recv(); resp["needs_more"] != true || correlationOf(t, resp) != "c3" {
		t.Fatalf("first chunk not answered with needs_more: %v", resp)
	}
	p.send(MsgTypeRequestBodyChunk, `{"correlation_id":"c3","data":"`+last+`","is_last":true,"total_size":null,"chunk_index":1,"bytes_received":9}`)
	if msg, _ := p.recv(); !strings.Contains(string(msg.Payload), `"body":"body:some evil"`) {
		t.Fatalf("body not seen whole: %s", msg.Payload)
	}
}

func TestUDS_ConfigureRejected(t *testing.T) {
	agent := &wireAgent{BaseAgentV2: *NewBaseAgentV2(), reject: true}
	p := startUDS(t, agent)
	p.handshake()
	p.send(MsgTypeConfigure, `{"config":{},"config_version":null,"correlation_id":"cfg"}`)
	msg, resp := p.recv()
	if !strings.Contains(string(msg.Payload), `"status":500`) || correlationOf(t, resp) != "cfg" {
		t.Fatalf("rejection not reported as a block: %s", msg.Payload)
	}
}

func TestUDS_HandshakeRejectsOtherVersions(t *testing.T) {
	p := startUDS(t, &wireAgent{BaseAgentV2: *NewBaseAgentV2()})
	p.send(MsgTypeHandshakeRequest, `{"supported_versions":[1],"proxy_id":"p","proxy_version":"0","config":null}`)
	msg, resp := p.recv()
	if resp["success"] != false || resp["error"] == nil || resp["capabilities"] == nil {
		t.Fatalf("bad rejection: %s", msg.Payload)
	}
}

func TestFraming_LengthCountsTypeByte(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteMessageV2(&buf, &V2Message{Type: MsgTypePong, Payload: []byte(`{}`)}); err != nil {
		t.Fatal(err)
	}
	if got := buf.Bytes(); !bytes.Equal(got, []byte{0, 0, 0, 3, 0x42, '{', '}'}) {
		t.Fatalf("frame %v", got)
	}
	msg, err := ReadMessageV2(&buf)
	if err != nil || msg.Type != MsgTypePong || string(msg.Payload) != "{}" {
		t.Fatalf("round trip: %v %v", msg, err)
	}
	if msg, err := ReadMessageV2(&buf); msg != nil || err != nil {
		t.Fatalf("clean EOF: %v %v", msg, err)
	}
	if _, err := ReadMessageV2(bytes.NewReader([]byte{0, 0, 0, 0})); err == nil {
		t.Fatal("zero length accepted")
	}
	if _, err := ReadMessageV2(bytes.NewReader([]byte{0x7f, 0, 0, 0})); err == nil {
		t.Fatal("oversized frame accepted")
	}
}
