package kude

import (
	"bytes"
	"errors"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
	"strings"
	"testing"
)

func TestGeneratorFilter(t *testing.T) {
	out := bytes.Buffer{}
	pipeline := kio.Pipeline{
		Inputs: []kio.Reader{
			&kio.ByteReader{Reader: strings.NewReader(`{jack: black}`)},
		},
		Filters: []kio.Filter{
			Generate(func() ([]*yaml.RNode, error) {
				return []*yaml.RNode{
					yaml.MustParse(`{foo: bar}`),
				}, nil
			}),
		},
		Outputs: []kio.Writer{kio.ByteWriter{Writer: &out, Style: yaml.LiteralStyle}},
	}
	if err := pipeline.Execute(); err != nil {
		t.Errorf("failed to execute pipeline: %v", err)
	}
	if out.String() != "jack: black\n---\nfoo: bar\n" {
		t.Errorf("unexpected output: %s", out.String())
	}
}

func TestGeneratorFilterError(t *testing.T) {
	out := bytes.Buffer{}
	pipeline := kio.Pipeline{
		Inputs: []kio.Reader{
			&kio.ByteReader{Reader: strings.NewReader(`{jack: black}`)},
		},
		Filters: []kio.Filter{
			Generate(func() ([]*yaml.RNode, error) {
				return nil, errors.New("generator error")
			}),
		},
		Outputs: []kio.Writer{kio.ByteWriter{Writer: &out, Style: yaml.LiteralStyle}},
	}
	if err := pipeline.Execute(); err == nil {
		t.Fatal("expected pipeline to fail, but it did not")
	} else if err.Error() != "failed generating nodes: generator error" {
		t.Errorf("expected error 'failed generating nodes: generator error', got: %s", err.Error())
	}
}
