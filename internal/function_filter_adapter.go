package internal

import (
	"fmt"
	"github.com/arikkfir/kude/pkg"
	"log"
	"os"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type FunctionFilterAdapter struct {
	Logger *log.Logger
	Name   string
	Target pkg.Function
}

func (a *FunctionFilterAdapter) Filter(rns []*yaml.RNode) ([]*yaml.RNode, error) {
	a.Logger.Printf("Invoking '%s'", a.Name)

	r1, w1, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create input pipe: %w", err)
	}

	nodeWriter := kio.ByteWriter{Writer: w1, KeepReaderAnnotations: true}
	if err := nodeWriter.Write(rns); err != nil {
		return nil, fmt.Errorf("failed marshalling nodes: %w", err)
	}
	w1.Close()

	r2, w2, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create output pipe: %w", err)
	}

	if err := a.Target.Invoke(r1, w2); err != nil {
		return nil, fmt.Errorf("failed invoking function: %w", err)
	}
	w2.Close()

	nodeReader := kio.ByteReader{Reader: r2}
	nodes, err := nodeReader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed unmarshalling nodes: %w", err)
	}

	return nodes, nil
}
