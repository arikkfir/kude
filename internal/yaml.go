package internal

import (
	"bytes"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
)

func NodeToString(node *yaml.Node) (string, error) {
	buffer := &bytes.Buffer{}
	encoder := yaml.NewEncoder(buffer)
	encoder.SetIndent(2)
	if err := encoder.Encode(node); err != nil {
		return "", fmt.Errorf("failed encoding node: %w", err)
	}
	encoder.Close()
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
