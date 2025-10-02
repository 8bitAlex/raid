package utils

import "fmt"

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
