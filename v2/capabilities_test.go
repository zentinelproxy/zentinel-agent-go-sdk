package v2

import (
	"testing"
)

func TestNewAgentCapabilities(t *testing.T) {
	caps := NewAgentCapabilities()

	if !caps.HandlesRequestHeaders {
		t.Error("expected HandlesRequestHeaders to be true by default")
	}
	if caps.HandlesRequestBody {
		t.Error("expected HandlesRequestBody to be false by default")
	}
	if caps.HandlesResponseHeaders {
		t.Error("expected HandlesResponseHeaders to be false by default")
	}
	if caps.HandlesResponseBody {
		t.Error("expected HandlesResponseBody to be false by default")
	}
	if !caps.SupportsCancellation {
		t.Error("expected SupportsCancellation to be true by default")
	}
	if caps.MaxConcurrentRequests != nil {
		t.Error("expected MaxConcurrentRequests to be nil by default")
	}
}

func TestAgentCapabilities_Builder(t *testing.T) {
	caps := NewAgentCapabilities().
		HandleRequestHeaders().
		HandleRequestBody().
		HandleResponseHeaders().
		HandleResponseBody().
		WithStreaming().
		WithMaxConcurrentRequests(100).
		WithFeature("custom-feature")

	if !caps.HandlesRequestHeaders {
		t.Error("expected HandlesRequestHeaders to be true")
	}
	if !caps.HandlesRequestBody {
		t.Error("expected HandlesRequestBody to be true")
	}
	if !caps.HandlesResponseHeaders {
		t.Error("expected HandlesResponseHeaders to be true")
	}
	if !caps.HandlesResponseBody {
		t.Error("expected HandlesResponseBody to be true")
	}
	if !caps.SupportsStreaming {
		t.Error("expected SupportsStreaming to be true")
	}
	if caps.MaxConcurrentRequests == nil || *caps.MaxConcurrentRequests != 100 {
		t.Errorf("expected MaxConcurrentRequests to be 100, got %v", caps.MaxConcurrentRequests)
	}
	if !caps.HasFeature("custom-feature") {
		t.Error("expected HasFeature('custom-feature') to be true")
	}
}

func TestAgentCapabilities_All(t *testing.T) {
	caps := NewAgentCapabilities().All()

	if !caps.HandlesRequestHeaders {
		t.Error("expected HandlesRequestHeaders to be true")
	}
	if !caps.HandlesRequestBody {
		t.Error("expected HandlesRequestBody to be true")
	}
	if !caps.HandlesResponseHeaders {
		t.Error("expected HandlesResponseHeaders to be true")
	}
	if !caps.HandlesResponseBody {
		t.Error("expected HandlesResponseBody to be true")
	}
	if !caps.SupportsStreaming {
		t.Error("expected SupportsStreaming to be true")
	}
	if !caps.SupportsCancellation {
		t.Error("expected SupportsCancellation to be true")
	}
}

func TestAgentCapabilities_Clone(t *testing.T) {
	caps := NewAgentCapabilities().
		HandleRequestBody().
		WithMaxConcurrentRequests(50).
		WithFeature("feature1")

	clone := caps.Clone()

	// Verify values are copied
	if clone.HandlesRequestBody != caps.HandlesRequestBody {
		t.Error("clone HandlesRequestBody mismatch")
	}
	if clone.MaxConcurrentRequests == nil || *clone.MaxConcurrentRequests != 50 {
		t.Errorf("clone MaxConcurrentRequests mismatch: %v", clone.MaxConcurrentRequests)
	}
	if !clone.HasFeature("feature1") {
		t.Error("clone should have feature1")
	}

	// Modify original
	caps.WithFeature("feature2")
	*caps.MaxConcurrentRequests = 100

	// Clone should not be affected
	if clone.HasFeature("feature2") {
		t.Error("clone should not have feature2 after modifying original")
	}
	if *clone.MaxConcurrentRequests != 50 {
		t.Error("clone MaxConcurrentRequests should not be affected by original modification")
	}
}

func TestAgentCapabilities_WithoutCancellation(t *testing.T) {
	caps := NewAgentCapabilities().WithoutCancellation()

	if caps.SupportsCancellation {
		t.Error("expected SupportsCancellation to be false")
	}
}

func TestAgentCapabilities_WithFeatures(t *testing.T) {
	caps := NewAgentCapabilities().WithFeatures("f1", "f2", "f3")

	if len(caps.SupportedFeatures) != 3 {
		t.Errorf("expected 3 features, got %d", len(caps.SupportedFeatures))
	}
	if !caps.HasFeature("f1") || !caps.HasFeature("f2") || !caps.HasFeature("f3") {
		t.Error("expected all features to be present")
	}
	if caps.HasFeature("f4") {
		t.Error("expected HasFeature('f4') to be false")
	}
}
