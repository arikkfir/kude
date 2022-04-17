package internal

import (
	"context"
	"fmt"
	"github.com/arikkfir/kude/pkg"
	"github.com/google/uuid"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sigs.k8s.io/kustomize/kyaml/kio"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
	"strings"
)

const (
	APIVersion = "kude.kfirs.com/v1alpha1"
	Kind       = "Pipeline"
)

type Pipeline interface {
	Execute() error
}

type Function interface {
	GetName() string
	Invoke(ctx context.Context, r io.Reader, w io.Writer) error
}

func NewPipeline(dir string, writer kio.Writer) (Pipeline, error) {
	kudeYamlPath := filepath.Join(dir, "kude.yaml")
	manifestReader, err := os.Open(kudeYamlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open kude.yaml at '%s': %w", kudeYamlPath, err)
	}
	return NewPipelineFromReader(dir, manifestReader, writer)
}

func NewPipelineFromReader(dir string, manifestReader io.Reader, writer kio.Writer) (Pipeline, error) {
	pwd, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Read kude.yaml
	yaml, err := ioutil.ReadAll(manifestReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read YAML: %w", err)
	}
	kudeNode, err := kyaml.Parse(string(yaml))
	if err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Validate apiVersion and kind
	if kudeNode.GetApiVersion() != APIVersion {
		// TODO: support older versions of kude API group
		return nil, fmt.Errorf("unsupported pipeline apiVersion: '%s' (should be '%s')", kudeNode.GetApiVersion(), APIVersion)
	}
	if kudeNode.GetKind() != Kind {
		return nil, fmt.Errorf("unsupported pipeline kind: '%s' (should be '%s')", kudeNode.GetKind(), Kind)
	}

	// Build inputs
	resources, err := kudeNode.GetSlice("resources")
	if err != nil {
		if _, ok := err.(kyaml.NoFieldError); !ok {
			return nil, fmt.Errorf("failed to get pipeline: %w", err)
		} else {
			resources = []interface{}{}
		}
	}
	inputs := make([]kio.Reader, 0)
	for _, url := range resources {
		inputs = append(inputs, &resourceReader{
			pwd: pwd,
			url: url.(string),
		})
	}

	// Build filters
	functions, err := kudeNode.GetSlice("pipeline")
	if err != nil {
		if _, ok := err.(kyaml.NoFieldError); !ok {
			return nil, fmt.Errorf("failed to get pipeline: %w", err)
		} else {
			functions = []interface{}{}
		}
	}
	filters := make([]kio.Filter, 0)
	for _, v := range functions {
		funcConfig := v.(map[string]interface{})
		name, ok := funcConfig["name"].(string)
		if !ok {
			name = uuid.New().String()
		}
		image, ok := funcConfig["image"].(string)
		if !ok {
			return nil, fmt.Errorf("failed to get image for function '%s': %w", name, err)
		} else if !strings.Contains(image, ":") {
			image = image + ":" + strings.Join(pkg.GetVersion().Build, ".")
		}
		entrypoint, ok := funcConfig["entrypoint"].([]string)
		if !ok {
			entrypoint = nil
		}
		user, ok := funcConfig["user"].(string)
		if !ok {
			user = ""
		}
		allowNetwork, ok := funcConfig["network"].(bool)
		if !ok {
			allowNetwork = false
		}
		config, ok := funcConfig["config"].(map[string]interface{})
		if !ok {
			config = map[string]interface{}{}
		}
		var mounts []string
		if list, ok := funcConfig["mounts"].([]interface{}); ok {
			for _, bind := range list {
				mounts = append(mounts, bind.(string))
			}
		}
		filters = append(filters, &dockerFunction{
			pwd:          pwd,
			bindsRegexp:  regexp.MustCompile(`mount://([^:]+)(?::([^:]+))?`),
			name:         name,
			image:        image,
			entrypoint:   entrypoint,
			user:         user,
			allowNetwork: allowNetwork,
			config:       config,
			mounts:       mounts,
		})
	}
	filters = append(filters, &referencesResolverFunction{})

	// Compose the pipeline
	pipeline := kio.Pipeline{
		Inputs:  inputs,
		Filters: filters,
		Outputs: []kio.Writer{writer},
	}
	return &pipeline, nil
}
