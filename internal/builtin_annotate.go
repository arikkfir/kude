package internal

import (
	"fmt"
	"github.com/arikkfir/kude/pkg"
	"io"
	"log"
	"os"
	"path/filepath"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type Annotate struct {
	Name     string                `json:"name" yaml:"name"`
	Value    string                `json:"value" yaml:"value"`
	Path     string                `json:"path" yaml:"path"`
	Includes []pkg.TargetingFilter `json:"includes" yaml:"includes"`
	Excludes []pkg.TargetingFilter `json:"excludes" yaml:"excludes"`
	logger   *log.Logger
	pwd      string
}

func (f *Annotate) Configure(logger *log.Logger, pwd, _, _ string) error {
	f.logger = logger
	f.pwd = pwd
	return nil
}

func (f *Annotate) Invoke(r io.Reader, w io.Writer) error {
	if f.Name == "" {
		return fmt.Errorf("the '%s' property is required for this function", "name")
	}

	var value string
	if f.Path != "" {
		path := f.Path
		if !filepath.IsAbs(f.Path) {
			path = filepath.Join(f.pwd, f.Path)
		}
		valueFileBytes, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed reading '%s': %w", path, err)
		}
		value = string(valueFileBytes)
	} else {
		value = f.Value
	}

	pipeline := kio.Pipeline{
		Inputs: []kio.Reader{&kio.ByteReader{Reader: r}},
		Filters: []kio.Filter{
			pkg.Fanout(
				yaml.Tee(
					pkg.SingleResourceTargeting(f.Includes, f.Excludes),
					yaml.SetAnnotation(f.Name, value),
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
