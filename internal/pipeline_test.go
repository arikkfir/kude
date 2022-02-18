package internal

import (
	"bytes"
	"fmt"
	"github.com/arikkfir/kude/test"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"github.com/sirupsen/logrus"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
			return fs.SkipDir
		} else {
			absPath, err := filepath.Abs(path)
			if err != nil {
				t.Error(err)
			}
			t.Run("PATH="+path, func(t *testing.T) {
				//t.Parallel()
				pipeline, err := CreatePipeline(absPath)
				if err != nil {
					t.Error(err)
				}
				actual := bytes.Buffer{}
				err = pipeline.executePipeline(&actual)
				if err != nil {
					t.Fatal(err)
				}
				expectedFile, err := os.Open(filepath.Join(absPath, "expected.yaml"))
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
	if os.Getenv("BUILD_FUNCTIONS") == "1" {
		err := test.BuildFunctionDockerImages()
		if err != nil {
			panic(err)
		}
	}
	os.Exit(m.Run())
}
