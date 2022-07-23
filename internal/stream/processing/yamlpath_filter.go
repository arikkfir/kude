package processing

import (
	"context"
	"fmt"
	. "github.com/arikkfir/kude/internal/stream/types"
	"github.com/vmware-labs/yaml-jsonpath/pkg/yamlpath"
	"gopkg.in/yaml.v3"
)

func YAMLPathFilter(expr string) NodeTransformer {
	path, err := yamlpath.NewPath(expr)
	if err != nil {
		panic(fmt.Errorf("failed compiling YAML path '%s': %w", expr, err))
	}
	return func(ctx context.Context, n *yaml.Node, c chan *yaml.Node) error {
		if matches, err := path.Find(n); err != nil {
			return fmt.Errorf("YAML path filter '%s' failed: %w", expr, err)
		} else {
			for _, node := range matches {
				c <- node
			}
			return nil
		}
	}
}
