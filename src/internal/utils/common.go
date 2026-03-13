package utils

import (
	"encoding/json"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

func MergeErr(errs []error) error {
	var result string
	for _, err := range errs {
		if len(result) == 0 {
			result = err.Error()
		} else {
			result = result + ", " + err.Error()
		}
	}
	return fmt.Errorf("%s", result)
}

func YAMLToJSON(file io.Reader) ([]byte, error) {
	var data interface{}
	if err := yaml.NewDecoder(file).Decode(&data); err != nil {
		return nil, err
	}
	return json.Marshal(data)
}
