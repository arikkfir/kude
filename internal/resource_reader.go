package internal

import (
	"fmt"
	"github.com/hashicorp/go-getter"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"sigs.k8s.io/kustomize/kyaml/kio"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
	"strings"
)

type resourceReader struct {
	pwd string
	url string
}

func (rr *resourceReader) Read() ([]*kyaml.RNode, error) {
	localDir, cleanup, err := rr.downloadURL()
	defer cleanup()
	if err != nil {
		return nil, fmt.Errorf("failed to download '%s': %w", rr.url, err)
	}
	return rr.readDownloadedDirectory(localDir)
}

func (rr *resourceReader) downloadURL() (path string, cleanup func(), err error) {
	path, err = ioutil.TempDir("", "")
	if err != nil {
		return path, cleanup, fmt.Errorf("failed to create temp directory: %w", err)
	}
	cleanup = func() { os.RemoveAll(path) }

	detected, err := getter.Detect(rr.url, rr.pwd, getter.Detectors)
	if err != nil {
		return path, cleanup, fmt.Errorf("failed to detect type of '%s': %w", rr.url, err)
	}
	if strings.HasPrefix(detected, "file://") {
		// go-getter complains when it tries to download a file from a "file://" URLs and dst already exists
		os.RemoveAll(path)
	}

	err = getter.GetAny(path, rr.url, rr.pwdGetterClientOption)
	if err != nil {
		return path, cleanup, fmt.Errorf("failed to download %s: %w", rr.url, err)
	}
	return path, cleanup, err
}

func (rr *resourceReader) readDownloadedDirectory(path string) ([]*kyaml.RNode, error) {
	kudeManifestURL := filepath.Join(rr.url, "kude.yaml")
	kudeManifestFile := filepath.Join(path, "kude.yaml")

	kudeManifestStat, err := os.Stat(kudeManifestFile)
	if err != nil {
		if os.IsNotExist(err) {
			// no "kude.yaml"
			aggregator := &resourceAggregator{}
			err := filepath.WalkDir(path, aggregator.walkSimpleDirectory)
			if err != nil {
				return nil, fmt.Errorf("failed walking '%s' ('%s'): %w", path, rr.url, err)
			}
			return aggregator.resources, nil
		} else {
			return nil, fmt.Errorf("failed inspecting '%s' ('%s'): %w", kudeManifestFile, kudeManifestURL, err)
		}
	} else if kudeManifestStat.IsDir() {
		return nil, fmt.Errorf("illegal package! '%s' must be a file, not a directory", kudeManifestURL)
	} else {
		var rns []*kyaml.RNode
		pipeline, err := BuildPipeline(path, kio.WriterFunc(func(_rns []*kyaml.RNode) error {
			rns = _rns
			return nil
		}))
		if err != nil {
			return nil, fmt.Errorf("failed to build pipeline: %w", err)
		} else if err := pipeline.Execute(); err != nil {
			return nil, fmt.Errorf("failed to execute pipeline: %w", err)
		}
		return rns, nil
	}
}

func (rr *resourceReader) pwdGetterClientOption(client *getter.Client) error {
	client.Pwd = rr.pwd
	return nil
}

type resourceAggregator struct {
	resources []*kyaml.RNode
}

func (r *resourceAggregator) walkSimpleDirectory(path string, e fs.DirEntry, err error) error {
	if err != nil {
		return fmt.Errorf("failed walking '%s': %w", path, err)
	}
	if e.Type() == fs.ModeSymlink {
		target, err := os.Readlink(path)
		if err != nil {
			return fmt.Errorf("failed reading symlink '%s': %w", path, err)
		}
		return filepath.WalkDir(target, r.walkSimpleDirectory)
	} else if !e.IsDir() && filepath.Ext(path) == ".yaml" {
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
	}
	return nil
}
