package sys

import (
	"log"
	"os"
	"path"
	"strings"

	"github.com/mitchellh/go-homedir"
)

// Path Separator as a string
const Sep = string(os.PathSeparator)

func GetHomeDir() string {
	home, err := homedir.Dir()
	if err != nil {
		log.Fatalf("Failed to get home directory: %v", err)
	}
	return home
}

func CreateFile(filePath string) *os.File {
	pathEx := ExpandPath(filePath)
	os.MkdirAll(path.Dir(pathEx), os.ModeDir|0755)

	file, err := os.Create(pathEx)
	if err != nil {
		log.Fatalf("Failed to create file '%s': %v", pathEx, err)
	}
	return file
}

func FileExists(path string) bool {
	path = ExpandPath(path)
	if _, err := os.Open(path); err != nil {
		return false
	}
	return true
}

func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~") {
		return strings.Replace(path, "~", GetHomeDir(), 1)
	}
	return os.ExpandEnv(path)
}
