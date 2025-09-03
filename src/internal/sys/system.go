package sys

import (
	"fmt"
	"log"
	"os"
	"path"

	"github.com/mitchellh/go-homedir"
)

// Path Separator as a string
const Sep = string(os.PathSeparator)

func GetHomeDir() string {
	home, err := homedir.Dir()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	return home
}

func CreateFile(filePath string) *os.File {
	pathEx := os.ExpandEnv(filePath)
	os.MkdirAll(path.Dir(pathEx), os.ModeDir|0x1ED)

	file, err := os.Create(pathEx)
	if err != nil {
		log.Fatalf("Failed to create file '%s': %v", pathEx, err)
	}
	return file
}

func FileExists(path string) bool {
	path = os.ExpandEnv(path)
	if _, err := os.Open(path); err != nil {
		return false
	}
	return true
}
