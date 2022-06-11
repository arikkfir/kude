package kude

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"path/filepath"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
	"strconv"
)

type CreateConfigMapEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Path  string `json:"path"`
}

type CreateConfigMap struct {
	Name      string                 `json:"name" yaml:"name"`
	Namespace string                 `json:"namespace" yaml:"namespace"`
	Immutable *bool                  `json:"immutable" yaml:"immutable"`
	Contents  []CreateConfigMapEntry `json:"contents" yaml:"contents"`
}

func (f *CreateConfigMap) Invoke(_ *log.Logger, pwd, _, _ string, r io.Reader, w io.Writer) error {
	if f.Name == "" {
		return fmt.Errorf("%s is required for creating config maps", "name")
	}

	allContent := bytes.Buffer{}
	data := make(map[string]string)
	for i, content := range f.Contents {
		if content.Key == "" {
			return fmt.Errorf("key is required for all entries (missing for entry %d)", i)
		}
		if content.Value == "" && content.Path == "" {
			return fmt.Errorf("value or path is required for all entries (missing for entry %d)", i)
		}
		if content.Value != "" && content.Path != "" {
			return fmt.Errorf("value and path cannot be used together in a single entry (encountered for entry %d)", i)
		}
		var value string
		if content.Value != "" {
			value = content.Value
		} else {
			path := content.Path
			if !filepath.IsAbs(content.Path) {
				path = filepath.Join(pwd, content.Path)
			}
			contents, err := ioutil.ReadFile(path)
			if err != nil {
				return fmt.Errorf("error reading file '%s': %w", path, err)
			}
			value = string(contents)
		}
		data[content.Key] = value
		allContent.Write([]byte(value))
	}

	// Generate an SHA hash of the contents, for a name corresponding to data contents
	// This ensures that changes in content result in a different name
	hash := sha1.New()
	hash.Write(allContent.Bytes())
	hashedName := f.Name + "-" + hex.EncodeToString(hash.Sum(nil))

	// Execute pipeline on provided resources
	pipeline := kio.Pipeline{
		Inputs: []kio.Reader{&kio.ByteReader{Reader: r}},
		Filters: []kio.Filter{Generate(func() ([]*yaml.RNode, error) {
			dataNode := yaml.NewMapRNode(&data)
			for _, n := range dataNode.Content() {
				n.Tag = yaml.NodeTagString
			}
			node, err := yaml.NewMapRNode(nil).Pipe(
				yaml.Tee(yaml.SetField(yaml.APIVersionField, yaml.NewScalarRNode("v1"))),
				yaml.Tee(yaml.SetField(yaml.KindField, yaml.NewScalarRNode("ConfigMap"))),
				yaml.Tee(yaml.SetK8sName(hashedName)),
				yaml.Tee(yaml.SetAnnotation(PreviousNameAnnotationName, f.Name)),
				yaml.Tee(yaml.SetField("data", dataNode)),
			)
			if err != nil {
				return nil, fmt.Errorf("error generating ConfigMap: %w", err)
			}
			if f.Immutable != nil {
				immutableNode := yaml.NewScalarRNode(strconv.FormatBool(*f.Immutable))
				immutableNode.YNode().Tag = "!!bool"
				err := node.PipeE(yaml.SetField("immutable", immutableNode))
				if err != nil {
					return nil, fmt.Errorf("error setting immutable field: %w", err)
				}
			}
			if f.Namespace != "" {
				err := node.PipeE(yaml.Tee(yaml.SetK8sNamespace(f.Namespace)))
				if err != nil {
					return nil, fmt.Errorf("error setting configMap namespace: %w", err)
				}
			}
			return []*yaml.RNode{node}, nil
		})},
		Outputs: []kio.Writer{kio.ByteWriter{Writer: w}},
	}
	if err := pipeline.Execute(); err != nil {
		return fmt.Errorf("pipeline invocation failed: %w", err)
	}
	return nil
}
