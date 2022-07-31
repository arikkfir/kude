package kude

import (
	"bytes"
	"context"
	"github.com/arikkfir/kude/internal"
	"io/ioutil"
	"log"
	"regexp"
	"testing"
)

func TestExecutionImpl_ExecuteToWriter(t *testing.T) {
	kudeYAML := `###
apiVersion: kude.kfirs.com/v1alpha2
kind: Pipeline
resources:
- service-account.yaml`
	saYAML := `###
apiVersion: v1
kind: ServiceAccount
metadata:
  name: test`
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	} else if err := ioutil.WriteFile(dir+"/kude.yaml", []byte(kudeYAML), 0644); err != nil {
		t.Fatal(err)
	} else if err := ioutil.WriteFile(dir+"/service-account.yaml", []byte(saYAML), 0644); err != nil {
		t.Fatal(err)
	}

	p, err := NewPipeline(dir)
	if err != nil {
		t.Fatal(err)
	}

	e, err := NewExecution(p, log.New(&internal.TestWriter{T: t}, "", 0))
	if err != nil {
		t.Fatal(err)
	}

	w, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	w.Close()
	if err := e.ExecuteToWriter(context.Background(), w); err == nil {
		t.Errorf("expected error, got nil")
	} else if matches, reErr := regexp.Match("failed to encode node to process stdout: yaml: write error: write .+: file already closed", []byte(err.Error())); reErr != nil {
		t.Fatal(err)
	} else if !matches {
		t.Errorf("expected error to match, got %s", err.Error())
	}
}

func TestExecutionImplExecuteToWriterNoResources(t *testing.T) {
	kudeYAML := `###
apiVersion: kude.kfirs.com/v1alpha2
kind: Pipeline
resources: []`
	dir, err := ioutil.TempDir("", "")
	out := &bytes.Buffer{}
	if err != nil {
		t.Error(err)
	} else if err := ioutil.WriteFile(dir+"/kude.yaml", []byte(kudeYAML), 0644); err != nil {
		t.Error(err)
	} else if p, err := NewPipeline(dir); err != nil {
		t.Error(err)
	} else if e, err := NewExecution(p, log.New(&internal.TestWriter{T: t}, "", 0)); err != nil {
		t.Error(err)
	} else if err := e.ExecuteToWriter(context.Background(), out); err != nil {
		t.Error(err)
	} else if out.String() != "" {
		t.Errorf("expected empty output, got: %s", out.String())
	}
}

func TestExecutionImplExecuteToWriterWithBadPipeline(t *testing.T) {
	kudeYAML := `###
apiVersion: kude.kfirs.com/v1alpha2
kind: Pipeline
resources:
- service-account.yaml`
	saYAML := `###
a : b : c`
	dir, err := ioutil.TempDir("", "")
	out := &bytes.Buffer{}
	if err != nil {
		t.Error(err)
	} else if err := ioutil.WriteFile(dir+"/kude.yaml", []byte(kudeYAML), 0644); err != nil {
		t.Error(err)
	} else if err := ioutil.WriteFile(dir+"/service-account.yaml", []byte(saYAML), 0644); err != nil {
		t.Error(err)
	} else if p, err := NewPipeline(dir); err != nil {
		t.Error(err)
	} else if e, err := NewExecution(p, log.New(&internal.TestWriter{T: t}, "", 0)); err != nil {
		t.Error(err)
	} else if err := e.ExecuteToWriter(context.Background(), out); err == nil {
		t.Errorf("expected error, got nil")
	} else if matches, reErr := regexp.Match("pipeline error: failed streaming resources found in 'service-account.yaml': failed to stream resources of 'service-account.yaml': failed to aggregate resources from '.+/service-account.yaml': failed to parse '.+/service-account.yaml': yaml: line 2: mapping values are not allowed in this context", []byte(err.Error())); reErr != nil {
		t.Fatal(err)
	} else if !matches {
		t.Errorf("expected error to match, got: %s", err.Error())
	}
}
