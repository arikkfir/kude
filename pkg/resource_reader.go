package kude

import (
	"context"
	"errors"
	"fmt"
	"github.com/arikkfir/kude/internal"
	"github.com/arikkfir/kyaml/pkg"
	"github.com/hashicorp/go-getter/v2"
	"gopkg.in/yaml.v3"
	"io"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

type resourceReader struct {
	ctx    context.Context
	pwd    string
	logger *log.Logger
	target chan *kyaml.RNode
}

func (r *resourceReader) Read(url string) error {
	path, err := ioutil.TempDir("", "")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	// We only use "ioutil.TempDir" to create a temporary name, but we might need it as a file (we don't yet know)
	// So we delete it right after...
	os.RemoveAll(path)

	client := getter.Client{}
	var result *getter.GetResult

	req := getter.Request{Src: url, Dst: path, Pwd: r.pwd, Copy: true, GetMode: getter.ModeAny}
	if result, err = client.Get(r.ctx, &req); err != nil {
		return fmt.Errorf("failed to download '%s': %w", url, err)
	}

	if err := r.process(result.Dst); err != nil {
		return fmt.Errorf("failed to stream resources of '%s': %w", url, err)
	}
	return nil
}

func (r *resourceReader) process(path string) error {
	if stat, err := os.Stat(path); err != nil {
		return fmt.Errorf("failed to stat '%s': %w", path, err)
	} else if stat.IsDir() {
		err := r.processDirectory(path)
		if err != nil {
			return fmt.Errorf("failed to aggregate resources from '%s': %w", path, err)
		}
	} else if err := r.processFile(path); err != nil {
		return fmt.Errorf("failed to aggregate resources from '%s': %w", path, err)
	}
	return nil
}

func (r *resourceReader) processFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open '%s': %w", path, err)
	}

	r.logger.Printf("Processing: %s", path)
	decoder := yaml.NewDecoder(f)
	for {
		node := &yaml.Node{}
		if err := decoder.Decode(node); err != nil {
			if errors.Is(err, io.EOF) {
				break
			} else {
				return fmt.Errorf("failed to parse '%s': %w", path, err)
			}
		}
		if node.Kind == yaml.DocumentNode {
			node = node.Content[0]
		}
		r.target <- &kyaml.RNode{N: node}
	}
	return nil
}

func (r *resourceReader) processDirectory(path string) error {
	err := filepath.WalkDir(path, r.walkSimpleDirectory)
	if err != nil {
		return fmt.Errorf("failed walking '%s': %w", path, err)
	}
	return nil
}

func (r *resourceReader) walkSimpleDirectory(path string, e fs.DirEntry, err error) error {
	if err != nil {
		return err
	} else if e.Type() == fs.ModeSymlink {
		target, err := os.Readlink(path)
		if err != nil {
			return fmt.Errorf("failed reading symlink '%s': %w", path, err)
		}
		// TODO: support file symlinks (not just directories)
		return r.processDirectory(target)
	} else if e.IsDir() {
		kudeYAMLFile := filepath.Join(path, "kude.yaml")
		if stat, err := os.Stat(kudeYAMLFile); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				// no kude.yaml inside this directory; let walker traverse into this path
				return nil
			} else {
				return fmt.Errorf("failed to stat '%s': %w", kudeYAMLFile, err)
			}
		} else if stat.IsDir() {
			return fmt.Errorf("expecting 'kude.yaml' to be a file, not a directory: %s", kudeYAMLFile)
		} else {
			r.logger.Printf("Processing pipeline: %s", path)
			p, err := NewPipeline(path)
			if err != nil {
				return fmt.Errorf("failed to create pipeline from '%s': %w", path, err)
			}

			e, err := NewExecution(p, internal.NamedLogger(r.logger, filepath.Base(path)))
			if err != nil {
				return fmt.Errorf("failed to create execution for pipeline in '%s': %w", path, err)
			}

			if err := e.ExecuteToChannel(r.ctx, r.target); err != nil {
				return fmt.Errorf("failed to execute pipeline in '%s': %w", path, err)
			}
			return fs.SkipDir
		}
	} else if filepath.Ext(path) == ".yaml" || filepath.Ext(path) == ".yml" {
		return r.processFile(path)
	} else {
		return nil
	}
}
