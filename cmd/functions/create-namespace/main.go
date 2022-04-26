package main

import (
	"fmt"
	"github.com/arikkfir/kude/pkg"
	"github.com/spf13/viper"
	"os"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

func main() {
	pkg.Configure()

	name := viper.GetString("name")
	if name == "" {
		panic(fmt.Errorf("namespace name is required"))
	}

	pipeline := kio.Pipeline{
		Inputs: []kio.Reader{&kio.ByteReader{Reader: os.Stdin}},
		Filters: []kio.Filter{pkg.Generate(func() ([]*yaml.RNode, error) {
			node, err := yaml.NewMapRNode(nil).Pipe(
				yaml.Tee(yaml.SetField(yaml.APIVersionField, yaml.NewScalarRNode("v1"))),
				yaml.Tee(yaml.SetField(yaml.KindField, yaml.NewScalarRNode("Namespace"))),
				yaml.Tee(yaml.SetK8sName(name)),
			)
			if err != nil {
				return nil, fmt.Errorf("error generating namespace: %w", err)
			}
			return []*yaml.RNode{node}, nil
		})},
		Outputs: []kio.Writer{kio.ByteWriter{Writer: os.Stdout}},
	}
	if err := pipeline.Execute(); err != nil {
		panic(fmt.Errorf("pipeline invocation failed: %w", err))
	}
}
