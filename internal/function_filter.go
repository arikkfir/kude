package internal

import (
	_ "embed"
	"fmt"
	"github.com/arikkfir/kude/pkg"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/kustomize/kyaml/yaml"
	"strings"
)

//go:embed reference_types.yaml
var rawReferenceTypesYAML string
var referenceTypes map[v1.GroupVersionKind][]referenceType

func init() {
	var rawRefs []referenceType
	err := yaml.Unmarshal([]byte(rawReferenceTypesYAML), &rawRefs)
	if err != nil {
		panic(fmt.Errorf("error unmarshalling reference types: %w", err))
	}

	refTypes := make(map[v1.GroupVersionKind][]referenceType)
	for _, rawRef := range rawRefs {
		gvk := v1.GroupVersionKind{Group: rawRef.Group, Version: rawRef.Version, Kind: rawRef.Kind}
		if refs, ok := refTypes[gvk]; ok {
			refTypes[gvk] = append(refs, rawRef)
		} else {
			refTypes[gvk] = []referenceType{rawRef}
		}
	}
	referenceTypes = refTypes
}

type ResolverFilter struct{}

func (r *ResolverFilter) Filter(rns []*yaml.RNode) ([]*yaml.RNode, error) {

	// Build a map of resources that should be replaced
	resourceMappings, err := r.collectRenamedResources(rns)
	if err != nil {
		return nil, fmt.Errorf("error collecting renamed resources: %w", err)
	}

	// Iterate resources and replace references
	for _, resource := range rns {
		resourceKind := resource.GetKind()
		resourceNamespace := resource.GetNamespace()

		resourceAPIVersion := resource.GetApiVersion()
		var resourceAPIGroup, resourceAPIGroupVersion string
		if lastSlashIndex := strings.LastIndex(resourceAPIVersion, "/"); lastSlashIndex < 0 {
			resourceAPIGroup = ""
			resourceAPIGroupVersion = resourceAPIVersion
		} else {
			resourceAPIGroup = resourceAPIVersion[0:lastSlashIndex]
			resourceAPIGroupVersion = resourceAPIVersion[lastSlashIndex+1:]
		}

		gvk := v1.GroupVersionKind{Group: resourceAPIGroup, Version: resourceAPIGroupVersion, Kind: resourceKind}
		if refTypes, ok := referenceTypes[gvk]; ok {
			for _, refType := range refTypes {
				err := refType.resolve(resource, resourceNamespace, resourceMappings)
				if err != nil {
					return nil, fmt.Errorf("error resolving reference: %w", err)
				}
			}
		}
	}
	return rns, nil
}

func (r *ResolverFilter) collectRenamedResources(resources []*yaml.RNode) (map[string]string, error) {
	resourceMappings := make(map[string]string)
	for _, resource := range resources {
		annotations := resource.GetAnnotations()
		if annotations != nil {
			if previousName, ok := annotations[pkg.PreviousNameAnnotationName]; ok && previousName != "" {
				apiVersion := resource.GetApiVersion()
				kind := resource.GetKind()
				namespace := resource.GetNamespace()
				key := fmt.Sprintf("%s/%s/%s/%s", apiVersion, kind, namespace, previousName)
				resourceMappings[key] = resource.GetName()
			}
		}
	}
	return resourceMappings, nil
}

type referenceType struct {
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

func (r *referenceType) resolve(resource *yaml.RNode, resourceNamespace string, renamedResources map[string]string) error {
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
