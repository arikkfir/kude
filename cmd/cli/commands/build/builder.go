package build

import (
	"context"
	"fmt"
	kude "github.com/arikkfir/kude/pkg"
	"io"
	"log"
	"path/filepath"
)

func build(pwd string, logger *log.Logger, writer io.Writer) error {
	pwd, err := filepath.Abs(pwd)
	if err != nil {
		return fmt.Errorf("failed converting path '%s' to an absolute path: %w", pwd, err)
	}

	ctx := context.Background()

	pipeline, err := kude.NewPipeline(pwd)
	if err != nil {
		return fmt.Errorf("failed to create pipeline: %w", err)
	}

	execution, err := kude.NewExecution(pipeline, logger)
	if err != nil {
		return fmt.Errorf("failed to create pipeline execution: %w", err)
	}

	return execution.ExecuteToWriter(ctx, writer)
}
