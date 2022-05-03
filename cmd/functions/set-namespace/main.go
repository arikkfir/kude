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

	namespace := viper.GetString("namespace")
	if namespace == "" {
		panic(fmt.Errorf("namespace is required"))
	}

	labelSelector := viper.GetString("label-selector")

	pipeline := kio.Pipeline{
		Inputs: []kio.Reader{&kio.ByteReader{Reader: os.Stdin}},
		Filters: []kio.Filter{
			pkg.Fanout(
				yaml.Tee(
					pkg.SingleResourceLabelSelector(labelSelector),
					yaml.SetK8sNamespace(namespace),
				),
			),
		},
		Outputs: []kio.Writer{kio.ByteWriter{Writer: os.Stdout}},
	}
	if err := pipeline.Execute(); err != nil {
		panic(fmt.Errorf("pipeline invocation failed: %w", err))
	}
}
