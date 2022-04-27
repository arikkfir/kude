package internal

import (
	"context"
	"fmt"
	"github.com/arikkfir/kude/pkg"
	"github.com/google/uuid"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sigs.k8s.io/kustomize/kyaml/kio"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
	"strings"
	"time"
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

func NewPipeline(logger *log.Logger, dir string, writer kio.Writer) (Pipeline, error) {
	logger.Printf("Building pipeline at '%s'", dir)
	kudeYamlPath := filepath.Join(dir, "kude.yaml")
	manifestReader, err := os.Open(kudeYamlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open kude.yaml at '%s': %w", kudeYamlPath, err)
	}
	return NewPipelineFromReader(logger, dir, manifestReader, writer)
}

func NewPipelineFromReader(logger *log.Logger, dir string, manifestReader io.Reader, writer kio.Writer) (Pipeline, error) {
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
			logger: logger,
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
		prefix := logger.Prefix()
		if prefix != "" {
			prefix = "---" + prefix
		} else {
			prefix = "---> "
		}
		funcLogger := log.New(logger.Writer(), prefix, logger.Flags())

		f := dockerFunction{logger: funcLogger, pwd: pwd}
		funcConfig := v.(map[string]interface{})
		if name, ok := funcConfig["name"].(string); ok {
			f.name = name
		} else {
			f.name = uuid.NewString()
		}
		image, ok := funcConfig["image"].(string)
		if !ok {
			return nil, fmt.Errorf("failed to get image for function '%s': %w", f.name, err)
		} else if !strings.Contains(image, ":") {
			image = image + ":" + strings.Join(pkg.GetVersion().Build, ".")
		}
		f.image = image
		if entrypoint, ok := funcConfig["entrypoint"].([]string); ok {
			f.entrypoint = entrypoint
		}
		if user, ok := funcConfig["user"].(string); ok {
			f.user = user
		}
		if workDir, ok := funcConfig["workDir"].(string); ok {
			f.workDir = workDir
		}
		if allowNetwork, ok := funcConfig["network"].(bool); ok {
			f.allowNetwork = allowNetwork
		}
		if config, ok := funcConfig["config"].(map[string]interface{}); ok {
			f.config = config
		}
		if timeoutStr, ok := funcConfig["timeout"].(string); ok {
			timeout, err := time.ParseDuration(timeoutStr)
			if err != nil {
				return nil, fmt.Errorf("invalid timeout '%s' specified for function '%s': %w", timeoutStr, f.name, err)
			}
			f.timeout = timeout
		} else {
			f.timeout = 10 * time.Minute
		}
		if mountsList, ok := funcConfig["mounts"].([]interface{}); ok {
			var mounts []string
			for _, bind := range mountsList {
				mounts = append(mounts, bind.(string))
			}
			f.mounts = mounts
		}
		filters = append(filters, &f)
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
