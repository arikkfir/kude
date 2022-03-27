package internal

import (
	"bytes"
	"path/filepath"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"testing"
)

func TestBuildPipelineInfersFunctionVersion(t *testing.T) {
	absPath, err := filepath.Abs("../test/pipeline/inferred-function-version")
	if err != nil {
		t.Error(err)
	}
	pipeline, err := BuildPipeline(absPath, &kio.ByteWriter{Writer: &bytes.Buffer{}})
	if err != nil {
		t.Fatal(err)
	}
	df := pipeline.Filters[0].(*dockerFunction)
	if df.image != "ghcr.io/arikkfir/kude/functions/annotate:v0" {
		t.Errorf("expected image to be ghcr.io/arikkfir/kude/functions/annotate:v0, got %s", df.image)
	}

	absPath, err = filepath.Abs("../test/pipeline/single-function.test")
	if err != nil {
		t.Error(err)
	}
	pipeline, err = BuildPipeline(absPath, &kio.ByteWriter{Writer: &bytes.Buffer{}})
	if err != nil {
		t.Fatal(err)
	}
	df = pipeline.Filters[0].(*dockerFunction)
	if df.image != "ghcr.io/arikkfir/kude/functions/annotate:test" {
		t.Errorf("expected image to be ghcr.io/arikkfir/kude/functions/annotate:test, got %s", df.image)
	}
}
