package pkg

import (
	"fmt"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type generator struct {
	generator GeneratorFunc
}

type GeneratorFunc func() ([]*yaml.RNode, error)

func (g generator) Filter(resources []*yaml.RNode) ([]*yaml.RNode, error) {
	generated, err := g.generator()
	if err != nil {
		return nil, fmt.Errorf("failed generating nodes: %w", err)
	}
	resources = append(resources, generated...)
	return resources, nil
}

func Generate(generatorFunc GeneratorFunc) kio.Filter {
	return generator{generator: generatorFunc}
}
