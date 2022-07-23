package processing

import (
	"context"
	"fmt"
	"github.com/arikkfir/kude/internal"
	. "github.com/arikkfir/kude/internal/stream/types"
	"gopkg.in/yaml.v3"
)

func SetK8sResourceNamespace(namespace string) NodeProcessor {
	return func(ctx context.Context, n *yaml.Node) error {
		if n.Kind != yaml.MappingNode {
			return nil
		}

		metadataNode, err := internal.GetOrCreateChildKey(n, "metadata")
		if err != nil {
			return fmt.Errorf("failed to get or create metadata node: %w", err)
		}
		metadataNode.Kind = yaml.MappingNode
		metadataNode.Tag = "!!map"

		namespaceNode, err := internal.GetOrCreateChildKey(metadataNode, "namespace")
		if err != nil {
			return fmt.Errorf("failed to get or create metadata.namespace node: %w", err)
		}
		namespaceNode.Kind = yaml.ScalarNode
		namespaceNode.Tag = "!!str"
		namespaceNode.Value = namespace
		return nil
	}
}
