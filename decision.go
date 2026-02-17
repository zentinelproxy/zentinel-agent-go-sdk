package zentinel

import (
	"encoding/base64"
	"encoding/json"
)

// Decision is a fluent builder for agent decisions.
type Decision struct {
	decision             interface{}
	requestHeaders       []HeaderOp
	responseHeaders      []HeaderOp
	routingMetadata      map[string]string
	audit                AuditMetadata
	needsMore            bool
	requestBodyMutation  map[string]interface{}
	responseBodyMutation map[string]interface{}
}

// Allow creates an allow decision (pass request through).
func Allow() *Decision {
	return &Decision{
		decision:        "allow",
		requestHeaders:  []HeaderOp{},
		responseHeaders: []HeaderOp{},
		routingMetadata: map[string]string{},
		audit:           AuditMetadata{},
	}
}

// Block creates a block decision (reject with status).
func Block(status int) *Decision {
	return &Decision{
		decision: map[string]interface{}{
			"block": map[string]interface{}{
				"status": status,
			},
		},
		requestHeaders:  []HeaderOp{},
		responseHeaders: []HeaderOp{},
		routingMetadata: map[string]string{},
		audit:           AuditMetadata{},
	}
}

// Deny creates a deny decision (block with 403).
func Deny() *Decision {
	return Block(403)
}

// Unauthorized creates an unauthorized decision (block with 401).
func Unauthorized() *Decision {
	return Block(401)
}

// RateLimited creates a rate limited decision (block with 429).
func RateLimited() *Decision {
	return Block(429)
}

// Redirect creates a redirect decision.
func Redirect(url string, status int) *Decision {
	return &Decision{
		decision: map[string]interface{}{
			"redirect": map[string]interface{}{
				"url":    url,
				"status": status,
			},
		},
		requestHeaders:  []HeaderOp{},
		responseHeaders: []HeaderOp{},
		routingMetadata: map[string]string{},
		audit:           AuditMetadata{},
	}
}

// RedirectPermanent creates a permanent redirect decision (301).
func RedirectPermanent(url string) *Decision {
	return Redirect(url, 301)
}

// Challenge creates a challenge decision (e.g., CAPTCHA).
func Challenge(challengeType string, params map[string]interface{}) *Decision {
	challengeData := map[string]interface{}{
		"challenge_type": challengeType,
	}
	if params != nil {
		challengeData["params"] = params
	}
	return &Decision{
		decision: map[string]interface{}{
			"challenge": challengeData,
		},
		requestHeaders:  []HeaderOp{},
		responseHeaders: []HeaderOp{},
		routingMetadata: map[string]string{},
		audit:           AuditMetadata{},
	}
}

// WithBody sets the response body for block decisions.
func (d *Decision) WithBody(body string) *Decision {
	if decision, ok := d.decision.(map[string]interface{}); ok {
		if block, ok := decision["block"].(map[string]interface{}); ok {
			block["body"] = body
		}
	}
	return d
}

// WithJSONBody sets a JSON response body for block decisions.
func (d *Decision) WithJSONBody(value interface{}) *Decision {
	if decision, ok := d.decision.(map[string]interface{}); ok {
		if block, ok := decision["block"].(map[string]interface{}); ok {
			jsonBytes, err := json.Marshal(value)
			if err == nil {
				block["body"] = string(jsonBytes)
				if block["headers"] == nil {
					block["headers"] = map[string]string{}
				}
				if headers, ok := block["headers"].(map[string]string); ok {
					headers["Content-Type"] = "application/json"
				}
			}
		}
	}
	return d
}

// WithBlockHeader adds a header to the block response.
func (d *Decision) WithBlockHeader(name, value string) *Decision {
	if decision, ok := d.decision.(map[string]interface{}); ok {
		if block, ok := decision["block"].(map[string]interface{}); ok {
			if block["headers"] == nil {
				block["headers"] = map[string]string{}
			}
			if headers, ok := block["headers"].(map[string]string); ok {
				headers[name] = value
			}
		}
	}
	return d
}

// AddRequestHeader adds a header to the upstream request.
func (d *Decision) AddRequestHeader(name, value string) *Decision {
	d.requestHeaders = append(d.requestHeaders, HeaderOp{
		Operation: "set",
		Name:      name,
		Value:     &value,
	})
	return d
}

// RemoveRequestHeader removes a header from the upstream request.
func (d *Decision) RemoveRequestHeader(name string) *Decision {
	d.requestHeaders = append(d.requestHeaders, HeaderOp{
		Operation: "remove",
		Name:      name,
	})
	return d
}

// AddResponseHeader adds a header to the client response.
func (d *Decision) AddResponseHeader(name, value string) *Decision {
	d.responseHeaders = append(d.responseHeaders, HeaderOp{
		Operation: "set",
		Name:      name,
		Value:     &value,
	})
	return d
}

// RemoveResponseHeader removes a header from the client response.
func (d *Decision) RemoveResponseHeader(name string) *Decision {
	d.responseHeaders = append(d.responseHeaders, HeaderOp{
		Operation: "remove",
		Name:      name,
	})
	return d
}

// WithRoutingMetadata adds routing metadata.
func (d *Decision) WithRoutingMetadata(key, value string) *Decision {
	d.routingMetadata[key] = value
	return d
}

// WithTag adds a single audit tag.
func (d *Decision) WithTag(tag string) *Decision {
	d.audit.Tags = append(d.audit.Tags, tag)
	return d
}

// WithTags adds multiple audit tags.
func (d *Decision) WithTags(tags ...string) *Decision {
	d.audit.Tags = append(d.audit.Tags, tags...)
	return d
}

// WithRuleID adds a rule ID to audit metadata.
func (d *Decision) WithRuleID(ruleID string) *Decision {
	d.audit.RuleIDs = append(d.audit.RuleIDs, ruleID)
	return d
}

// WithConfidence sets the confidence score.
func (d *Decision) WithConfidence(confidence float64) *Decision {
	d.audit.Confidence = &confidence
	return d
}

// WithReasonCode adds a reason code.
func (d *Decision) WithReasonCode(code string) *Decision {
	d.audit.ReasonCodes = append(d.audit.ReasonCodes, code)
	return d
}

// WithMetadata adds custom audit metadata.
func (d *Decision) WithMetadata(key string, value interface{}) *Decision {
	if d.audit.Custom == nil {
		d.audit.Custom = map[string]interface{}{}
	}
	d.audit.Custom[key] = value
	return d
}

// NeedsMoreData indicates that the agent needs more data (body chunks).
func (d *Decision) NeedsMoreData() *Decision {
	d.needsMore = true
	return d
}

// WithRequestBodyMutation sets request body mutation.
func (d *Decision) WithRequestBodyMutation(data []byte, chunkIndex int) *Decision {
	var encodedData *string
	if data != nil {
		encoded := base64.StdEncoding.EncodeToString(data)
		encodedData = &encoded
	}
	d.requestBodyMutation = map[string]interface{}{
		"data":        encodedData,
		"chunk_index": chunkIndex,
	}
	return d
}

// WithResponseBodyMutation sets response body mutation.
func (d *Decision) WithResponseBodyMutation(data []byte, chunkIndex int) *Decision {
	var encodedData *string
	if data != nil {
		encoded := base64.StdEncoding.EncodeToString(data)
		encodedData = &encoded
	}
	d.responseBodyMutation = map[string]interface{}{
		"data":        encodedData,
		"chunk_index": chunkIndex,
	}
	return d
}

// Build builds the AgentResponse.
func (d *Decision) Build() AgentResponse {
	return AgentResponse{
		Version:              ProtocolVersion,
		Decision:             d.decision,
		RequestHeaders:       d.requestHeaders,
		ResponseHeaders:      d.responseHeaders,
		RoutingMetadata:      d.routingMetadata,
		Audit:                d.audit,
		NeedsMore:            d.needsMore,
		RequestBodyMutation:  d.requestBodyMutation,
		ResponseBodyMutation: d.responseBodyMutation,
	}
}

// Decisions provides shorthand functions for common decisions.
var Decisions = struct {
	Allow       func() *Decision
	Deny        func() *Decision
	Unauthorized func() *Decision
	RateLimited func() *Decision
	Block       func(status int, body string) *Decision
	Redirect    func(url string, permanent bool) *Decision
}{
	Allow:       Allow,
	Deny:        Deny,
	Unauthorized: Unauthorized,
	RateLimited: RateLimited,
	Block: func(status int, body string) *Decision {
		d := Block(status)
		if body != "" {
			d = d.WithBody(body)
		}
		return d
	},
	Redirect: func(url string, permanent bool) *Decision {
		if permanent {
			return RedirectPermanent(url)
		}
		return Redirect(url, 302)
	},
}
