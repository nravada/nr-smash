package classifier

import (
	"testing"
)

func TestClassificationConstants(t *testing.T) {
	if Bug != "bug" {
		t.Errorf("Bug constant = %v, want 'bug'", Bug)
	}
	if Noise != "noise" {
		t.Errorf("Noise constant = %v, want 'noise'", Noise)
	}
}

func TestNewClassifier(t *testing.T) {
	c := NewClassifier("test-api-key")
	if c == nil {
		t.Fatal("NewClassifier() returned nil")
	}
	if c.apiKey != "test-api-key" {
		t.Errorf("NewClassifier() apiKey = %v, want 'test-api-key'", c.apiKey)
	}
	if c.cache == nil {
		t.Error("NewClassifier() cache is nil")
	}
}

// Note: Full integration tests with the Anthropic API would require
// a real API key and network access. Those tests would be better suited
// for an integration test suite that runs separately from unit tests.
