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

	var value string
	valuePath := viper.GetString("path")
	if valuePath != "" {
		valueFileBytes, err := os.ReadFile("/workspace/" + valuePath)
		if err != nil {
			panic(fmt.Errorf("failed reading '%s': %w", valuePath, err))
		}
		value = string(valueFileBytes)
	} else {
		value = viper.GetString("value")
	}

	pipeline := kio.Pipeline{
		Inputs:  []kio.Reader{&kio.ByteReader{Reader: os.Stdin}},
		Filters: []kio.Filter{pkg.Fanout(yaml.SetAnnotation(viper.GetString("name"), value))},
		Outputs: []kio.Writer{kio.ByteWriter{Writer: os.Stdout}},
	}
	if err := pipeline.Execute(); err != nil {
		panic(fmt.Errorf("pipeline invocation failed: %w", err))
	}
}
