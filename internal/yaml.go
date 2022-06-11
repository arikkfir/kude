package internal

import (
	"bytes"
	"fmt"
	"io"
	"sigs.k8s.io/kustomize/kyaml/kio/kioutil"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

func RemoveKYAMLAnnotations(node *yaml.RNode) error {
	annotations := node.GetAnnotations()
	delete(annotations, kioutil.IndexAnnotation)
	delete(annotations, kioutil.PathAnnotation)
	delete(annotations, kioutil.SeqIndentAnnotation)
	delete(annotations, kioutil.IdAnnotation)
	//goland:noinspection GoDeprecation
	delete(annotations, kioutil.LegacyIndexAnnotation)
	//goland:noinspection GoDeprecation
	delete(annotations, kioutil.LegacyPathAnnotation)
	//goland:noinspection GoDeprecation
	delete(annotations, kioutil.LegacyIdAnnotation)
	delete(annotations, kioutil.InternalAnnotationsMigrationResourceIDAnnotation)
	if err := node.SetAnnotations(annotations); err != nil {
		return fmt.Errorf("failed to remove KYAML annotations: %w", err)
	}
	return nil
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
