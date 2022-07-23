package generate

import (
	"context"
	"fmt"
	. "github.com/arikkfir/kude/internal/stream/types"
	"gopkg.in/yaml.v3"
	"os"
)

func FromFile(path string) NodeGenerator {
	return func(ctx context.Context, target chan *yaml.Node) error {
		if f, err := os.Open(path); err != nil {
			return fmt.Errorf("failed to create file node generator: %w", err)
		} else {
			return FromReader(f)(ctx, target)
		}
	}
}
