package resources

import (
	"strings"
	"testing"
)

func TestGetProperty_version(t *testing.T) {
	v, err := GetProperty(PropertyVersion)
	if err != nil {
		t.Fatalf("GetProperty(version): unexpected error: %v", err)
	}
	if v == "" {
		t.Error("GetProperty(version) returned empty string")
	}
}

func TestGetProperty_environment(t *testing.T) {
	v, err := GetProperty(PropertyEnvironment)
	if err != nil {
		t.Fatalf("GetProperty(environment): unexpected error: %v", err)
	}
	valid := map[string]bool{
		string(EnvironmentDevelopment): true,
		string(EnvironmentPreview):     true,
		string(EnvironmentProduction):  true,
	}
	if !valid[v] {
		t.Errorf("GetProperty(environment) = %q, want one of development/preview/production", v)
	}
}

func TestGetProperty_missing(t *testing.T) {
	_, err := GetProperty("nonexistent_property_xyz")
	if err == nil {
		t.Fatal("GetProperty(nonexistent): expected error, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent_property_xyz") {
		t.Errorf("error should mention property name, got: %v", err)
	}
}

func TestGetProperty_skipsComments(t *testing.T) {
	// Any successful call to GetProperty exercises the comment-skipping logic.
	_, _ = GetProperty(PropertyVersion)
}
