package internal

import (
	"bytes"
	"fmt"
	"github.com/arikkfir/kude/test"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"io"
	"os"
	"strings"
	"testing"
)

func TestDeployments(t *testing.T) {
	entries, err := os.ReadDir("../test")
	if err != nil {
		t.Fatal(err)
	}

	for _, dirEntry := range entries {
		entry := dirEntry
		if entry.IsDir() && !strings.HasSuffix(entry.Name(), ".disabled") {
			t.Run("DEP="+entry.Name(), func(t *testing.T) {
				//t.Parallel()

				t.Logf("Creating pipeline for %s\n", entry.Name())
				pipeline, err := CreatePipeline("../test/" + entry.Name())
				if err != nil {
					t.Fatal(err)
				}

				t.Logf("Executing pipeline for %s\n", entry.Name())
				actual := bytes.Buffer{}
				err = pipeline.executePipeline(&actual)
				if err != nil {
					t.Fatal(err)
				}

				t.Logf("Verifying pipeline output for %s\n", entry.Name())
				expectedFile, err := os.Open(fmt.Sprintf("../test/%s/expected.yaml", entry.Name()))
				if err != nil {
					t.Fatal(err)
				}
				expected, err := io.ReadAll(expectedFile)
				if err != nil {
					t.Fatal(err)
				}
				if string(expected) != actual.String() {
					edits := myers.ComputeEdits(span.URIFromPath("expected"), string(expected), actual.String())
					diff := fmt.Sprint(gotextdiff.ToUnified("expected", "actual", string(expected), edits))
					t.Errorf("Incorrect output:\n%s", diff)
				}
			})
		}
	}
}

func TestMain(m *testing.M) {
	if os.Getenv("BUILD_FUNCTIONS") == "1" {
		err := test.BuildFunctionDockerImages()
		if err != nil {
			panic(err)
		}
	}
	os.Exit(m.Run())
}
