package functions

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
}

func (f *YQ) Invoke(logger *log.Logger, pwd, _, _ string, r io.Reader, w io.Writer) error {
	if f.Expression == "" {
		return fmt.Errorf("expression is required")
	}

	cmd := exec.CommandContext(context.Background(), "yq", f.Expression)
	cmd.Stdin = r
	cmd.Stdout = w
	cmd.Stderr = os.Stderr
	cmd.Dir = pwd
	logger.Printf("Starting process: %v", cmd.Args)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run process: %w", err)
	}
	return nil
}
