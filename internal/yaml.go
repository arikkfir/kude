package internal

import (
	"bytes"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
)

func GetChildKeyNode(mappingNode *yaml.Node, key string) (*yaml.Node, error) {
	if mappingNode.Kind == yaml.DocumentNode {
		mappingNode = mappingNode.Content[0]
	}
	if mappingNode.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("node is not a mapping")
	}
	for i := 0; i < len(mappingNode.Content); i += 2 {
		keyNode := mappingNode.Content[i]
		if keyNode.Value == key {
			return mappingNode.Content[i+1], nil
		}
	}
	return nil, nil
}

func MustChildKeyNode(mappingNode *yaml.Node, key string) *yaml.Node {
	if n, err := GetChildKeyNode(mappingNode, key); err != nil {
		panic(fmt.Errorf("failed getting child key node '%s': %w", key, err))
	} else if n == nil {
		panic(fmt.Errorf("could not find child key node '%s' in node", key))
	} else {
		return n
	}
}

func MustChildStringKey(mappingNode *yaml.Node, key string) string {
	return MustChildKeyNode(mappingNode, key).Value
}

func GetOrCreateChildKey(mappingNode *yaml.Node, key string) (*yaml.Node, error) {
	if mappingNode.Kind == yaml.DocumentNode {
		mappingNode = mappingNode.Content[0]
	}
	if mappingNode.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("node is not a mapping")
	}
	for i := 0; i < len(mappingNode.Content); i += 2 {
		keyNode := mappingNode.Content[i]
		if keyNode.Value == key {
			return mappingNode.Content[i+1], nil
		}
	}
	keyNode := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: key,
	}
	childNode := &yaml.Node{}
	mappingNode.Content = append(mappingNode.Content, keyNode, childNode)
	return childNode, nil
}

func NodeToString(node *yaml.Node) (string, error) {
	buffer := &bytes.Buffer{}
	if err := yaml.NewEncoder(buffer).Encode(node); err != nil {
		return "", fmt.Errorf("failed encoding node: %w", err)
	}
	return buffer.String(), nil
}

func FormatYAML(yamlString io.Reader) (string, error) {
	const formatYAMLFailureMessage = `%s: %w
======
%s
======`

	formatted := bytes.Buffer{}
	decoder := yaml.NewDecoder(yamlString)
	encoder := yaml.NewEncoder(&formatted)
	encoder.SetIndent(2)
	for {
		var data interface{}
		if err := decoder.Decode(&data); err != nil {
			if err == io.EOF {
				break
			}
			return "", fmt.Errorf(formatYAMLFailureMessage, "failed decoding YAML", err, yamlString)
		} else if err := encoder.Encode(data); err != nil {
			return "", fmt.Errorf(formatYAMLFailureMessage, "failed encoding struct", err, yamlString)
		}
	}
	encoder.Close()
	return formatted.String(), nil
}
