package sys

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode"

	"github.com/mitchellh/go-homedir"
)

// Sep is the OS path separator as a string.
const Sep = string(os.PathSeparator)

// ANSI terminal color codes.
const (
	colorYellow = "\033[33m"
	colorReset  = "\033[0m"
)

// Yellow wraps s in yellow ANSI color codes.
func Yellow(s string) string {
	return colorYellow + s + colorReset
}

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

// CopyFile copies the file at src to dest, creating parent directories as needed.
func CopyFile(src, dest string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	return os.WriteFile(dest, data, 0644)
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

// ExpandPath expands environment variables, a leading ~, and resolves relative
// paths to absolute using the current working directory.
func ExpandPath(input string) string {
	if input == "" {
		return input
	}

	input = os.ExpandEnv(input)
	input = strings.TrimSpace(input)
	input, _ = homedir.Expand(input)

	// On Windows, preserve POSIX-style absolute paths (for example,
	// "/usr/local/bin") rather than canonicalizing them into a drive-rooted
	// Windows path via filepath.Abs.
	if runtime.GOOS == "windows" && strings.HasPrefix(input, "/") {
		return input
	}

	if abs, err := filepath.Abs(input); err == nil {
		input = abs
	}
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

// LatestGitHubRelease queries the GitHub releases API for the given repo
// (e.g. "owner/repo") and returns the latest stable version without the leading "v".
// Returns an empty string if the request fails, times out, or no release exists.
func LatestGitHubRelease(repo string) string {
	return latestGitHubRelease("https://api.github.com", repo)
}

// LatestGitHubPreRelease returns the latest pre-release version for the given repo.
// Returns an empty string if none is found or the request fails.
func LatestGitHubPreRelease(repo string) string {
	return latestGitHubPreRelease("https://api.github.com", repo)
}

func latestGitHubRelease(baseURL, repo string) string {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(baseURL + "/repos/" + repo + "/releases/latest")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var result struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ""
	}
	return strings.TrimPrefix(result.TagName, "v")
}

func latestGitHubPreRelease(baseURL, repo string) string {
	client := &http.Client{Timeout: 2 * time.Second}
	for page := 1; page <= 10; page++ {
		url := fmt.Sprintf("%s/repos/%s/releases?per_page=10&page=%d", baseURL, repo, page)
		resp, err := client.Get(url)
		if err != nil {
			return ""
		}
		var releases []struct {
			TagName    string `json:"tag_name"`
			Prerelease bool   `json:"prerelease"`
		}
		err = json.NewDecoder(resp.Body).Decode(&releases)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK || err != nil || len(releases) == 0 {
			return ""
		}
		for _, r := range releases {
			if r.Prerelease {
				return strings.TrimPrefix(r.TagName, "v")
			}
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
