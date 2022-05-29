package internal

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
)

// TODO: download YQ into temp dir instead of bundling it inside the Docker image

type YQ struct {
	Expression string `json:"expression" yaml:"expression"`
	logger     *log.Logger
	pwd        string
}

func (f *YQ) Configure(logger *log.Logger, pwd, _, _ string) error {
	f.logger = logger
	f.pwd = pwd
	return nil
}

func (f *YQ) Invoke(r io.Reader, w io.Writer) error {
	if f.Expression == "" {
		return fmt.Errorf("expression is required")
	}

	cmd := exec.CommandContext(context.Background(), "yq", f.Expression)
	cmd.Stdin = r
	cmd.Stdout = w
	cmd.Stderr = os.Stderr
	cmd.Dir = f.pwd
	f.logger.Printf("Starting process: %v", cmd.Args)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run process: %w", err)
	}
	return nil
}
