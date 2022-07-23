package processing

import (
	"context"
	"github.com/arikkfir/kude/internal"
	. "github.com/arikkfir/kude/internal/stream/types"
	"gopkg.in/yaml.v3"
)

func AnnotateK8sResource(name string, value interface{}) NodeProcessor {
	return func(ctx context.Context, n *yaml.Node) error {
		return internal.SetAnnotation(n, name, value)
	}
}
