package main

import (
	_ "embed"
	"github.com/arikkfir/kude/internal"
	"github.com/arikkfir/kude/pkg"
	"github.com/arikkfir/kude/test"
	"io"
	"log"
	"testing"
)

//go:embed scenario-set-namespace-target-name.yaml
var scenarioSetNamespaceTargetNameYAML string

func TestSetNamespaceTargetNameDocker(t *testing.T) {
    factory := func(logger *log.Logger, pwd string, manifestReader io.Reader, output io.Writer) (pkg.Package, error) {
        return internal.NewPackage(logger, pwd, manifestReader, output, false)
    }
    if err := test.RunScenario(t, "set-namespace-target-name", scenarioSetNamespaceTargetNameYAML, factory); err != nil {
        t.Fatalf("Scenario failed: %v", err)
    }
}

func TestSetNamespaceTargetNameInternal(t *testing.T) {
    factory := func(logger *log.Logger, pwd string, manifestReader io.Reader, output io.Writer) (pkg.Package, error) {
        return internal.NewPackage(logger, pwd, manifestReader, output, true)
    }
    if err := test.RunScenario(t, "set-namespace-target-name", scenarioSetNamespaceTargetNameYAML, factory); err != nil {
        t.Fatalf("Scenario failed: %v", err)
    }
}
