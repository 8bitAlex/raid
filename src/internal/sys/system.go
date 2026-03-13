package sys

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mitchellh/go-homedir"
)

// Sep is the OS path separator as a string.
const Sep = string(os.PathSeparator)

// Platform identifies the host operating system.
type Platform string

const (
	Windows Platform = "windows"
	Linux   Platform = "linux"
	Darwin  Platform = "darwin"
	Other   Platform = "other"
)

// GetHomeDir returns the current user's home directory, fatally logging on failure.
func GetHomeDir() string {
	home, err := homedir.Dir()
	if err != nil {
		log.Fatalf("Failed to get home directory: %v", err)
	}
	return home
}

// CreateFile opens or creates the file at filePath for reading and writing, creating parent directories as needed.
func CreateFile(filePath string) (*os.File, error) {
	pathEx := ExpandPath(filePath)
	if err := os.MkdirAll(filepath.Dir(pathEx), 0755); err != nil {
		return nil, fmt.Errorf("failed to create directories for '%s': %w", filePath, err)
	}
	return os.OpenFile(pathEx, os.O_RDWR|os.O_CREATE, 0644)
}

// FileExists reports whether the file or directory at path exists.
func FileExists(path string) bool {
	path = ExpandPath(path)
	if _, err := os.Stat(path); err != nil {
		return false
	}
	return true
}

// Expand expands environment variables and home directory references in each whitespace-delimited
// token of input, then rejoins them with spaces.
func Expand(input string) string {
	if input == "" {
		return input
	}
	parts := SplitInput(input)
	for i, p := range parts {
		parts[i] = ExpandPath(p)
	}
	return strings.Join(parts, " ")
}

// ExpandPath expands environment variables and a leading ~ in the given path.
func ExpandPath(input string) string {
	if input == "" {
		return input
	}

	input = os.ExpandEnv(input)
	input = strings.TrimSpace(input)
	input, _ = homedir.Expand(input)
	return input
}

// SplitInput splits a command string into tokens, respecting double-quoted segments.
// todo: replace with a proper shell-quoting parser
func SplitInput(input string) []string {
	var out []string
	var b strings.Builder
	inQuote := false
	skipSpace := false

	for _, ch := range input {
		switch {
		case ch == '"':
			inQuote = !inQuote
			if !inQuote && b.Len() > 0 {
				out = append(out, b.String())
				b.Reset()
			}
			skipSpace = false
		case ch == ' ' && !inQuote:
			if !skipSpace && b.Len() > 0 {
				out = append(out, b.String())
				b.Reset()
			}
			skipSpace = true
		default:
			skipSpace = false
			b.WriteRune(ch)
		}
	}
	if b.Len() > 0 {
		out = append(out, b.String())
	}
	return out
}

// GetPlatform returns the current operating system as a Platform value.
func GetPlatform() Platform {
	switch runtime.GOOS {
	case "windows":
		return Windows
	case "darwin":
		return Darwin
	case "linux":
		return Linux
	default:
		return Other
	}
}
