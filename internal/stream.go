package internal

import (
	"fmt"
	"github.com/hashicorp/go-getter"
	"gopkg.in/yaml.v3"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path"
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
	if strings.HasSuffix(strings.ToLower(url), ".yaml") {
		return s.downloadAndAddFile(url)
	} else {
		return s.downloadAndAddDirectory(url)
	}
}

func (s *stream) downloadAndAddFile(url string) error {
	tempFile, err := ioutil.TempFile("", path.Base(url)+".*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() { os.Remove(tempFile.Name()) }()

	err = getter.GetFile(tempFile.Name(), url, s.pwdGetterClientOption)
	if err != nil {
		return fmt.Errorf("failed to download %s: %w", url, err)
	}

	return s.addFile(tempFile.Name())
}

func (s *stream) downloadAndAddDirectory(url string) error {
	tempDir, err := ioutil.TempDir("", path.Base(url)+".*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() { os.RemoveAll(tempDir) }()

	err = getter.Get(tempDir, url, s.pwdGetterClientOption)
	if err != nil {
		return fmt.Errorf("failed to download %s: %w", url, err)
	}

	manifest := filepath.Join(tempDir, "kude.yaml")
	stat, err := os.Stat(manifest)
	if err != nil {
		if os.IsNotExist(err) {
			return s.addSimpleDirectory(tempDir)
		} else {
			return fmt.Errorf("failed inspecting '%s' (for '%s'): %w", manifest, url, err)
		}
	} else if stat.IsDir() {
		return fmt.Errorf("illegal package - expected '%s' to be a file, not a directory", filepath.Join(url, "kude.yaml"))
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
	err = kude.executePipeline(w)
	if err != nil {
		return fmt.Errorf("failed evaluating kude package at '%s': %w", dir, err)
	}
	return s.addReader(r)
}

func (s *stream) addSimpleDirectory(dir string) error {
	err := filepath.WalkDir(dir, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("failed walking '%s': %w", path, err)
		}
		if !e.IsDir() && filepath.Ext(path) == ".yaml" {
			return s.addFile(path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed walking '%s': %w", dir, err)
	}
	return nil
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
