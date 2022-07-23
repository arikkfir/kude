package processing

import (
	"context"
	"fmt"
	. "github.com/arikkfir/kude/internal/stream/types"
	"gopkg.in/yaml.v3"
	"sync"
)

const (
	handlerNodesBufferSize = 100
)

func Tee(transformers ...NodeTransformer) NodeProcessor {
	return func(ctx context.Context, n *yaml.Node) error {
		wg := &sync.WaitGroup{}
		exitCh := make(chan error, 1000) // TODO: adjust error channel size

		input := make(chan *yaml.Node, 1)
		input <- n
		close(input)

		// Create a chain of handlers, where the first handler receives nodes from generators, and each subsequent
		// handler receives nodes from its predecessor.
		for transformerIndex, transformer := range transformers {
			output := make(chan *yaml.Node, handlerNodesBufferSize)
			wg.Add(1)
			go func(i int, t NodeTransformer, input chan *yaml.Node, output chan *yaml.Node) {
				defer wg.Done()
				defer close(output)
				for {
					node, ok := <-input
					if !ok {
						return
					}
					if err := t(ctx, node, output); err != nil {
						exitCh <- fmt.Errorf("transformer %d failed: %w", i, err)
						return
					}
				}
			}(transformerIndex, transformer, input, output)
			input = output // next transformer's input will be output of this transformer
		}

		// Wait for all goroutines to finish (even if they fail, they should finish) and then check if an error occurred
		wg.Wait()
		select {
		case err := <-exitCh:
			return err
		default:
			return nil
		}
	}
}
