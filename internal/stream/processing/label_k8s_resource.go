package processing

import (
	"context"
	"fmt"
	"github.com/arikkfir/kude/internal"
	. "github.com/arikkfir/kude/internal/stream/types"
	"gopkg.in/yaml.v3"
	"strconv"
)

func LabelK8sResource(name string, value interface{}) NodeProcessor {
	return func(ctx context.Context, n *yaml.Node) error {
		if n.Kind != yaml.MappingNode {
			return nil
		}

		var tv, tag string
		switch v := value.(type) {
		case int:
			tv = strconv.Itoa(v)
			tag = "!!int"
		case int8:
			tv = strconv.Itoa(int(v))
			tag = "!!int"
		case int16:
			tv = strconv.Itoa(int(v))
			tag = "!!int"
		case int32:
			tv = strconv.Itoa(int(v))
			tag = "!!int"
		case int64:
			tv = strconv.Itoa(int(v))
			tag = "!!int"
		case string:
			tv = v
			tag = "!!str"
		case bool:
			tv = strconv.FormatBool(v)
			tag = "!!bool"
		default:
			panic(fmt.Sprintf("unsupported type %T", v))
		}

		metadataNode, err := internal.GetOrCreateChildKey(n, "metadata")
		if err != nil {
			return fmt.Errorf("failed to get or create metadata node: %w", err)
		}
		metadataNode.Kind = yaml.MappingNode
		metadataNode.Tag = "!!map"

		labelsNode, err := internal.GetOrCreateChildKey(metadataNode, "labels")
		if err != nil {
			return fmt.Errorf("failed to get or create metadata.labels node: %w", err)
		}
		labelsNode.Kind = yaml.MappingNode
		labelsNode.Tag = "!!map"

		valueNode, err := internal.GetOrCreateChildKey(labelsNode, name)
		if err != nil {
			return fmt.Errorf("failed to get or create metadata.labels node: %w", err)
		}
		valueNode.Kind = yaml.ScalarNode
		valueNode.Tag = tag
		valueNode.Value = tv
		return nil
	}
}
