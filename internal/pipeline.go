package internal

import (
	"fmt"
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
	metav1.TypeMeta `json:",inline"`
	pwd             string
	Resources       []string   `yaml:"resources"`
	Functions       []Function `yaml:"pipeline"`
}

func CreatePipeline(dir string) (*Pipeline, error) {
	pwd, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %v", err)
	}

	var pipeline = Pipeline{
		TypeMeta: metav1.TypeMeta{
			APIVersion: APIVersion,
			Kind:       Kind,
		},
		pwd: pwd,
	}

	// Read the file
	manifestFile := path.Join(pwd, "kude.yaml")
	if data, err := ioutil.ReadFile(manifestFile); err != nil {
		return nil, fmt.Errorf("failed reading '%s': %v", manifestFile, err)
	} else if err := yaml.Unmarshal(data, &pipeline); err != nil {
		return nil, fmt.Errorf("failed reading '%s': %v", manifestFile, err)
	}

	// Validate
	if pipeline.APIVersion != APIVersion {
		return nil, fmt.Errorf("unsupported pipeline apiVersion: %s (should be '%s')", pipeline.APIVersion, APIVersion)
	}
	if pipeline.Kind != Kind {
		return nil, fmt.Errorf("unsupported pipeline kind: %s (should be '%s')", pipeline.Kind, Kind)
	}

	// Initialize functions
	for i := range pipeline.Functions {
		function := &pipeline.Functions[i]
		function.pwd = pipeline.pwd
		if function.Name == "" {
			function.Name = "devbot-function-" + strconv.Itoa(i)
		}
	}
	return &pipeline, nil
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

	stream := NewStream(p.pwd, pw)
	defer stream.Close()
	for _, resource := range p.Resources {
		err := stream.Add(resource)
		if err != nil {
			return fmt.Errorf("failed reading resource '%s': %w", resource, err)
		}
	}
	err = pw.Close()
	if err != nil {
		return fmt.Errorf("failed to close pipe: %v", err)
	}

	for _, function := range p.Functions {
		fr, fw, err := os.Pipe()
		if err != nil {
			return fmt.Errorf("failed to create pipe: %v", err)
		}

		err = function.invokeFunction(pr, fw)
		if err != nil {
			return fmt.Errorf("failed invoking function '%s': %w", function.Name, err)
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
