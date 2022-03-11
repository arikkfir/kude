package internal

import (
	"context"
	"fmt"
	ss "github.com/arikkfir/kude/internal/stream"
	"gopkg.in/yaml.v3"
	"io"
	"io/ioutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"path"
	"path/filepath"
	"strconv"
)

const (
	APIVersion = "kude.io/v1alpha1"
	Kind       = "Pipeline"
	Version    = "v0.0.1"
)

type Pipeline struct {
	pwd       string
	resources []string `yaml:"resources"`
	functions []Function
}

func CreatePipeline(dir string) (*Pipeline, error) {
	pwd, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	type function struct {
		Name       string                 `json:"name"`
		Image      string                 `json:"image"`
		Entrypoint []string               `json:"entrypoint,omitempty"`
		User       string                 `json:"user,omitempty"`
		Config     map[string]interface{} `json:",inline"`
	}
	type pipeline struct {
		metav1.TypeMeta `json:",inline"`
		Resources       []string   `yaml:"resources"`
		Pipeline        []function `yaml:"pipeline"`
	}

	// Read the file
	var p = pipeline{
		TypeMeta: metav1.TypeMeta{
			APIVersion: APIVersion,
			Kind:       Kind,
		},
	}
	manifestFile := path.Join(pwd, "kude.yaml")
	if data, err := ioutil.ReadFile(manifestFile); err != nil {
		return nil, fmt.Errorf("failed reading '%s': %v", manifestFile, err)
	} else if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("failed reading '%s': %v", manifestFile, err)
	}

	// Validate
	if p.APIVersion != APIVersion {
		return nil, fmt.Errorf("unsupported pipeline apiVersion: %s (should be '%s')", p.APIVersion, APIVersion)
	}
	if p.Kind != Kind {
		return nil, fmt.Errorf("unsupported pipeline kind: %s (should be '%s')", p.Kind, Kind)
	}

	// Initialize functions
	functions := make([]Function, 0)
	for i := range p.Pipeline {
		f := &p.Pipeline[i]
		if f.Name == "" {
			f.Name = "kude-function-" + strconv.Itoa(i)
		}
		dockerFunc, err := newDockerFunction(pwd, f.Name, f.Image, f.Entrypoint, f.User, f.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to create function '%s': %w", f.Name, err)
		}
		functions = append(functions, dockerFunc)
	}
	functions = append(functions, &referencesResolverFunction{})

	return &Pipeline{
		pwd:       pwd,
		resources: p.Resources,
		functions: functions,
	}, nil
}

func (p *Pipeline) Execute() error {
	err := p.executePipeline(os.Stdout)
	if err != nil {
		return err
	}
	return nil
}

func (p *Pipeline) executePipeline(w io.Writer) error {
	pr, pw, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("failed to create pipe: %v", err)
	}

	stream := ss.NewStream(p.pwd, handleDirectory, pw)
	defer stream.Close()
	for _, resource := range p.resources {
		err := stream.Add(resource)
		if err != nil {
			return fmt.Errorf("failed reading resource '%s': %w", resource, err)
		}
	}
	err = pw.Close()
	if err != nil {
		return fmt.Errorf("failed to close pipe: %v", err)
	}

	ctx := context.Background()
	for _, function := range p.functions {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		fr, fw, err := os.Pipe()
		if err != nil {
			return fmt.Errorf("failed to create pipe: %v", err)
		}

		err = function.Invoke(ctx, pr, fw)
		if err != nil {
			return fmt.Errorf("failed invoking function '%s': %w", function.GetName(), err)
		}
		fw.Close()

		pr = fr
	}
	pw.Close()

	_, err = io.Copy(w, pr)
	if err != nil {
		return fmt.Errorf("failed serializing output: %w", err)
	}
	return nil
}
