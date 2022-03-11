package pkg

import (
	"fmt"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type pipelineFanout struct {
	processor yaml.Filter
}

func (s pipelineFanout) Filter(resources []*yaml.RNode) ([]*yaml.RNode, error) {
	for i := range resources {
		resource := resources[i]
		_, err := resource.Pipe(s.processor)
		if err != nil {
			return nil, err
		}
	}
	return resources, nil
}

func Fanout(processor yaml.Filter) kio.Filter {
	return pipelineFanout{processor: processor}
}

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
