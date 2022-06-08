package pkg

import (
	"bytes"
	"github.com/spf13/viper"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type testFunction1 struct {
	Foo      string `expected:"bar"`
	Some     string `expected:"thing"`
	logger   *log.Logger
	pwd      string
	cacheDir string
	tempDir  string
}

func (f *testFunction1) Configure(logger *log.Logger, pwd, cacheDir, tempDir string) error {
	f.logger = logger
	f.pwd = pwd
	f.cacheDir = cacheDir
	f.tempDir = tempDir
	return nil
}

func (f *testFunction1) Invoke(input io.Reader, output io.Writer) error {
	_, err := io.Copy(output, input)
	return err
}

func TestInvokeFunction(t *testing.T) {
	dir := t.TempDir()
	fileName := t.Name() + ".yaml"
	contents := []byte(`{"foo": "bar", "some": "badValue"}`)
	if err := os.WriteFile(filepath.Join(dir, fileName), contents, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUDE_SOME", "thing")
	logger := log.New(io.Discard, "prefix", log.LstdFlags)
	v := viper.New()
	f := testFunction1{}
	const stdinContent = "hello world"
	stdin := strings.NewReader(stdinContent)
	stdout := bytes.Buffer{}
	if err := invokeFunction(logger, v, dir, fileName, &f, stdin, &stdout); err != nil {
		t.Fatal(err)
	}
	if f.logger != logger {
		t.Errorf("logger not set")
	}
	if f.logger.Flags() != 0 {
		t.Errorf("logger flags not reset")
	}
	if dir, err := os.Getwd(); err != nil {
		t.Errorf("failed to get current working directory: %v", err)
	} else if f.pwd != dir {
		t.Errorf("pwd expected to be '%s', got '%s'", dir, f.pwd)
	}
	if f.cacheDir != DockerCacheDir {
		t.Errorf("cacheDir expected to be '%s', got '%s'", DockerCacheDir, f.cacheDir)
	}
	if f.tempDir != DockerTempDir {
		t.Errorf("tempDir expected to be '%s', got '%s'", DockerTempDir, f.tempDir)
	}
	if f.Foo != "bar" {
		t.Errorf("'foo' not set to 'bar'")
	}
	if f.Some != "thing" {
		t.Errorf("'some' not set to 'thing'")
	}
	if stdout.String() != stdinContent {
		t.Errorf("stdout expected to be '%s', got '%s'", stdinContent, stdout.String())
	}
}
