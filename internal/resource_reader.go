package internal

import (
	"context"
	"errors"
	"fmt"
	"github.com/hashicorp/go-getter/v2"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sigs.k8s.io/kustomize/kyaml/kio"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
)

type resourceReader struct {
	useInternalFunctions bool
	logger               *log.Logger
	pwd                  string
	url                  string
	resources            []*kyaml.RNode
}

func (r *resourceReader) Read() ([]*kyaml.RNode, error) {
	ctx := context.Background()

	path, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	// We only use "ioutil.TempDir" to create a temporary name, but we might need it as a file (we don't yet know)
	// So we delete it right after...
	os.RemoveAll(path)

	client := getter.Client{}
	var result *getter.GetResult

	req := getter.Request{Src: r.url, Dst: path, Pwd: r.pwd, Copy: true, GetMode: getter.ModeAny}
	if result, err = client.Get(ctx, &req); err != nil {
		return nil, fmt.Errorf("failed to download '%s': %w", r.url, err)
	}

	if err := r.Add(result.Dst); err != nil {
		return nil, fmt.Errorf("failed to aggregate resources of '%s': %w", r.url, err)
	} else {
		return r.resources, nil
	}
}

func (r *resourceReader) Add(path string) error {
	if stat, err := os.Stat(path); err != nil {
		return fmt.Errorf("failed to stat '%s': %w", path, err)
	} else if stat.IsDir() {
		err := r.AddDirectory(path)
		if err != nil {
			return fmt.Errorf("failed to aggregate resources from '%s': %w", path, err)
		}
	} else if err := r.AddFile(path); err != nil {
		return fmt.Errorf("failed to aggregate resources from '%s': %w", path, err)
	}
	return nil
}

func (r *resourceReader) AddFile(path string) error {
	if stat, err := os.Stat(path); err != nil {
		return fmt.Errorf("could not stat '%s': %w", path, err)
	} else if stat.IsDir() {
		return fmt.Errorf("path '%s' is not a file: %w", path, err)
	}

	reader, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open '%s': %w", path, err)
	}
	defer reader.Close()

	byteReader := kio.ByteReader{Reader: reader}
	rns, err := byteReader.Read()
	if err != nil {
		return fmt.Errorf("failed to read '%s': %w", path, err)
	}
	r.resources = append(r.resources, rns...)
	return nil
}

func (r *resourceReader) AddDirectory(path string) error {
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
		// TODO: support file symlinks
		return r.AddDirectory(target)
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
			pr, pw, err := os.Pipe()
			if err != nil {
				return fmt.Errorf("failed to create pipe: %w", err)
			}

			manifestReader, err := os.Open(kudeYAMLFile)
			if err != nil {
				return fmt.Errorf("failed to open package manifest at '%s': %w", kudeYAMLFile, err)
			}

			if pipeline, err := NewPackage(ChildLogger(r.logger), path, manifestReader, pw, r.useInternalFunctions); err != nil {
				return fmt.Errorf("failed to build Kude pipeline from '%s': %w", path, err)
			} else if err := pipeline.Execute(); err != nil {
				return fmt.Errorf("failed to execute Kude pipeline in '%s': %w", path, err)
			}
			pw.Close()

			byteReader := &kio.ByteReader{Reader: pr}
			if rns, err := byteReader.Read(); err != nil {
				return fmt.Errorf("failed to read results of package at '%s': %w", path, err)
			} else {
				r.resources = append(r.resources, rns...)
			}
			return fs.SkipDir
		}
	} else if filepath.Ext(path) == ".yaml" || filepath.Ext(path) == ".yml" {
		return r.AddFile(path)
	} else {
		return nil
	}
}
