package internal

import (
	"fmt"
	"os"
)

func MustGetwd() string {
	if pwd, err := os.Getwd(); err != nil {
		panic(fmt.Errorf("failed to get current working directory: %w", err))
	} else {
		return pwd
	}
}
