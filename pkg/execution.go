package kude

import (
	"context"
	"io"
	"log"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type Execution interface {
	GetPipeline() Pipeline
	GetLogger() *log.Logger
	ExecuteToWriter(ctx context.Context, w io.Writer) error
	ExecuteToSink(ctx context.Context, target chan *yaml.RNode) error
}
