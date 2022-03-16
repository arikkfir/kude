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
	fmt.Fprintf(os.Stderr, "Will set annotation '%s: %s'", viper.GetString("name"), viper.GetString("value"))
	pipeline := kio.Pipeline{
		Inputs:  []kio.Reader{&kio.ByteReader{Reader: os.Stdin}},
		Filters: []kio.Filter{pkg.Fanout(yaml.SetAnnotation(viper.GetString("name"), viper.GetString("value")))},
		Outputs: []kio.Writer{kio.ByteWriter{Writer: os.Stdout}},
	}
	if err := pipeline.Execute(); err != nil {
		panic(err)
	}
}