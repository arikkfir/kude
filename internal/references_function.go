package internal

import (
	"context"
	_ "embed"
	"fmt"
	"github.com/vmware-labs/yaml-jsonpath/pkg/yamlpath"
	"gopkg.in/yaml.v3"
	"io"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

type referencesResolverFunction struct{}

type referenceType struct {
	Group   string `yaml:"group"`
	Version string `yaml:"version"`
	Kind    string `yaml:"kind"`
	Field   struct {
		NamePath                    string `yaml:"namePath"`
		NamespacePathRelativeToName string `yaml:"namespacePathRelativeToName"`
		Type                        struct {
			Group   string `yaml:"group"`
			Version string `yaml:"version"`
			Kind    string `yaml:"kind"`
		} `yaml:"type"`
	} `yaml:"field"`
}

func (r *referenceType) resolve(resource *yaml.Node, resourceNamespace string, renamedResources map[string]string) error {
	namePath, err := yamlpath.NewPath(r.Field.NamePath)
	if err != nil {
		return fmt.Errorf("error creating name path: %w", err)
	}

	nameNodes, err := namePath.Find(resource)
	if err != nil {
		return fmt.Errorf("error finding name path: %w", err)
	} else if len(nameNodes) == 0 {
		return nil
	}

	var refFieldAPIVersion string
	if r.Field.Type.Group == "" {
		refFieldAPIVersion = r.Field.Type.Version
	} else {
		refFieldAPIVersion = r.Field.Type.Group + "/" + r.Field.Type.Version
	}

	for _, nameNode := range nameNodes {
		namespace := resourceNamespace
		if r.Field.NamespacePathRelativeToName != "" {
			namespacePath, err := yamlpath.NewPath(r.Field.NamespacePathRelativeToName)
			if err != nil {
				return fmt.Errorf("error creating namespace path: %w", err)
			}

			namespaceNodes, err := namespacePath.Find(nameNode)
			if err != nil {
				return fmt.Errorf("error finding namespace path: %w", err)
			}

			if len(namespaceNodes) == 1 {
				namespace = namespaceNodes[0].Value
			} else if len(namespaceNodes) > 1 {
				return fmt.Errorf("namespace path must return one node, but returned %d instead", len(namespaceNodes))
			}
		}

		key := fmt.Sprintf("%s/%s/%s/%s", refFieldAPIVersion, r.Field.Type.Kind, namespace, nameNode.Value)
		if newName, ok := renamedResources[key]; ok {
			nameNode.SetString(newName)
		}
	}
	return nil
}

func (r *referencesResolverFunction) GetName() string {
	return "references"
}

func (r *referencesResolverFunction) Invoke(_ context.Context, resourcesReader io.Reader, outputWriter io.Writer) error {

	// Read the resources from stdin
	resources, err := readAllYAMLDocuments(resourcesReader)
	if err != nil {
		return fmt.Errorf("error reading resources: %w", err)
	}

	// Build a map of resources that should be replaced
	resourceMappings, err := r.collectRenamedResources(resources)
	if err != nil {
		return fmt.Errorf("error collecting renamed resources: %w", err)
	}

	// Iterate resources and replace references
	for _, resource := range resources {
		resourceAPIVersion, resourceKind, resourceNamespace, _, err := getYAMLResourceInfo(resource)
		if err != nil {
			return fmt.Errorf("error extracting resource info: %w", err)
		}
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
					return fmt.Errorf("error resolving reference: %w", err)
				}
			}
		}
	}

	// Print resources back out
	err = writeAllYAMLDocuments(resources, outputWriter)
	if err != nil {
		return fmt.Errorf("error writing resources: %w", err)
	}
	return nil
}

func (r *referencesResolverFunction) collectRenamedResources(resources []*yaml.Node) (map[string]string, error) {
	resourceMappings := make(map[string]string)
	for _, resource := range resources {
		apiVersion, kind, namespace, name, err := getYAMLResourceInfo(resource)
		if err != nil {
			return nil, fmt.Errorf("error extracting resource info: %w", err)
		}
		previousName, err := getYAMLNodeScalarValue(resource, fmt.Sprintf("$.metadata.annotations['%s']", "kude.kfirs.com/previous-name"))
		if err != nil {
			return nil, fmt.Errorf("error finding previous-name: %w", err)
		} else if previousName != "" {
			key := fmt.Sprintf("%s/%s/%s/%s", apiVersion, kind, namespace, previousName)
			resourceMappings[key] = name
		}
	}
	return resourceMappings, nil
}

func readAllYAMLDocuments(r io.Reader) ([]*yaml.Node, error) {
	documents := make([]*yaml.Node, 0)
	decoder := yaml.NewDecoder(r)
	for {
		var node yaml.Node
		err := decoder.Decode(&node)
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("failed decoding YAML document: %w", err)
		}
		documents = append(documents, &node)
	}
	return documents, nil
}

func writeAllYAMLDocuments(documents []*yaml.Node, w io.Writer) error {
	encoder := yaml.NewEncoder(w)
	encoder.SetIndent(2)
	for _, resource := range documents {
		if err := encoder.Encode(resource); err != nil {
			return fmt.Errorf("error encoding document: %w", err)
		}
	}
	return nil
}

func getYAMLNodeScalarValue(node *yaml.Node, pathExpr string) (string, error) {
	path, err := yamlpath.NewPath(pathExpr)
	if err != nil {
		panic(err)
	}
	if values, err := path.Find(node); err != nil {
		return "", err
	} else if len(values) == 0 {
		return "", nil
	} else if len(values) == 1 {
		return values[0].Value, nil
	} else {
		return "", fmt.Errorf("multiple values found for path %s", pathExpr)
	}
}

func getYAMLResourceInfo(resourceNode *yaml.Node) (apiVersion, kind, namespace, name string, err error) {
	apiVersion, err = getYAMLNodeScalarValue(resourceNode, "$.apiVersion")
	if err != nil {
		return apiVersion, kind, namespace, name, fmt.Errorf("error finding apiVersion: %w", err)
	}
	kind, err = getYAMLNodeScalarValue(resourceNode, "$.kind")
	if err != nil {
		return apiVersion, kind, namespace, name, fmt.Errorf("error finding kind: %w", err)
	}
	namespace, err = getYAMLNodeScalarValue(resourceNode, "$.metadata.namespace")
	if err != nil {
		return apiVersion, kind, namespace, name, fmt.Errorf("error finding namespace: %w", err)
	}
	name, err = getYAMLNodeScalarValue(resourceNode, "$.metadata.name")
	if err != nil {
		return apiVersion, kind, namespace, name, fmt.Errorf("error finding name: %w", err)
	}
	return apiVersion, kind, namespace, name, nil
}
