package internal

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"path/filepath"
	"regexp"
	"sigs.k8s.io/kustomize/kyaml/kio"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
)

const (
	APIVersion = "kude.kfirs.com/v1alpha1"
	Kind       = "Pipeline"
)

func BuildPipeline(dir string, writer kio.Writer) (*kio.Pipeline, error) {
	pwd, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Read kude.yaml
	kudeYamlPath := filepath.Join(pwd, "kude.yaml")
	kudeNode, err := kyaml.ReadFile(kudeYamlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read '%s': %w", kudeYamlPath, err)
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
		return nil, fmt.Errorf("failed to get resources: %w", err)
	}
	inputs := make([]kio.Reader, 0)
	for _, url := range resources {
		inputs = append(inputs, &resourceReader{
			logger: logrus.WithField("url", url),
			pwd:    pwd,
			url:    url.(string),
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
		}
		entrypoint, ok := funcConfig["entrypoint"].([]string)
		if !ok {
			entrypoint = nil
		}
		user, ok := funcConfig["user"].(string)
		if !ok {
			user = ""
		}
		config, ok := funcConfig["config"].(map[string]interface{})
		if !ok {
			config = map[string]interface{}{}
		}
		filters = append(filters, &dockerFunction{
			pwd:         pwd,
			logger:      logrus.WithField("function", name),
			bindsRegexp: regexp.MustCompile(`mount://([^:]+)(?::([^:]+))?`),
			name:        name,
			image:       image,
			entrypoint:  entrypoint,
			user:        user,
			config:      config,
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
