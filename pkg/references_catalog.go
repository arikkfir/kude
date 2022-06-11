package kude

import (
	_ "embed"
	"fmt"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

//go:embed references_catalog.yaml
var rawReferenceTypesYAML string
var referencesCatalog map[v1.GroupVersionKind][]referencePoint

type referencePoint struct {
	Group   string `yaml:"group"`
	Version string `yaml:"version"`
	Kind    string `yaml:"kind"`
	Field   struct {
		NamePath []string `yaml:"namePath"`
		Type     struct {
			Group   string `yaml:"group"`
			Version string `yaml:"version"`
			Kind    string `yaml:"kind"`
		} `yaml:"type"`
	} `yaml:"field"`
}

func (r *referencePoint) resolve(resource *yaml.RNode, resourceNamespace string, renamedResources map[string]string) error {
	rns := make([]*yaml.Node, 0)
	pathMatcher := &yaml.PathMatcher{Path: r.Field.NamePath}
	err := resource.PipeE(
		pathMatcher,
		yaml.FilterFunc(func(object *yaml.RNode) (*yaml.RNode, error) {
			rns = append(rns, object.Content()[0])
			return object, nil
		}),
	)
	if err != nil {
		return fmt.Errorf("error finding name paths: %w", err)
	} else if len(rns) == 0 {
		return nil
	}

	var refFieldAPIVersion string
	if r.Field.Type.Group == "" {
		refFieldAPIVersion = r.Field.Type.Version
	} else {
		refFieldAPIVersion = r.Field.Type.Group + "/" + r.Field.Type.Version
	}

	for _, rn := range rns {
		namespace := resourceNamespace
		key := fmt.Sprintf("%s/%s/%s/%s", refFieldAPIVersion, r.Field.Type.Kind, namespace, rn.Value)
		if newName, ok := renamedResources[key]; ok {
			rn.SetString(newName)
		}
	}
	return nil
}

func init() {
	var rawRefs []referencePoint
	err := yaml.Unmarshal([]byte(rawReferenceTypesYAML), &rawRefs)
	if err != nil {
		panic(fmt.Errorf("error unmarshalling reference types: %w", err))
	}

	refTypes := make(map[v1.GroupVersionKind][]referencePoint)
	for _, rawRef := range rawRefs {
		gvk := v1.GroupVersionKind{Group: rawRef.Group, Version: rawRef.Version, Kind: rawRef.Kind}
		if refs, ok := refTypes[gvk]; ok {
			refTypes[gvk] = append(refs, rawRef)
		} else {
			refTypes[gvk] = []referencePoint{rawRef}
		}
	}
	referencesCatalog = refTypes
}
