package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/arikkfir/kude/pkg"
	"github.com/spf13/viper"
	"io"
	"log"
	"os"
	"os/exec"
	"sigs.k8s.io/kustomize/kyaml/kio"
)

func main() {
	log.Default().SetFlags(0)
	pkg.Configure()

	expr := viper.GetString("expr")
	if expr == "" {
		panic(fmt.Errorf("expr is required"))
	}

	pr, pw, err := os.Pipe()
	if err != nil {
		panic(fmt.Errorf("failed to create pipe: %w", err))
	}

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "yq", expr)
	cmd.Stderr = os.Stderr
	cmd.Stdout = pw
	cmd.Stdin = os.Stdin
	log.Printf("Starting process: %v", cmd.Args)
	if err := cmd.Start(); err != nil {
		panic(fmt.Errorf("failed to start process: %w", err))
	}

	go func() {
		defer pw.Close()
		if err := cmd.Wait(); err != nil {
			panic(fmt.Errorf("process failed: %w", err))
		}
	}()

	validation := bytes.Buffer{}
	tee := io.TeeReader(pr, &validation)
	pipeline := kio.Pipeline{
		Inputs:  []kio.Reader{&kio.ByteReader{Reader: tee}},
		Filters: []kio.Filter{},
		Outputs: []kio.Writer{kio.ByteWriter{Writer: os.Stdout}},
	}
	if err := pipeline.Execute(); err != nil {
		if err := cmd.Wait(); err != nil {
			log.Printf("process failed: %v", err)
		}
		panic(fmt.Errorf("the YAML pipeline failed (did \"yq\" output valid YAML?): %w\n===\n%s", err, validation.String()))
	}
}
