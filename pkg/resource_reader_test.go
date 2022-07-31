package kude

import (
	"context"
	"fmt"
	"github.com/arikkfir/kude/internal"
	kyaml "github.com/arikkfir/kyaml/pkg"
	"io"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"testing"
)

func TestResourceReaderReadEmptyDir(t *testing.T) {
	rr := &resourceReader{
		ctx:    context.Background(),
		pwd:    internal.MustGetwd(),
		logger: log.New(&internal.TestWriter{T: t}, "", 0),
		target: make(chan *kyaml.RNode, 100),
	}
	if err := rr.Read(""); err == nil {
		t.Error("expected error, got nil")
	} else if err.Error() != "failed to download '': error downloading ''" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestResourceReaderReadNonExistentDir(t *testing.T) {
	rr := &resourceReader{
		ctx:    context.Background(),
		pwd:    internal.MustGetwd(),
		logger: log.New(&internal.TestWriter{T: t}, "", 0),
		target: make(chan *kyaml.RNode, 100),
	}
	if err := rr.Read("/non-existent-file"); err == nil {
		t.Error("expected error, got nil")
	} else if err.Error() != "failed to download '/non-existent-file': stat /non-existent-file: no such file or directory" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestResourceReaderReadFile(t *testing.T) {
	target := make(chan *kyaml.RNode, 100)
	rr := &resourceReader{
		ctx:    context.Background(),
		pwd:    internal.MustGetwd(),
		logger: log.New(&internal.TestWriter{T: t}, "", 0),
		target: target,
	}
	dir1, err := os.MkdirTemp("", "resource_reader")
	if err != nil {
		t.Fatalf("failed to create tempdir: %v", err)
	}
	defer os.RemoveAll(dir1)
	yml1 := `###
apiVersion: v1
kind: ServiceAccount
metadata:
  name: sa`
	if f, err := ioutil.TempFile("", "*.yaml"); err != nil {
		t.Errorf("failed to create temp file: %v", err)
	} else if _, err := io.WriteString(f, yml1); err != nil {
		t.Errorf("failed to create file: %v", err)
	} else if err := f.Close(); err != nil {
		t.Errorf("failed to close file: %v", err)
	} else if err := rr.Read(f.Name()); err != nil {
		t.Errorf("failed to read file '%s': %v", f.Name(), err)
	} else {
		close(target)
		if len(target) != 1 {
			t.Errorf("expected 1 resource, got %d", len(target))
		} else if r, ok := <-target; !ok || r == nil {
			t.Errorf("no resource found")
		} else if name, err := r.GetName(); err != nil {
			t.Errorf("failed to get resource name: %v", err)
		} else if name != "sa" {
			t.Errorf("expected resource name 'sa', got '%s'", name)
		}
	}
}

func TestResourceReaderReadInvalidYAMLFile(t *testing.T) {
	target := make(chan *kyaml.RNode, 100)
	rr := &resourceReader{
		ctx:    context.Background(),
		pwd:    internal.MustGetwd(),
		logger: log.New(&internal.TestWriter{T: t}, "", 0),
		target: target,
	}
	dir, err := os.MkdirTemp("", "resource_reader")
	if err != nil {
		t.Fatalf("failed to create tempdir: %v", err)
	}
	defer os.RemoveAll(dir)
	yml1 := `###
apiVersion: v1
kind: ServiceAccount
  invalid: true
metadata:
  name: sa`
	if f, err := ioutil.TempFile("", "*.yaml"); err != nil {
		t.Errorf("failed to create temp file: %v", err)
	} else if _, err := io.WriteString(f, yml1); err != nil {
		t.Errorf("failed to create file: %v", err)
	} else if err := f.Close(); err != nil {
		t.Errorf("failed to close file: %v", err)
	} else if err := rr.Read(f.Name()); err == nil {
		t.Error("expected error, got nil")
	} else if matches, reErr := regexp.Match(fmt.Sprintf("failed to stream resources of '%s': failed to aggregate resources from '.+': failed to parse '.+': yaml: line 4: mapping values are not allowed in this context", f.Name()), []byte(err.Error())); reErr != nil {
		t.Errorf("failed matching error message: %v", err)
	} else if !matches {
		t.Errorf("unexpected error message for dir '%s' and file '%s': %s", dir, f.Name(), err.Error())
	}
}

func TestResourceReaderReadDir(t *testing.T) {
	target := make(chan *kyaml.RNode, 100)
	rr := &resourceReader{
		ctx:    context.Background(),
		pwd:    internal.MustGetwd(),
		logger: log.New(&internal.TestWriter{T: t}, "", 0),
		target: target,
	}

	dir, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fatalf("failed to create tempdir: %v", err)
	}
	defer os.RemoveAll(dir)

	yml1 := `###
apiVersion: v1
kind: ServiceAccount
metadata:
  name: sa`
	if f, err := ioutil.TempFile(dir, "*.yaml"); err != nil {
		t.Errorf("failed to create temp file: %v", err)
	} else if _, err := io.WriteString(f, yml1); err != nil {
		t.Errorf("failed to create file: %v", err)
	} else if err := f.Close(); err != nil {
		t.Errorf("failed to close file: %v", err)
	} else if err := rr.Read(dir); err != nil {
		t.Errorf("failed to read file '%s': %v", f.Name(), err)
	} else {
		close(target)
		if len(target) != 1 {
			t.Errorf("expected 1 resource, got %d", len(target))
		} else if r, ok := <-target; !ok || r == nil {
			t.Errorf("no resource found")
		} else if name, err := r.GetName(); err != nil {
			t.Errorf("failed to get resource name: %v", err)
		} else if name != "sa" {
			t.Errorf("expected resource name 'sa', got '%s'", name)
		}
	}
}
