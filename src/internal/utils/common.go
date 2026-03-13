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

func YAMLToJSON(file io.Reader) ([]byte, error) {
	var data interface{}
	if err := yaml.NewDecoder(file).Decode(&data); err != nil {
		return nil, err
	}
	return json.Marshal(data)
}
