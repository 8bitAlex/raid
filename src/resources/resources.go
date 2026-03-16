// Package resources exposes embedded application resources.
package resources

import (
	_ "embed"
	"fmt"
	"strings"
)

// Property identifies a key in app.properties.
type Property string

const (
	PropertyVersion     Property = "version"
	PropertyEnvironment Property = "environment"
)

// Environment identifies the runtime environment the binary was built for.
type Environment string

const (
	EnvironmentDevelopment Environment = "development"
	EnvironmentPreview     Environment = "preview"
	EnvironmentProduction  Environment = "production"
)

//go:embed app.properties
var appProperties []byte

// GetProperty returns the value of the named property from app.properties.
// Returns an error if the property is not found.
func GetProperty(name Property) (string, error) {
	for _, line := range strings.Split(string(appProperties), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if ok && strings.TrimSpace(key) == string(name) {
			return strings.TrimSpace(value), nil
		}
	}
	return "", fmt.Errorf("property %q not found in app.properties", name)
}
