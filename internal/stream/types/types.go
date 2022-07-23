package types

import (
	"context"
	"gopkg.in/yaml.v3"
	"io"
)

// NodeGenerator is a function that generates nodes for the stream.
type NodeGenerator func(context.Context, chan *yaml.Node) error

// NodeProcessor is a function that processes individual nodes.
type NodeProcessor func(ctx context.Context, node *yaml.Node) error

// NodeTransformer is a function that receives a node, and transforms it into zero or more subsequent nodes.
type NodeTransformer func(ctx context.Context, node *yaml.Node, output chan *yaml.Node) error

// NodeSink is a final target for the pipeline to dump nodes into.
type NodeSink interface {
	Process(ctx context.Context, node *yaml.Node) error
	io.Closer
}

func NodeTransformerOf(p NodeProcessor) NodeTransformer {
	return func(ctx context.Context, node *yaml.Node, output chan *yaml.Node) error {
		if err := p(ctx, node); err != nil {
			return err
		} else {
			output <- node
			return nil
		}
	}
}
