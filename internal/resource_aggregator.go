package internal

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sigs.k8s.io/kustomize/kyaml/kio"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
)

type resourceAggregator struct {
	logger    *log.Logger
	resources []*kyaml.RNode
}

func (r *resourceAggregator) Add(path string) error {
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

func (r *resourceAggregator) AddFile(path string) error {
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

func (r *resourceAggregator) AddDirectory(path string) error {
	err := filepath.WalkDir(path, r.walkSimpleDirectory)
	if err != nil {
		return fmt.Errorf("failed walking '%s': %w", path, err)
	}
	return nil
}

func (r *resourceAggregator) walkSimpleDirectory(path string, e fs.DirEntry, err error) error {
	if err != nil {
		return err
	} else if e.Type() == fs.ModeSymlink {
		target, err := os.Readlink(path)
		if err != nil {
			return fmt.Errorf("failed reading symlink '%s': %w", path, err)
		}
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
			var rns []*kyaml.RNode
			var prefix string
			if r.logger.Prefix() == "" {
				prefix = "---> "
			} else {
				prefix = "---" + r.logger.Prefix()
			}
			logger := log.New(r.logger.Writer(), prefix, r.logger.Flags())
			pipeline, err := NewPipeline(logger, path, kio.WriterFunc(func(_rns []*kyaml.RNode) error {
				rns = _rns
				return nil
			}))
			if err != nil {
				return fmt.Errorf("failed to build Kude pipeline from '%s': %w", path, err)
			} else if err := pipeline.Execute(); err != nil {
				return fmt.Errorf("failed to execute Kude pipeline in '%s': %w", path, err)
			}
			r.resources = append(r.resources, rns...)
			return fs.SkipDir
		}
	} else if filepath.Ext(path) == ".yaml" || filepath.Ext(path) == ".yml" {
		return r.AddFile(path)
	} else {
		return nil
	}
}
