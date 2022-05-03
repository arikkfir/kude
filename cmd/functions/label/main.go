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

	labelName := viper.GetString("name")
	if labelName == "" {
		panic(fmt.Errorf("label name is required"))
	}

	var value string
	if valuePath := viper.GetString("path"); valuePath != "" {
		bytes, err := os.ReadFile("/workspace/" + valuePath)
		if err != nil {
			panic(fmt.Errorf("failed to read file %s: %w", valuePath, err))
		}
		value = string(bytes)
	} else {
		value = viper.GetString("value")
	}

	labelSelector := viper.GetString("label-selector")

	pipeline := kio.Pipeline{
		Inputs: []kio.Reader{&kio.ByteReader{Reader: os.Stdin}},
		Filters: []kio.Filter{
			pkg.Fanout(
				yaml.Tee(
					pkg.SingleResourceLabelSelector(labelSelector),
					yaml.SetLabel(labelName, value),
				),
			),
		},
		Outputs: []kio.Writer{kio.ByteWriter{Writer: os.Stdout}},
	}
	if err := pipeline.Execute(); err != nil {
		panic(fmt.Errorf("pipeline invocation failed: %w", err))
	}
}
