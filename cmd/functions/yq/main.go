package main

import (
	"context"
	"github.com/arikkfir/kude/pkg"
	"github.com/spf13/viper"
	"os"
	"os/exec"
)

func main() {
	pkg.Configure()

	expr := viper.GetString("expr")
	if expr == "" {
		panic("expr is required")
	}

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "yq", expr)
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		panic(err)
	}
}
