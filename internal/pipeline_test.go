package internal

import (
	"bytes"
	"fmt"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"strings"
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

func TestDeployments(t *testing.T) {
	root := "../test"
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			t.Error(fmt.Errorf("error walking '%s': %w", path, err))
			return nil
		} else if !d.IsDir() {
			return nil
		} else if root == path {
			return nil
		} else if !strings.HasSuffix(path, ".test") {
			return nil
		} else {
			absPath, err := filepath.Abs(path)
			if err != nil {
				t.Error(err)
			}
			t.Run("PATH="+path, func(t *testing.T) {
				t.Parallel()

				actual := bytes.Buffer{}
				pipeline, err := BuildPipeline(absPath, &kio.ByteWriter{Writer: &actual})
				if err != nil {
					t.Fatal(err)
				} else if err := pipeline.Execute(); err != nil {
					t.Fatal(err)
				}

				actualFormatted := bytes.Buffer{}
				decoder := yaml.NewDecoder(&actual)
				encoder := yaml.NewEncoder(&actualFormatted)
				encoder.SetIndent(2)
				for {
					var data interface{}
					if err := decoder.Decode(&data); err != nil {
						if err == io.EOF {
							break
						}
						t.Fatal(err)
					}
					if err := encoder.Encode(data); err != nil {
						t.Fatal(err)
					}
				}

				expectedFile, err := os.Open(filepath.Join(absPath, "expected.yaml"))
				if err != nil {
					t.Fatal(err)
				}
				expected, err := io.ReadAll(expectedFile)
				if err != nil {
					t.Fatal(err)
				}
				if string(expected) != actualFormatted.String() {
					edits := myers.ComputeEdits(span.URIFromPath("expected"), string(expected), actualFormatted.String())
					diff := fmt.Sprint(gotextdiff.ToUnified("expected", "actual", string(expected), edits))
					t.Errorf("Incorrect output:\n%s", diff)
				}
			})
			return nil
		}
	})
	if err != nil {
		t.Error(err)
	}
}

func TestMain(m *testing.M) {
	logger := logrus.StandardLogger()
	logger.SetLevel(logrus.TraceLevel) // TODO: make this configurable
	logger.SetOutput(os.Stdout)
	logger.SetReportCaller(false)
	logger.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp:       true,
		DisableLevelTruncation: true,
		PadLevelText:           true,
	})
	os.Exit(m.Run())
}
