package pkg

import (
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
