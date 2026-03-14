package utils

import (
	"errors"
	"strings"
	"testing"
)

func TestMergeErr_singleError(t *testing.T) {
	err := MergeErr([]error{errors.New("only error")})
	if err == nil {
		t.Fatal("MergeErr() returned nil, want error")
	}
	if err.Error() != "only error" {
		t.Errorf("MergeErr() = %q, want %q", err.Error(), "only error")
	}
}

func TestMergeErr_multipleErrors(t *testing.T) {
	err := MergeErr([]error{errors.New("first"), errors.New("second"), errors.New("third")})
	if err == nil {
		t.Fatal("MergeErr() returned nil, want error")
	}
	msg := err.Error()
	for _, sub := range []string{"first", "second", "third"} {
		if !strings.Contains(msg, sub) {
			t.Errorf("MergeErr() = %q, missing %q", msg, sub)
		}
	}
}

func TestYAMLToJSON_validYAML(t *testing.T) {
	yaml := strings.NewReader("name: test\nvalue: 42")
	result, err := YAMLToJSON(yaml)
	if err != nil {
		t.Fatalf("YAMLToJSON() error: %v", err)
	}
	got := string(result)
	if !strings.Contains(got, `"name"`) || !strings.Contains(got, "test") {
		t.Errorf("YAMLToJSON() = %q, expected JSON with name and test", got)
	}
}

func TestYAMLToJSON_invalidYAML(t *testing.T) {
	invalid := strings.NewReader("key: [unclosed")
	_, err := YAMLToJSON(invalid)
	if err == nil {
		t.Fatal("YAMLToJSON() expected error for invalid YAML, got nil")
	}
}
