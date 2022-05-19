package pkg

import (
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type fanout struct {
	processor yaml.Filter
}

func (s fanout) Filter(resources []*yaml.RNode) ([]*yaml.RNode, error) {
	for i := range resources {
		resource := resources[i]
		_, err := resource.Pipe(s.processor)
		if err != nil {
			return nil, err
		}
	}
	return resources, nil
}

// Fanout adapters a single-resource filter to a kio.Filter interface which is suitable for a pipeline.
//
// TODO: replace with "kio.FilterAll" function
func Fanout(processor yaml.Filter) kio.Filter {
	return fanout{processor: processor}
}
