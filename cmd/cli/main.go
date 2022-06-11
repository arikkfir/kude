package main

import (
	"github.com/arikkfir/kude/cmd/cli/commands/root"
	"log"
	"os"
)

func init() {
	log.SetFlags(0)
}

func main() {
	if err := root.Cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
