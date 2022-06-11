package kude

import (
	"fmt"
	"io"
	"log"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"strings"
)

type CreateNamespace struct {
	Name string `json:"name" yaml:"name"`
}

func (f *CreateNamespace) Invoke(_ *log.Logger, _, _, _ string, r io.Reader, w io.Writer) error {
	if f.Name == "" {
		return fmt.Errorf("%s is required for creating namespaces", "name")
	}

	namespace := `
apiVersion: v1
kind: Namespace
metadata:
  name: ` + f.Name + `
`
	pipeline := kio.Pipeline{
		Inputs: []kio.Reader{
			&kio.ByteReader{Reader: strings.NewReader(namespace)},
			&kio.ByteReader{Reader: r},
		},
		Filters: []kio.Filter{},
		Outputs: []kio.Writer{kio.ByteWriter{Writer: w}},
	}
	if err := pipeline.Execute(); err != nil {
		return fmt.Errorf("pipeline invocation failed: %w", err)
	}
	return nil
}
