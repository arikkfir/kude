package main

import (
	"github.com/arikkfir/kude/cmd/cli/commands"
	"log"
	"os"
)

func init() {
	log.SetFlags(0)
}

func main() {
	err := commands.RootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
