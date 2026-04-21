package config

import (
	"strings"
	"testing"
)

func TestWithLoggerCapturesWarning(t *testing.T) {
	// Config without a `version` field triggers the "assuming version" warning.
	raw := []byte(`database:
  dialect: postgres
tables: {}
`)
	var captured []string
	cfg, err := Unmarshal(raw, WithLogger(func(msg string) {
		captured = append(captured, msg)
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Version != CurrentVersion {
		t.Fatalf("version = %d, want %d", cfg.Version, CurrentVersion)
	}
	if len(captured) != 1 {
		t.Fatalf("logger invocations = %d, want 1; got %v", len(captured), captured)
	}
	if !strings.Contains(captured[0], "config has no version field") {
		t.Fatalf("unexpected message: %q", captured[0])
	}
}

func TestWithLoggerNotCalledForVersionedConfig(t *testing.T) {
	raw := []byte(`version: 1
database:
  dialect: postgres
tables: {}
`)
	var captured []string
	if _, err := Unmarshal(raw, WithLogger(func(msg string) {
		captured = append(captured, msg)
	})); err != nil {
		t.Fatal(err)
	}
	if len(captured) != 0 {
		t.Fatalf("expected no warnings, got %v", captured)
	}
}
