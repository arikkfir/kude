package main

import (
	"fmt"
	"github.com/arikkfir/kude/pkg"
	"github.com/spf13/viper"
	"os"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"strings"
)

func main() {
	pkg.Configure()

	name := viper.GetString("name")
	if name == "" {
		panic(fmt.Errorf("namespace name is required"))
	}

	namespace := `
apiVersion: v1
kind: Namespace
metadata:
  name: ` + name + `
`
	pipeline := kio.Pipeline{
		Inputs: []kio.Reader{
			&kio.ByteReader{Reader: strings.NewReader(namespace)},
			&kio.ByteReader{Reader: os.Stdin},
		},
		Filters: []kio.Filter{},
		Outputs: []kio.Writer{kio.ByteWriter{Writer: os.Stdout}},
	}
	if err := pipeline.Execute(); err != nil {
		panic(fmt.Errorf("pipeline invocation failed: %w", err))
	}
}
