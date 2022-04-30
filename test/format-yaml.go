package test

import (
	"bytes"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
)

const formatYAMLFailureMessage = `%s: %w
======
%s
======`

func FormatYAML(yamlString io.Reader) (string, error) {
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
	return formatted.String(), nil
}
