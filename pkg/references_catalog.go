package kude

import (
	_ "embed"
	"fmt"
	"github.com/arikkfir/kyaml/pkg"
	"github.com/vmware-labs/yaml-jsonpath/pkg/yamlpath"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//go:embed references_catalog.yaml
var rawReferenceTypesYAML string
var referencesCatalog map[v1.GroupVersionKind][]referencePoint

type referencePoint struct {
	Group   string `yaml:"group"`
	Version string `yaml:"version"`
	Kind    string `yaml:"kind"`
	Field   struct {
		path *yamlpath.Path `yaml:"-"`
		Path string         `yaml:"path"`
		Type struct {
			Group   string `yaml:"group"`
			Version string `yaml:"version"`
			Kind    string `yaml:"kind"`
		} `yaml:"type"`
	} `yaml:"field"`
}

func (r *referencePoint) resolve(rn *kyaml.RNode, renamedResources map[string]string) error {
	matches, err := r.Field.path.Find(rn.N)
	if err != nil {
		return fmt.Errorf("failed invoking YAML path '%s': %w", r.Field.Path, err)
	} else if len(matches) == 0 {
		return nil
	}

	var refFieldAPIVersion string
	if r.Field.Type.Group == "" {
		refFieldAPIVersion = r.Field.Type.Version
	} else {
		refFieldAPIVersion = r.Field.Type.Group + "/" + r.Field.Type.Version
	}

	namespace, err := rn.GetNamespace()
	if err != nil {
		return fmt.Errorf("failed getting namespace: %w", err)
	}
	for _, match := range matches {
		if match.Value != "" {
			key := fmt.Sprintf("%s/%s/%s/%s", refFieldAPIVersion, r.Field.Type.Kind, namespace, match.Value)
			if newName, ok := renamedResources[key]; ok {
				match.SetString(newName)
			}
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
		if path, err := yamlpath.NewPath(rawRef.Field.Path); err != nil {
			panic(fmt.Errorf("failed compiling YAML path: %w", err))
		} else {
			rawRef.Field.path = path
		}
		gvk := v1.GroupVersionKind{Group: rawRef.Group, Version: rawRef.Version, Kind: rawRef.Kind}
		if refs, ok := refTypes[gvk]; ok {
			refTypes[gvk] = append(refs, rawRef)
		} else {
			refTypes[gvk] = []referencePoint{rawRef}
		}
	}
	referencesCatalog = refTypes
}
