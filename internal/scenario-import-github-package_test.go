package internal

import (
	_ "embed"
	
	"github.com/arikkfir/kude/pkg"
	"github.com/arikkfir/kude/test"
	"io"
	"log"
	"testing"
)

//go:embed scenario-import-github-package.yaml
var scenarioImportGithubPackageYAML string

func TestImportGithubPackageDocker(t *testing.T) {
    factory := func(logger *log.Logger, pwd string, manifestReader io.Reader, output io.Writer) (pkg.Package, error) {
        return NewPackage(logger, pwd, manifestReader, output, false)
    }
    if err := test.RunScenario(t, "import-github-package", scenarioImportGithubPackageYAML, factory); err != nil {
        t.Fatalf("Scenario failed: %v", err)
    }
}

func TestImportGithubPackageInternal(t *testing.T) {
    factory := func(logger *log.Logger, pwd string, manifestReader io.Reader, output io.Writer) (pkg.Package, error) {
        return NewPackage(logger, pwd, manifestReader, output, true)
    }
    if err := test.RunScenario(t, "import-github-package", scenarioImportGithubPackageYAML, factory); err != nil {
        t.Fatalf("Scenario failed: %v", err)
    }
}
