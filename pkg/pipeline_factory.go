package kude

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func newInliningPipeline(dir string) (Pipeline, error) {
	p, err := NewPipeline(dir)
	if err != nil {
		return nil, err
	}
	p.(*pipelineImpl).inlineBuiltinFunctions = true
	return p, nil
}

func NewPipeline(dir string) (Pipeline, error) {
	pwd, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	pipelineFilePath := filepath.Join(pwd, "kude.yaml")
	f, err := os.Open(pipelineFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open '%s': %w", pipelineFilePath, err)
	}

	decoder := yaml.NewDecoder(f)
	decoder.KnownFields(true)
	p := pipelineImpl{pwd: pwd}
	if err := decoder.Decode(&p); err != nil {
		return nil, fmt.Errorf("failed to decode '%s': %w", pipelineFilePath, err)
	}

	if apiVersion := p.GetAPIVersion(); apiVersion != PipelineAPIVersion {
		return nil, fmt.Errorf("unsupported apiVersion: '%s' (should be '%s')", apiVersion, PipelineAPIVersion)
	} else if kind := p.GetKind(); kind != PipelineKind {
		return nil, fmt.Errorf("unsupported kind: '%s' (should be '%s')", kind, PipelineKind)
	}

	for i, step := range p.Steps {
		if step.ID == "" {
			step.ID = strconv.Itoa(i + 1)
			if len(step.ID) < 3 {
				step.ID = strings.Repeat("0", 3-len(step.ID)) + step.ID
			}
		}
		if step.Image == "" {
			return nil, fmt.Errorf("step #%d (%s) has an empty image", i, step.Name)
		} else if !strings.Contains(step.Image, ":") {
			step.Image = step.Image + ":" + strings.Join(GetVersion().Build, ".")
		}
		if step.Name == "" {
			step.Name = step.ID + " // " + step.Image
		}
		if step.Workdir == "" {
			step.Workdir = "/workspace"
		}
	}

	return &p, nil
}
