// Guardrail agent example for AI content safety.
//
// This example demonstrates a guardrail agent that:
// - Detects prompt injection attempts in user input
// - Detects PII (emails, phone numbers, SSN patterns)
// - Returns structured detection results with confidence scores
package main

import (
	"context"
	"regexp"
	"strings"

	zentinel "github.com/zentinelproxy/zentinel-agent-go-sdk"
)

// GuardrailAgent inspects content for prompt injection and PII.
type GuardrailAgent struct {
	zentinel.BaseAgent
	injectionPatterns []injectionPattern
	piiPatterns       []piiPattern
}

type injectionPattern struct {
	regex    *regexp.Regexp
	category string
}

type piiPattern struct {
	regex       *regexp.Regexp
	category    string
	description string
}

// NewGuardrailAgent creates a new guardrail agent with detection patterns.
func NewGuardrailAgent() *GuardrailAgent {
	return &GuardrailAgent{
		injectionPatterns: []injectionPattern{
			{regexp.MustCompile(`(?i)ignore\s+(all\s+)?(previous|prior|above)\s+(instructions?|prompts?)`), "ignore_instructions"},
			{regexp.MustCompile(`(?i)disregard\s+(all\s+)?(previous|prior|above)`), "disregard_previous"},
			{regexp.MustCompile(`(?i)you\s+are\s+now\s+(a|an|in)\s+`), "role_switch"},
			{regexp.MustCompile(`(?i)pretend\s+(you('re|are)|to\s+be)`), "pretend_role"},
			{regexp.MustCompile(`(?i)system\s*:\s*`), "system_prompt_inject"},
			{regexp.MustCompile(`\[INST\]|\[/INST\]|<<SYS>>|<</SYS>>`), "llama_format_inject"},
			{regexp.MustCompile(`<\|im_start\|>|<\|im_end\|>`), "chatml_format_inject"},
		},
		piiPatterns: []piiPattern{
			{regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`), "email", "Email address"},
			{regexp.MustCompile(`\b\d{3}[-.]?\d{3}[-.]?\d{4}\b`), "phone", "Phone number"},
			{regexp.MustCompile(`\b\d{3}[-]?\d{2}[-]?\d{4}\b`), "ssn", "Social Security Number"},
			{regexp.MustCompile(`\b\d{4}[-\s]?\d{4}[-\s]?\d{4}[-\s]?\d{4}\b`), "credit_card", "Credit card number"},
		},
	}
}

// Name returns the agent name.
func (a *GuardrailAgent) Name() string {
	return "guardrail-agent"
}

// OnGuardrailInspect inspects content for prompt injection or PII.
func (a *GuardrailAgent) OnGuardrailInspect(ctx context.Context, event *zentinel.GuardrailInspectEvent) *zentinel.GuardrailResponse {
	switch event.InspectionType {
	case zentinel.GuardrailInspectionTypePromptInjection:
		return a.detectPromptInjection(event.Content)
	case zentinel.GuardrailInspectionTypePIIDetection:
		return a.detectPII(event.Content)
	default:
		return zentinel.NewGuardrailResponse()
	}
}

func (a *GuardrailAgent) detectPromptInjection(content string) *zentinel.GuardrailResponse {
	response := zentinel.NewGuardrailResponse()

	for _, pattern := range a.injectionPatterns {
		loc := pattern.regex.FindStringIndex(content)
		if loc != nil {
			detection := &zentinel.GuardrailDetection{
				Category:    "prompt_injection." + pattern.category,
				Description: "Potential prompt injection detected: " + strings.ReplaceAll(pattern.category, "_", " "),
				Severity:    zentinel.DetectionSeverityHigh,
				Confidence:  floatPtr(0.85),
				Span:        &zentinel.TextSpan{Start: loc[0], End: loc[1]},
			}
			response.AddDetection(detection)
		}
	}

	return response
}

func (a *GuardrailAgent) detectPII(content string) *zentinel.GuardrailResponse {
	response := zentinel.NewGuardrailResponse()
	redacted := content

	for _, pattern := range a.piiPatterns {
		matches := pattern.regex.FindAllStringIndex(content, -1)
		for _, loc := range matches {
			matched := content[loc[0]:loc[1]]
			detection := &zentinel.GuardrailDetection{
				Category:    "pii." + pattern.category,
				Description: pattern.description + " detected",
				Severity:    zentinel.DetectionSeverityMedium,
				Confidence:  floatPtr(0.95),
				Span:        &zentinel.TextSpan{Start: loc[0], End: loc[1]},
			}
			response.AddDetection(detection)
			redacted = strings.Replace(redacted, matched, "[REDACTED_"+strings.ToUpper(pattern.category)+"]", 1)
		}
	}

	if response.Detected {
		response.RedactedContent = &redacted
	}

	return response
}

func floatPtr(f float64) *float64 {
	return &f
}

func main() {
	zentinel.RunAgent(NewGuardrailAgent())
}
