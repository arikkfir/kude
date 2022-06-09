package pkg

import (
	"fmt"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type TargetingFilter struct {
	APIVersion    string `json:"apiVersion" yaml:"apiVersion"`
	Kind          string `json:"kind" yaml:"kind"`
	Namespace     string `json:"namespace" yaml:"namespace"`
	Name          string `json:"name" yaml:"name"`
	LabelSelector string `json:"labelSelector" yaml:"labelSelector"`
}

type singleResourceTargeting struct {
	includes []TargetingFilter
	excludes []TargetingFilter
}

func (s singleResourceTargeting) Filter(resource *yaml.RNode) (*yaml.RNode, error) {
	included := len(s.includes) == 0
	excluded := false
	for _, f := range s.includes {
		if f.APIVersion != "" && f.APIVersion != resource.GetApiVersion() {
			continue
		} else if f.Kind != "" && f.Kind != resource.GetKind() {
			continue
		} else if f.Namespace != "" && f.Namespace != resource.GetNamespace() {
			continue
		} else if f.Name != "" && f.Name != resource.GetName() {
			continue
		} else if f.LabelSelector != "" {
			if matches, err := resource.MatchesLabelSelector(f.LabelSelector); err != nil {
				return nil, fmt.Errorf("resource '%s/%s' failed matching labels selector '%s': %w", resource.GetNamespace(), resource.GetName(), f.LabelSelector, err)
			} else if !matches {
				continue
			}
		}
		included = true
	}
	for _, f := range s.excludes {
		if f.APIVersion != "" && f.APIVersion != resource.GetApiVersion() {
			continue
		} else if f.Kind != "" && f.Kind != resource.GetKind() {
			continue
		} else if f.Namespace != "" && f.Namespace != resource.GetNamespace() {
			continue
		} else if f.Name != "" && f.Name != resource.GetName() {
			continue
		} else if f.LabelSelector != "" {
			if matches, err := resource.MatchesLabelSelector(f.LabelSelector); err != nil {
				return nil, fmt.Errorf("resource '%s/%s' failed matching labels selector '%s': %w", resource.GetNamespace(), resource.GetName(), f.LabelSelector, err)
			} else if !matches {
				continue
			}
		}
		excluded = true
	}
	if included && !excluded {
		return resource, nil
	} else {
		return nil, nil
	}
}

func SingleResourceTargeting(includes, excludes []TargetingFilter) yaml.Filter {
	return singleResourceTargeting{includes, excludes}
}
