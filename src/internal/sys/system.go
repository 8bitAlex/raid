package sys

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"

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
// Permission errors are treated as the path existing to avoid silently
// overwriting or recreating inaccessible files.
func FileExists(path string) bool {
	path = ExpandPath(path)
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	return os.IsPermission(err)
}

// Expand expands environment variables in input without tokenizing, preserving
// quoting and spacing. Use ExpandPath for file system paths that also need ~ expansion.
func Expand(input string) string {
	return os.ExpandEnv(input)
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

// ValidateFileName reports whether name is safe to use as a filename across
// common operating systems. It rejects empty names, path separators, shell
// special characters, control characters, and names composed only of dots or spaces.
func ValidateFileName(name string) error {
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	for _, r := range name {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' ||
			r == '"' || r == '<' || r == '>' || r == '|' || r == '\x00' {
			return fmt.Errorf("contains invalid character %q", r)
		}
		if unicode.IsControl(r) {
			return fmt.Errorf("contains control character")
		}
	}
	if strings.Trim(name, ". ") == "" {
		return fmt.Errorf("cannot consist only of dots or spaces")
	}
	return nil
}

// ReadLine prints prompt to stdout and returns the trimmed line read from reader.
func ReadLine(reader *bufio.Reader, prompt string) string {
	fmt.Print(prompt)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}

// ReadYesNo prompts the user and returns true if they answer "y" or "yes" (case-insensitive).
func ReadYesNo(reader *bufio.Reader, prompt string) bool {
	answer := ReadLine(reader, prompt)
	return strings.EqualFold(answer, "y") || strings.EqualFold(answer, "yes")
}

// DetectGitDefaultBranch queries the remote at url to find its default branch without cloning.
// Returns an empty string if the remote is unreachable or the branch cannot be determined.
func DetectGitDefaultBranch(url string) string {
	out, err := exec.Command("git", "ls-remote", "--symref", url, "HEAD").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		// Format: "ref: refs/heads/<branch>\tHEAD"
		if strings.HasPrefix(line, "ref: refs/heads/") {
			return strings.TrimPrefix(strings.SplitN(line, "\t", 2)[0], "ref: refs/heads/")
		}
	}
	return ""
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
