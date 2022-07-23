package processing

import (
	"context"
	"fmt"
	. "github.com/arikkfir/kude/internal/stream/types"
	"gopkg.in/yaml.v3"
	"io"
)

func Print(w io.Writer) NodeProcessor {
	return func(ctx context.Context, n *yaml.Node) error {
		encoder := yaml.NewEncoder(w)
		encoder.SetIndent(2)
		if err := encoder.Encode(n); err != nil {
			return fmt.Errorf("failed encoding node: %w", err)
		}
		return nil
	}
}
