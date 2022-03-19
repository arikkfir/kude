package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"github.com/arikkfir/kude/pkg"
	"github.com/spf13/viper"
	"io/ioutil"
	"os"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

func main() {
	pkg.Configure()

	type Entry struct {
		Key   string `json:"key"`
		Value string `json:"value"`
		Path  string `json:"path"`
	}

	var entries = make([]Entry, 0)
	err := viper.UnmarshalKey("contents", &entries)
	if err != nil {
		panic(fmt.Errorf("failed to unmarshal contents: %w", err))
	}

	allContent := bytes.Buffer{}
	data := make(map[string]string)
	for _, content := range entries {
		if content.Key == "" {
			panic(fmt.Errorf("key is required for all entries"))
		}
		if content.Value == "" && content.Path == "" {
			panic(fmt.Errorf("value or path is required for all entries"))
		}
		if content.Value != "" && content.Path != "" {
			panic(fmt.Errorf("value and path cannot be used together in a single entry"))
		}
		var value string
		if content.Value != "" {
			value = content.Value
		} else {
			contents, err := ioutil.ReadFile("/workspace/" + content.Path)
			if err != nil {
				panic(fmt.Errorf("error reading file '%s': %w", content.Path, err))
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
	hashedName := viper.GetString("name") + "-" + hex.EncodeToString(hash.Sum(nil))

	// Execute pipeline on provided resources
	pipeline := kio.Pipeline{
		Inputs: []kio.Reader{&kio.ByteReader{Reader: os.Stdin}},
		Filters: []kio.Filter{pkg.Generate(func() ([]*yaml.RNode, error) {
			node, err := yaml.NewMapRNode(nil).Pipe(
				yaml.Tee(yaml.SetField(yaml.APIVersionField, yaml.NewScalarRNode("v1"))),
				yaml.Tee(yaml.SetField(yaml.KindField, yaml.NewScalarRNode("ConfigMap"))),
				yaml.Tee(yaml.SetK8sName(hashedName)),
				yaml.Tee(yaml.SetAnnotation(pkg.PreviousNameAnnotationName, viper.GetString("name"))),
				yaml.Tee(yaml.SetField("data", yaml.NewMapRNode(&data))),
			)
			if err != nil {
				return nil, fmt.Errorf("error generating ConfigMap: %w", err)
			}
			if viper.IsSet("immutable") {
				err := node.PipeE(yaml.SetField("immutable", yaml.NewScalarRNode(viper.GetString("immutable"))))
				if err != nil {
					return nil, fmt.Errorf("error setting immutable field: %w", err)
				}
			}
			if viper.IsSet("namespace") {
				err := node.PipeE(yaml.Tee(yaml.SetK8sNamespace(viper.GetString("namespace"))))
				if err != nil {
					return nil, fmt.Errorf("error generating ConfigMap: %w", err)
				}
			}
			return []*yaml.RNode{node}, nil
		})},
		Outputs: []kio.Writer{kio.ByteWriter{Writer: os.Stdout}},
	}
	if err := pipeline.Execute(); err != nil {
		panic(err)
	}
}
