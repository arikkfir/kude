package processing

import (
	"context"
	"fmt"
	"github.com/arikkfir/kude/internal"
	. "github.com/arikkfir/kude/internal/stream/types"
	"gopkg.in/yaml.v3"
)

type TargetingFilter struct {
	APIVersion    string `json:"apiVersion" yaml:"apiVersion"`
	Kind          string `json:"kind" yaml:"kind"`
	Namespace     string `json:"namespace" yaml:"namespace"`
	Name          string `json:"name" yaml:"name"`
	LabelSelector string `json:"labelSelector" yaml:"labelSelector"`
}

func K8sTargetingFilter(includes []TargetingFilter, excludes []TargetingFilter) NodeTransformer {
	return func(ctx context.Context, n *yaml.Node, c chan *yaml.Node) error {
		included := len(includes) == 0
		excluded := false
		apiVersion := internal.GetAPIVersion(n)
		kind := internal.GetKind(n)
		namespace := internal.GetNamespace(n)
		name := internal.GetName(n)
		for _, f := range includes {
			if f.APIVersion != "" && f.APIVersion != apiVersion {
				continue
			} else if f.Kind != "" && f.Kind != kind {
				continue
			} else if f.Namespace != "" && f.Namespace != namespace {
				continue
			} else if f.Name != "" && f.Name != name {
				continue
			} else if f.LabelSelector != "" {
				if matches, err := internal.IsMatchingLabelSelector(n, f.LabelSelector); err != nil {
					return fmt.Errorf("failed matching label selector '%s' to node: %w", f.LabelSelector, err)
				} else if !matches {
					continue
				}
			}
			included = true
		}
		for _, f := range excludes {
			if f.APIVersion != "" && f.APIVersion != apiVersion {
				continue
			} else if f.Kind != "" && f.Kind != kind {
				continue
			} else if f.Namespace != "" && f.Namespace != namespace {
				continue
			} else if f.Name != "" && f.Name != name {
				continue
			} else if f.LabelSelector != "" {
				if matches, err := internal.IsMatchingLabelSelector(n, f.LabelSelector); err != nil {
					return fmt.Errorf("failed matching label selector '%s' to node: %w", f.LabelSelector, err)
				} else if !matches {
					continue
				}
			}
			excluded = true
		}
		if included && !excluded {
			c <- n
		}
		return nil
	}
}
