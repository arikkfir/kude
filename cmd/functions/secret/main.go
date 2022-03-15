package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
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
	viper.SetDefault("type", "Opaque")
	pkg.Configure()

	// Read configuration file
	type Secret struct {
		Key   string `json:"key"`
		Value string `json:"value"`
		Path  string `json:"path"`
	}

	// Execute pipeline on provided resources
	pipeline := kio.Pipeline{
		Inputs: []kio.Reader{&kio.ByteReader{Reader: os.Stdin}},
		Filters: []kio.Filter{pkg.Generate(func() ([]*yaml.RNode, error) {
			// This example helped a lot:
			//		https://github.com/kubernetes-sigs/kustomize/blob/21e65990c1f7591c65d78db06b3d638141f8f740/api/internal/generators/secret.go

			// Generate data map
			var secretDefs = make([]Secret, 0)
			err := viper.UnmarshalKey("contents", &secretDefs)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal contents: %w", err)
			}

			contents := bytes.Buffer{}
			data := make(map[string]string)
			for _, content := range secretDefs {
				if content.Key == "" {
					return nil, fmt.Errorf("key is required for all entries")
				}
				if content.Value == "" && content.Path == "" {
					return nil, fmt.Errorf("value or path is required for all entries")
				}
				if content.Value != "" && content.Path != "" {
					return nil, fmt.Errorf("value and path cannot be used together in a single entry")
				}
				var value string
				if content.Value != "" {
					value = content.Value
				} else {
					contents, err := ioutil.ReadFile("/workspace/" + content.Path)
					if err != nil {
						return nil, fmt.Errorf("error reading file '%s': %w", content.Path, err)
					}
					value = string(contents)
				}
				data[content.Key] = base64.StdEncoding.EncodeToString([]byte(value))
				contents.Write([]byte(value))
			}

			// Generate a SHA hash of the contents, for a name corresponding to data contents
			// This ensures that changes in content result in a different name
			hash := sha1.New()
			hash.Write(contents.Bytes())
			hashedName := viper.GetString("name") + "-" + hex.EncodeToString(hash.Sum(nil))

			// Generate the Secret node
			node, err := yaml.NewMapRNode(nil).Pipe(
				yaml.Tee(yaml.SetField(yaml.APIVersionField, yaml.NewScalarRNode("v1"))),
				yaml.Tee(yaml.SetField(yaml.KindField, yaml.NewScalarRNode("Secret"))),
				yaml.Tee(yaml.SetK8sName(hashedName)),
				yaml.Tee(yaml.SetAnnotation(pkg.PreviousNameAnnotationName, viper.GetString("name"))),
				yaml.Tee(yaml.SetField("type", yaml.NewScalarRNode(viper.GetString("type")))),
				yaml.Tee(yaml.SetField("data", yaml.NewMapRNode(&data))),
			)
			if err != nil {
				return nil, fmt.Errorf("error generating secret: %w", err)
			}
			if viper.GetString("namespace") != "" {
				err := node.PipeE(yaml.Tee(yaml.SetK8sNamespace(viper.GetString("namespace"))))
				if err != nil {
					return nil, fmt.Errorf("error generating secret: %w", err)
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
