package generate

import (
	"context"
	"errors"
	"fmt"
	. "github.com/arikkfir/kude/internal/stream/types"
	"gopkg.in/yaml.v3"
	"io"
)

func FromReader(r io.Reader) NodeGenerator {
	return func(_ context.Context, target chan *yaml.Node) error {
		decoder := yaml.NewDecoder(r)
		for {
			node := &yaml.Node{}
			if err := decoder.Decode(node); err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				} else {
					return fmt.Errorf("failed parsing input: %w", err)
				}
			} else {
				if node.Kind == yaml.DocumentNode {
					node = node.Content[0]
				}
				target <- node
			}
		}
	}
}
