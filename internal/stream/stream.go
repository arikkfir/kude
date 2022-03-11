package stream

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
	pwd              string
	directoryHandler DirectoryHandler
	encoder          *yaml.Encoder
}

type Stream interface {
	Close() error
	Add(url string) error
	AddLocalDirectory(path string) error
	AddLocalFile(path string) error
	AddReader(r io.Reader) error
}

type DirectoryHandler func(url, path string, s Stream) error

func NewStream(pwd string, directoryHandler DirectoryHandler, w io.Writer) Stream {
	encoder := yaml.NewEncoder(w)
	encoder.SetIndent(2)
	return &stream{pwd: pwd, directoryHandler: directoryHandler, encoder: encoder}
}

func (s *stream) Close() error {
	return s.encoder.Close()
}

func (s *stream) Add(url string) error {
	tempDir, err := ioutil.TempDir("", "")
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

	if s.directoryHandler != nil {
		err := s.directoryHandler(url, tempDir, s)
		if err != nil {
			return fmt.Errorf("failed to handle directory '%s' ('%s'): %w", tempDir, url, err)
		}
		return nil
	} else {
		return fmt.Errorf("no directory handler defined")
	}
}

func (s *stream) AddLocalDirectory(path string) error {
	err := filepath.WalkDir(path, s.walkSimpleDirectory)
	if err != nil {
		return fmt.Errorf("failed walking '%s': %w", path, err)
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
		return s.AddLocalFile(path)
	} else {
		return nil
	}
}

func (s *stream) AddLocalFile(file string) error {
	reader, err := os.Open(file)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", file, err)
	}
	defer reader.Close()
	return s.AddReader(reader)
}

func (s *stream) AddReader(reader io.Reader) error {
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
