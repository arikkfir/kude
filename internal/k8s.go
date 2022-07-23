package internal

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/labels"
	"strconv"
	"strings"
)

func GetAPIVersion(node *yaml.Node) string {
	if apiVersionNode, err := GetChildKeyNode(node, "apiVersion"); err != nil {
		panic(fmt.Errorf("failed getting apiVersion node: %w", err))
	} else if apiVersionNode == nil {
		return ""
	} else {
		return apiVersionNode.Value
	}
}

func GetAPIGroupAndVersion(node *yaml.Node) (string, string) {
	apiVersion := GetAPIVersion(node)
	if lastSlashIndex := strings.LastIndex(apiVersion, "/"); lastSlashIndex < 0 {
		return "", apiVersion
	} else {
		return apiVersion[0:lastSlashIndex], apiVersion[lastSlashIndex+1:]
	}
}

func GetKind(node *yaml.Node) string {
	if kindNode, err := GetChildKeyNode(node, "kind"); err != nil {
		panic(fmt.Errorf("failed getting kind node: %w", err))
	} else if kindNode == nil {
		return ""
	} else {
		return kindNode.Value
	}
}

func GetNamespace(node *yaml.Node) string {
	if metadataNode, err := GetChildKeyNode(node, "metadata"); err != nil {
		panic(fmt.Errorf("failed getting metadata node: %w", err))
	} else if metadataNode == nil {
		return ""
	} else if namespaceNode, err := GetChildKeyNode(metadataNode, "namespace"); err != nil {
		panic(fmt.Errorf("failed getting namespace node: %w", err))
	} else if namespaceNode == nil {
		return ""
	} else {
		return namespaceNode.Value
	}
}

func GetName(node *yaml.Node) string {
	if metadataNode, err := GetChildKeyNode(node, "metadata"); err != nil {
		panic(fmt.Errorf("failed getting metadata node: %w", err))
	} else if metadataNode == nil {
		return ""
	} else if nameNode, err := GetChildKeyNode(metadataNode, "name"); err != nil {
		panic(fmt.Errorf("failed getting name node: %w", err))
	} else if nameNode == nil {
		return ""
	} else {
		return nameNode.Value
	}
}

func GetAnnotation(node *yaml.Node, name string) string {
	if metadataNode, err := GetChildKeyNode(node, "metadata"); err != nil {
		panic(fmt.Errorf("failed getting metadata node: %w", err))
	} else if metadataNode == nil {
		return ""
	} else if annotationsNode, err := GetChildKeyNode(metadataNode, "annotations"); err != nil {
		panic(fmt.Errorf("failed getting annotations node: %w", err))
	} else if annotationsNode == nil {
		return ""
	} else if annNode, err := GetChildKeyNode(annotationsNode, name); err != nil {
		panic(fmt.Errorf("failed getting annotation '%s' node: %w", name, err))
	} else if annNode == nil {
		return ""
	} else {
		return annNode.Value
	}
}

func SetAnnotation(n *yaml.Node, name string, value interface{}) error {
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("could not set annotation '%s' on given node, as it is not a mapping node", name)
	}

	var tv, tag string
	switch v := value.(type) {
	case int:
		tv = strconv.Itoa(v)
		tag = "!!int"
	case int8:
		tv = strconv.Itoa(int(v))
		tag = "!!int"
	case int16:
		tv = strconv.Itoa(int(v))
		tag = "!!int"
	case int32:
		tv = strconv.Itoa(int(v))
		tag = "!!int"
	case int64:
		tv = strconv.Itoa(int(v))
		tag = "!!int"
	case string:
		tv = v
		tag = "!!str"
	case bool:
		tv = strconv.FormatBool(v)
		tag = "!!bool"
	default:
		return fmt.Errorf("unsupported type %T", v)
	}

	metadataNode, err := GetOrCreateChildKey(n, "metadata")
	if err != nil {
		return fmt.Errorf("failed to get or create metadata node: %w", err)
	}
	metadataNode.Kind = yaml.MappingNode
	metadataNode.Tag = "!!map"

	annotationsNode, err := GetOrCreateChildKey(metadataNode, "annotations")
	if err != nil {
		return fmt.Errorf("failed to get or create metadata.annotations node: %w", err)
	}
	annotationsNode.Kind = yaml.MappingNode
	annotationsNode.Tag = "!!map"

	valueNode, err := GetOrCreateChildKey(annotationsNode, name)
	if err != nil {
		return fmt.Errorf("failed to get or create metadata.annotations node: %w", err)
	}
	valueNode.Kind = yaml.ScalarNode
	valueNode.Tag = tag
	valueNode.Value = tv
	return nil
}

func GetPreviousName(node *yaml.Node) string {
	return GetAnnotation(node, "kude.kfirs.com/previous-name")
}

func GetLabels(node *yaml.Node) (map[string]string, error) {
	metadataNode := MustChildKeyNode(node, "metadata")
	labelsNode, err := GetChildKeyNode(metadataNode, "labels")
	if err != nil {
		return nil, fmt.Errorf("failed getting labels node: %w", err)
	}
	labelsMap := make(map[string]string, len(labelsNode.Content)/2)
	for i := 0; i < len(labelsNode.Content); i += 2 {
		key := labelsNode.Content[i].Value
		value := labelsNode.Content[i+1].Value
		labelsMap[key] = value
	}
	return labelsMap, nil
}

func IsMatchingLabelSelector(n *yaml.Node, selector string) (bool, error) {
	s, err := labels.Parse(selector)
	if err != nil {
		return false, fmt.Errorf("failed parsing label selector '%s': %w", selector, err)
	}
	labelsMap, err := GetLabels(n)
	if err != nil {
		return false, fmt.Errorf("failed getting node labels: %w", err)
	}
	return s.Matches(labels.Set(labelsMap)), nil
}
