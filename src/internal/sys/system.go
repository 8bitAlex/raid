package sys

import (
	"fmt"
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
	os.MkdirAll(path.Dir(filePath), os.ModeDir | 0x1ED)

	file, err := os.Create(filePath)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	return file
}

func FileExists(path string) bool {
	if _, err := os.Open(path); err != nil {
		return false
	}
	return true
}
