package internal

import (
	"bytes"
	_ "embed"
	
	"github.com/arikkfir/kude/test/scenario"
	"github.com/arikkfir/kude/test/util"
	"log"
	"os"
	"strings"
	"testing"
)

//go:embed scenario-render-chart-with-no-values-file.yaml
var RenderChartWithNoValuesFileYAML string

func TestRenderChartWithNoValuesFile(t *testing.T) {
	s, err := scenario.OpenScenario("TestRenderChartWithNoValuesFile", strings.NewReader(RenderChartWithNoValuesFileYAML))
	if err != nil {
		t.Fatalf("Failed to open scenario: %s", err)
	}

    test := func(t *testing.T, inlineBuiltinFunctions bool) {
        stdout := bytes.Buffer{}
        logger := log.New(&util.TestWriter{T: t}, "", 0)
        if r, err := os.Open(s.ManifestPath); err != nil {
            s.VerifyError(t, err)
        } else if p, err := NewPackage(logger, s.Dir, r, &stdout, inlineBuiltinFunctions); err != nil {
            s.VerifyError(t, err)
        } else if err := p.Execute(); err != nil {
            s.VerifyError(t, err)
        } else {
            s.VerifyError(t, err)
            s.VerifyStdout(t, &stdout)
        }
    }

	t.Run("CLI", func(t *testing.T) { test(t, false) })
	t.Run("Inline", func(t *testing.T) { test(t, true) })
}
