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
