package seedapi

import "testing"

func TestSetupStepZeroValue(t *testing.T) {
	var s SetupStep
	if s.Kind != "" {
		t.Fatalf("zero Kind = %q, want empty", s.Kind)
	}
	if s.Required {
		t.Fatal("zero Required must be false")
	}
	if s.Element != nil {
		t.Fatal("zero Element must be nil")
	}
}
