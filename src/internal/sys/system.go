package sys

import (
	"log"
	"os"
	"path"
	"runtime"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/thoas/go-funk"
)

// Path Separator as a string
const Sep = string(os.PathSeparator)

type Platform string

const (
	Windows Platform = "windows"
	Linux   Platform = "linux"
	Darwin  Platform = "darwin"
	Other   Platform = "other"
)

func GetHomeDir() string {
	home, err := homedir.Dir()
	if err != nil {
		log.Fatalf("Failed to get home directory: %v", err)
	}
	return home
}

func CreateFile(filePath string) (*os.File, error) {
	pathEx := ExpandPath(filePath)
	if FileExists(pathEx) {
		return os.Open(pathEx)
	}

	os.MkdirAll(path.Dir(pathEx), os.ModeDir|0755)
	return os.Create(pathEx)
}

func FileExists(path string) bool {
	path = ExpandPath(path)
	if _, err := os.Stat(path); err != nil {
		return false
	}
	return true
}

func Expand(input string) string {
	if input == "" {
		return input
	}

	i := funk.Map(SplitInput(input), func(x string) string {
		return ExpandPath(x)
	}).([]string)
	return strings.Join(i, " ")
}

func ExpandPath(input string) string {
	if input == "" {
		return input
	}

	input = os.ExpandEnv(input)
	input = strings.TrimSpace(input)
	input, _ = homedir.Expand(input)
	return input
}

// this is a mess — todo
func SplitInput(input string) []string {
	out := []string{}
	quote := false
	ignore := false
	b := strings.Builder{}
	for _, s := range input {
		if s == '"' {
			quote = !quote
		}
		if s == ' ' && !quote {
			if ignore {
				continue
			}
			ignore = true
			if b.Len() > 0 {
				out = append(out, b.String())
				b.Reset()
			}
		} else if s == '"' && quote {
			ignore = false
			if b.Len() > 0 {
				out = append(out, b.String())
				b.Reset()
			}
			b.WriteRune(s)
		} else {
			ignore = false
			b.WriteRune(s)
		}
	}
	out = append(out, b.String())
	return out
}

// todo platform dependent files?
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
