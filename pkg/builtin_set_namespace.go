package kude

import (
	"fmt"
	"io"
	"log"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type SetNamespace struct {
	Namespace string            `json:"namespace" yaml:"namespace"`
	Includes  []TargetingFilter `json:"includes" yaml:"includes"`
	Excludes  []TargetingFilter `json:"excludes" yaml:"excludes"`
}

func (f *SetNamespace) Invoke(_ *log.Logger, _, _, _ string, r io.Reader, w io.Writer) error {
	if f.Namespace == "" {
		return fmt.Errorf("the '%s' property is required for this function", "name")
	}

	pipeline := kio.Pipeline{
		Inputs: []kio.Reader{&kio.ByteReader{Reader: r}},
		Filters: []kio.Filter{
			kio.FilterAll(
				yaml.Tee(
					SingleResourceTargeting(f.Includes, f.Excludes),
					yaml.SetK8sNamespace(f.Namespace),
				),
			),
		},
		Outputs: []kio.Writer{kio.ByteWriter{Writer: w}},
	}
	if err := pipeline.Execute(); err != nil {
		return fmt.Errorf("pipeline invocation failed: %w", err)
	}
	return nil
}
