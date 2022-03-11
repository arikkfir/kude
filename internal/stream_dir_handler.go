package internal

import (
	"fmt"
	ss "github.com/arikkfir/kude/internal/stream"
	"os"
	"path/filepath"
)

func handleDirectory(url string, path string, s ss.Stream) error {
	kudeManifestURL := filepath.Join(url, "kude.yaml")
	kudeManifestFile := filepath.Join(path, "kude.yaml")
	kudeManifestStat, err := os.Stat(kudeManifestFile)
	if err != nil {
		if os.IsNotExist(err) {
			err := s.AddLocalDirectory(path)
			if err != nil {
				return fmt.Errorf("error adding directory '%s' ('%s'): %s", kudeManifestFile, kudeManifestURL, err)
			}
			return nil
		} else {
			return fmt.Errorf("failed inspecting '%s' ('%s'): %w", kudeManifestFile, kudeManifestURL, err)
		}
	} else if kudeManifestStat.IsDir() {
		return fmt.Errorf("illegal package! '%s' must be a file, not a directory", kudeManifestURL)
	} else {
		return handleKudeDirectory(url, path, s)
	}
}

func handleKudeDirectory(url string, path string, s ss.Stream) error {
	// Read pipeline
	kude, err := CreatePipeline(path)
	if err != nil {
		return fmt.Errorf("failed reading kude package at '%s' ('%s'): %w", path, url, err)
	}

	// Execute pipeline
	r, w, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("failed creating pipe: %w", err)
	}
	defer w.Close()

	err = kude.executePipeline(w)
	if err != nil {
		return fmt.Errorf("failed evaluating kude package at '%s' ('%s'): %w", path, url, err)
	}
	w.Close() // required in order for reads from "r" not to block indefinitely...
	return s.AddReader(r)
}
