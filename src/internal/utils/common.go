package utils

import (
	"fmt"
	"os"
)

func HandleError(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
