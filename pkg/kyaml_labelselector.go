package pkg

import (
	"fmt"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type singleResourceLabelSelector struct {
	selector string
}

func (s singleResourceLabelSelector) Filter(resource *yaml.RNode) (*yaml.RNode, error) {
	if matches, err := resource.MatchesLabelSelector(s.selector); err != nil {
		return nil, fmt.Errorf("resource '%s/%s' failed matching labels selector '%s': %w", resource.GetNamespace(), resource.GetName(), s.selector, err)
	} else if matches {
		return resource, nil
	} else {
		return nil, nil
	}
}

func SingleResourceLabelSelector(selector string) yaml.Filter {
	return singleResourceLabelSelector{selector}
}

type multiResourceLabelSelector struct {
	selector string
}

func (s multiResourceLabelSelector) Filter(resources []*yaml.RNode) ([]*yaml.RNode, error) {
	result := make([]*yaml.RNode, 0)
	for _, resource := range resources {
		if matches, err := resource.MatchesLabelSelector(s.selector); err != nil {
			return nil, fmt.Errorf("resource '%s/%s' failed matching labels selector '%s': %w", resource.GetNamespace(), resource.GetName(), s.selector, err)
		} else if matches {
			result = append(result, resource)
		}
	}
	return result, nil
}

func MultiResourceLabelSelector(selector string) kio.Filter {
	return multiResourceLabelSelector{selector}
}
