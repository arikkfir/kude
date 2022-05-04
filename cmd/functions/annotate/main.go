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

	annotationName := viper.GetString("name")
	if annotationName == "" {
		panic(fmt.Errorf("annotation name is required"))
	}

	var value string
	if valuePath := viper.GetString("path"); valuePath != "" {
		valueFileBytes, err := os.ReadFile("/workspace/" + valuePath)
		if err != nil {
			panic(fmt.Errorf("failed reading '%s': %w", valuePath, err))
		}
		value = string(valueFileBytes)
	} else {
		value = viper.GetString("value")
	}

	includes := make([]pkg.TargetingFilter, 0)
	if err := viper.UnmarshalKey("targets.includes", &includes); err != nil {
		panic(fmt.Errorf("failed to unmarshal targeting includes: %w", err))
	}
	excludes := make([]pkg.TargetingFilter, 0)
	if err := viper.UnmarshalKey("targets.excludes", &excludes); err != nil {
		panic(fmt.Errorf("failed to unmarshal targeting excludes: %w", err))
	}

	pipeline := kio.Pipeline{
		Inputs: []kio.Reader{&kio.ByteReader{Reader: os.Stdin}},
		Filters: []kio.Filter{
			pkg.Fanout(
				yaml.Tee(
					pkg.SingleResourceTargeting(includes, excludes),
					yaml.SetAnnotation(annotationName, value),
				),
			),
		},
		Outputs: []kio.Writer{kio.ByteWriter{Writer: os.Stdout}},
	}
	if err := pipeline.Execute(); err != nil {
		panic(fmt.Errorf("pipeline invocation failed: %w", err))
	}
}
