package stream

import (
	"context"
	"fmt"
	. "github.com/arikkfir/kude/internal/stream/types"
	"gopkg.in/yaml.v3"
	"sync"
)

const (
	inputNodesBufferSize   = 1000
	handlerNodesBufferSize = 1000
)

type Stream interface {
	Generate(generator NodeGenerator) Stream
	Transform(transformer NodeTransformer) Stream
	Process(processor NodeProcessor) Stream
	Sink(sink NodeSink) Stream
	Execute(ctx context.Context) error
}

func NewStream() Stream {
	return &stream{}
}

// stream represents a stream of nodes, where nodes are originating from a set of input generators, and are then
// processed or transformed until finally placed into a set of sinks.
type stream struct {
	started    bool
	generators []NodeGenerator
	handlers   []NodeTransformer
	sinks      []NodeSink
}

// Generate adds another node generator to the stream.
func (p *stream) Generate(generator NodeGenerator) Stream {
	if p.started {
		panic("stream already started")
	}
	p.generators = append(p.generators, generator)
	return p
}

// Transform adds a node transformer to the stream.
func (p *stream) Transform(transformer NodeTransformer) Stream {
	if p.started {
		panic("stream already started")
	}
	p.handlers = append(p.handlers, transformer)
	return p
}

// Process adds a node processor to the stream.
func (p *stream) Process(processor NodeProcessor) Stream {
	if p.started {
		panic("stream already started")
	}
	p.handlers = append(p.handlers, func(ctx context.Context, node *yaml.Node, output chan *yaml.Node) error {
		if err := processor(ctx, node); err != nil {
			return err
		} else {
			output <- node
			return nil
		}
	})
	return p
}

// Sink adds a sink to the stream.
func (p *stream) Sink(sink NodeSink) Stream {
	if p.started {
		panic("stream already started")
	}
	p.sinks = append(p.sinks, sink)
	return p
}

// Execute executes the stream.
func (p *stream) Execute(ctx context.Context) error {
	p.started = true
	nodes := make(chan *yaml.Node, inputNodesBufferSize)
	exitCh := make(chan error, 1000)
	wg := &sync.WaitGroup{}

	////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// GENERATE NODES
	// --------------
	// Invoke each generator in a separate goroutine, and let it push nodes into our nodes channel.
	////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	gwg := &sync.WaitGroup{}
	for index, input := range p.generators {
		gwg.Add(1)
		go func(index int, input NodeGenerator) {
			defer gwg.Done()
			if err := input(ctx, nodes); err != nil {
				exitCh <- fmt.Errorf("generator %d failed: %w", index, err)
			}
		}(index, input)
	}

	////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// CLOSE NODES CHANNEL WHEN INPUTS ARE DONE
	// ----------------------------------------
	// After all input functions goroutines exit, we should close the nodes channel, to make sure downstream readers
	// can finish.
	////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	wg.Add(1)
	go func() {
		defer wg.Done()
		gwg.Wait()
		close(nodes)
	}()

	////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// PROCESS NODES
	// -------------
	// Create a chain of handlers, where the first handler receives nodes from generators, and each subsequent
	// handler receives nodes from its predecessor.
	////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	input := nodes
	for handlerIndex, handler := range p.handlers {
		output := make(chan *yaml.Node, handlerNodesBufferSize)
		wg.Add(1)
		go func(i int, h NodeTransformer, input chan *yaml.Node, output chan *yaml.Node) {
			defer wg.Done()
			defer close(output)
			for {
				node, ok := <-input
				if !ok {
					return
				}
				if err := h(ctx, node, output); err != nil {
					exitCh <- fmt.Errorf("handler %d failed: %w", i, err)
					return
				}
			}
		}(handlerIndex, handler, input, output)
		input = output // next handler's input will be output of this handler
	}

	////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// SINK NODES
	// -------------
	// Send output nodes from last handler to sinks.
	////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			node, ok := <-input
			if !ok {
				break
			} else if node == nil {
				panic(fmt.Errorf("unexpected nil node received for sinking"))
			}
			for i, sink := range p.sinks {
				if err := sink.Process(ctx, node); err != nil {
					exitCh <- fmt.Errorf("sink %d failed: %w", i, err)
					return
				}
			}
		}
	}()

	////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// WAIT FOR ALL GOROUTINES
	// -----------------------
	// Wait for all goroutines to finish (even if they fail, they should finish).
	////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	wg.Wait()

	////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// CLOSE SINKS
	////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	for i, sink := range p.sinks {
		err := sink.Close()
		if err != nil {
			return fmt.Errorf("sink %d failed to close: %w", i, err)
		}
	}

	////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// CONCLUDE
	// --------
	// Check if an error occurred or not. If one did, that error is returned.
	////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	select {
	case err := <-exitCh:
		return err
	default:
		return nil
	}
}
