package internal

import (
	"fmt"
	"github.com/hashicorp/go-getter"
	"gopkg.in/yaml.v3"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type stream struct {
	pwd     string
	encoder *yaml.Encoder
}

func NewStream(pwd string, w io.Writer) *stream {
	encoder := yaml.NewEncoder(w)
	encoder.SetIndent(2)
	return &stream{pwd: pwd, encoder: encoder}
}

func (s *stream) Close() error {
	return s.encoder.Close()
}

func (s *stream) Add(url string) error {
	safeLocalName := url
	safeLocalName = strings.ReplaceAll(safeLocalName, ".", "${dot}")
	safeLocalName = strings.ReplaceAll(safeLocalName, "/", "${bckslash}")
	safeLocalName = strings.ReplaceAll(safeLocalName, ":", "${colon}")
	safeLocalName = strings.ReplaceAll(safeLocalName, "\\", "${fwdslash}")
	tempDir, err := ioutil.TempDir("", safeLocalName+".*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() { os.RemoveAll(tempDir) }()

	detected, err := getter.Detect(url, s.pwd, getter.Detectors)
	if err != nil {
		return fmt.Errorf("failed to detect type of '%s': %w", url, err)
	}
	if strings.HasPrefix(detected, "file://") {
		// go-getter complains when it tries to download a file from a "file://" URLs and dst already exists
		os.RemoveAll(tempDir)
	}

	err = getter.GetAny(tempDir, url, s.pwdGetterClientOption)
	if err != nil {
		return fmt.Errorf("failed to download %s: %w", url, err)
	}

	kudeManifestFile := filepath.Join(tempDir, "kude.yaml")
	kudeManifestStat, err := os.Stat(kudeManifestFile)
	if err != nil {
		if os.IsNotExist(err) {
			return s.addSimpleDirectory(tempDir)
		} else {
			return fmt.Errorf("failed inspecting '%s' (for '%s'): %w", kudeManifestFile, url, err)
		}
	} else if kudeManifestStat.IsDir() {
		return fmt.Errorf("illegal package! '%s' must be a file, not a directory", filepath.Join(url, "kude.yaml"))
	} else {
		return s.addKudeDirectory(tempDir)
	}
}

func (s *stream) addKudeDirectory(dir string) error {
	// Read pipeline
	kude, err := CreatePipeline(dir)
	if err != nil {
		return fmt.Errorf("failed reading kude package at '%s': %w", dir, err)
	}

	// Execute pipeline
	r, w, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("failed creating pipe: %w", err)
	}
	defer w.Close()

	err = kude.executePipeline(w)
	if err != nil {
		return fmt.Errorf("failed evaluating kude package at '%s': %w", dir, err)
	}
	w.Close() // required in order for reads from "r" not to block indefinitely...
	return s.addReader(r)
}

func (s *stream) addSimpleDirectory(dir string) error {
	err := filepath.WalkDir(dir, s.walkSimpleDirectory)
	if err != nil {
		return fmt.Errorf("failed walking '%s': %w", dir, err)
	}
	return nil
}

func (s *stream) walkSimpleDirectory(path string, e fs.DirEntry, err error) error {
	if err != nil {
		return fmt.Errorf("failed walking '%s': %w", path, err)
	}
	if e.Type() == fs.ModeSymlink {
		target, err := os.Readlink(path)
		if err != nil {
			return fmt.Errorf("failed reading symlink '%s': %w", path, err)
		}
		return filepath.WalkDir(target, s.walkSimpleDirectory)
	} else if !e.IsDir() && filepath.Ext(path) == ".yaml" {
		return s.addFile(path)
	} else {
		return nil
	}
}

func (s *stream) addFile(file string) error {
	reader, err := os.Open(file)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", file, err)
	}
	defer reader.Close()
	return s.addReader(reader)
}

func (s *stream) addReader(reader io.Reader) error {
	decoder := yaml.NewDecoder(reader)
	for {
		var yamlStruct interface{}
		err := decoder.Decode(&yamlStruct)
		if err == io.EOF {
			break
		} else if err != nil {
			return fmt.Errorf("failed decoding YAML: %w", err)
		}
		err = s.encoder.Encode(yamlStruct)
		if err != nil {
			return fmt.Errorf("failed encoding YAML: %w", err)
		}
	}
	return nil
}

func (s *stream) pwdGetterClientOption(client *getter.Client) error {
	client.Pwd = s.pwd
	return nil
}
