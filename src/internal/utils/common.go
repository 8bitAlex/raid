package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

// MergeErr combines multiple errors into a single error, skipping nil entries.
// Returns nil if all errors are nil or the slice is empty.
func MergeErr(errs []error) error {
	var msgs []string
	for _, err := range errs {
		if err != nil {
			msgs = append(msgs, err.Error())
		}
	}
	if len(msgs) == 0 {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(msgs, ", "))
}

// YAMLToJSON converts the first YAML document in file to JSON.
// Returns an error if the reader contains more than one YAML document, as
// only the first would be validated and silently ignoring later documents
// can mask configuration mistakes.
func YAMLToJSON(file io.Reader) ([]byte, error) {
	dec := yaml.NewDecoder(file)
	var data interface{}
	if err := dec.Decode(&data); err != nil {
		return nil, err
	}
	var extra interface{}
	if err := dec.Decode(&extra); err == nil {
		return nil, fmt.Errorf("multi-document YAML is not supported for schema validation")
	}
	return json.Marshal(data)
}
