package v2

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/rs/zerolog/log"
	zentinel "github.com/zentinelproxy/zentinel-agent-go-sdk"
)

// udsConn serves one proxy connection on the Unix socket transport.
type udsConn struct {
	conn    net.Conn
	handler *AgentHandlerV2
	agent   AgentV2
	writeMu sync.Mutex
}

// ServeUDSConn speaks protocol v2 on an accepted Unix socket connection until
// it closes: the handshake, then events, each answered on the same
// connection. It is what the runner's UDS transport runs per connection.
func ServeUDSConn(ctx context.Context, conn net.Conn, handler *AgentHandlerV2) error {
	c := &udsConn{conn: conn, handler: handler, agent: handler.Agent()}
	return c.serve(ctx)
}

func (c *udsConn) write(msgType byte, payload interface{}) error {
	msg, err := NewV2Message(msgType, payload)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return WriteMessageV2(c.conn, msg)
}

// respond sends an AgentResponse. The proxy multiplexes requests over one
// connection and routes each response by the correlation id in audit.custom.
func (c *udsConn) respond(correlationID string, resp zentinel.AgentResponse) error {
	if resp.Audit.Custom == nil {
		resp.Audit.Custom = map[string]interface{}{}
	} else {
		custom := make(map[string]interface{}, len(resp.Audit.Custom)+1)
		for k, v := range resp.Audit.Custom {
			custom[k] = v
		}
		resp.Audit.Custom = custom
	}
	resp.Audit.Custom["correlation_id"] = correlationID
	if resp.RequestHeaders == nil {
		resp.RequestHeaders = []zentinel.HeaderOp{}
	}
	if resp.ResponseHeaders == nil {
		resp.ResponseHeaders = []zentinel.HeaderOp{}
	}
	if resp.RoutingMetadata == nil {
		resp.RoutingMetadata = map[string]string{}
	}
	return c.write(MsgTypeAgentResponse, resp)
}

func (c *udsConn) handshake(r io.Reader) error {
	msg, err := ReadMessageV2(r)
	if err != nil {
		return fmt.Errorf("failed to read handshake: %w", err)
	}
	if msg == nil {
		return fmt.Errorf("connection closed before the handshake")
	}
	if msg.Type != MsgTypeHandshakeRequest {
		return fmt.Errorf("expected HandshakeRequest, got %s", msg.TypeName())
	}

	caps := c.agent.Capabilities().ToUds(c.agent.Name())
	var req HandshakeRequest
	if err := msg.ParsePayload(&req); err != nil {
		_ = c.write(MsgTypeHandshakeResponse, NewHandshakeResponseError(caps, "invalid handshake request: "+err.Error()))
		return fmt.Errorf("invalid handshake request: %w", err)
	}
	if !req.supportsVersion() {
		reason := fmt.Sprintf("agent speaks protocol v2; proxy offered %v", req.SupportedVersions)
		_ = c.write(MsgTypeHandshakeResponse, NewHandshakeResponseError(caps, reason))
		return fmt.Errorf("%s", reason)
	}
	log.Info().Str("proxy", req.ProxyID).Str("proxy_version", req.ProxyVersion).Msg("Handshake completed (UDS)")
	return c.write(MsgTypeHandshakeResponse, NewHandshakeResponse(caps))
}

func (c *udsConn) serve(ctx context.Context) error {
	reader := bufio.NewReader(c.conn)
	if err := c.handshake(reader); err != nil {
		return err
	}

	var inflight sync.WaitGroup
	defer inflight.Wait()

	for {
		msg, err := ReadMessageV2(reader)
		if err != nil {
			return err
		}
		if msg == nil {
			return nil
		}

		switch msg.Type {
		case MsgTypePing:
			c.writeMu.Lock()
			err = WriteMessageV2(c.conn, &V2Message{Type: MsgTypePong, Payload: msg.Payload})
			c.writeMu.Unlock()
		case MsgTypeCancel:
			var cancel struct {
				CorrelationID string `json:"correlation_id"`
			}
			if msg.ParsePayload(&cancel) == nil {
				c.handler.Cancel(ctx, cancel.CorrelationID)
			}
		case MsgTypeConfigure:
			// Handled before the next message is read: the proxy does not
			// use the connection until the configuration is acknowledged.
			err = c.configure(ctx, msg)
		case MsgTypeRequestHeaders, MsgTypeRequestBodyChunk, MsgTypeResponseHeaders,
			MsgTypeResponseBodyChunk, MsgTypeRequestComplete, MsgTypeWebSocketFrame:
			inflight.Add(1)
			go func(msg *V2Message) {
				defer inflight.Done()
				cid, resp := c.dispatch(ctx, msg)
				if werr := c.respond(cid, resp); werr != nil {
					log.Error().Err(werr).Msg("Failed to write agent response")
				}
			}(msg)
		default:
			log.Debug().Str("type", msg.TypeName()).Msg("Ignoring message")
		}
		if err != nil {
			return err
		}
	}
}

func (c *udsConn) configure(ctx context.Context, msg *V2Message) error {
	var payload struct {
		CorrelationID string                 `json:"correlation_id"`
		Config        map[string]interface{} `json:"config"`
		ConfigVersion *string                `json:"config_version"`
	}
	resp := zentinel.Allow().Build()
	if err := msg.ParsePayload(&payload); err != nil {
		log.Error().Err(err).Msg("Invalid Configure message")
		resp = zentinel.Block(500).WithBody("invalid configuration: " + err.Error()).Build()
	} else if err := c.handler.Configure(ctx, payload.Config); err != nil {
		log.Error().Err(err).Msg("Configuration rejected")
		resp = zentinel.Block(500).WithBody("configuration rejected: " + err.Error()).Build()
	} else {
		log.Info().Msg("Configuration applied")
	}
	return c.respond(payload.CorrelationID, resp)
}

// dispatch decodes an event and runs the handler. A payload that does not
// decode is answered with allow, as the proxy's own server does, keeping the
// correlation id so the proxy is not left waiting.
func (c *udsConn) dispatch(ctx context.Context, msg *V2Message) (string, zentinel.AgentResponse) {
	var ids struct {
		CorrelationID string `json:"correlation_id"`
		Metadata      struct {
			CorrelationID string `json:"correlation_id"`
		} `json:"metadata"`
	}
	_ = json.Unmarshal(msg.Payload, &ids)
	cid := ids.CorrelationID
	if cid == "" {
		cid = ids.Metadata.CorrelationID
	}
	fail := func(err error) (string, zentinel.AgentResponse) {
		log.Error().Err(err).Str("type", msg.TypeName()).Msg("Failed to decode event")
		return cid, zentinel.Allow().Build()
	}

	switch msg.Type {
	case MsgTypeRequestHeaders:
		var ev zentinel.RequestHeadersEvent
		if err := msg.ParsePayload(&ev); err != nil {
			return fail(err)
		}
		if ev.Metadata.CorrelationID == "" {
			ev.Metadata.CorrelationID = cid
		}
		return ev.Metadata.CorrelationID, c.handler.RequestHeaders(ctx, &ev)
	case MsgTypeRequestBodyChunk:
		var ev zentinel.RequestBodyChunkEvent
		if err := msg.ParsePayload(&ev); err != nil {
			return fail(err)
		}
		data, err := ev.DecodedData()
		if err != nil {
			return fail(err)
		}
		return cid, c.handler.RequestBodyChunk(ctx, ev.CorrelationID, data, ev.IsLast)
	case MsgTypeResponseHeaders:
		var ev zentinel.ResponseHeadersEvent
		if err := msg.ParsePayload(&ev); err != nil {
			return fail(err)
		}
		return cid, c.handler.ResponseHeaders(ctx, &ev)
	case MsgTypeResponseBodyChunk:
		var ev zentinel.ResponseBodyChunkEvent
		if err := msg.ParsePayload(&ev); err != nil {
			return fail(err)
		}
		data, err := ev.DecodedData()
		if err != nil {
			return fail(err)
		}
		return cid, c.handler.ResponseBodyChunk(ctx, ev.CorrelationID, data, ev.IsLast)
	case MsgTypeRequestComplete:
		var ev zentinel.RequestCompleteEvent
		if err := msg.ParsePayload(&ev); err != nil {
			return fail(err)
		}
		return cid, c.handler.RequestComplete(ctx, &ev)
	default:
		// WebSocket frames: this SDK has no frame hook yet; let them through.
		return cid, zentinel.Allow().Build()
	}
}
