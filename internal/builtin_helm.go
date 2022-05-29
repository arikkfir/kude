package internal

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"strings"
)

type Helm struct {
	Version  string   `json:"version" yaml:"version"`
	Args     []string `json:"args" yaml:"args"`
	logger   *log.Logger
	pwd      string
	cacheDir string
	tempDir  string
}

func (f *Helm) Configure(logger *log.Logger, pwd, cacheDir, tempDir string) error {
	f.logger = logger
	f.pwd = pwd
	f.cacheDir = cacheDir
	f.tempDir = tempDir
	return nil
}

func (f *Helm) Invoke(r io.Reader, w io.Writer) error {
	arch := runtime.GOOS + "-" + runtime.GOARCH
	if f.Version == "" {
		f.Version = "3.8.1"
	} else if strings.HasPrefix(f.Version, "v") {
		f.Version = f.Version[1:]
	}

	helmFile := filepath.Join(f.cacheDir, "helm-v"+f.Version+"-"+arch)
	if _, err := os.Stat(helmFile); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			helmArchiveFile := filepath.Join(f.tempDir, "helm-v"+f.Version+"-"+arch+".tar.gz")
			if _, err := os.Stat(helmArchiveFile); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					err := f.downloadHelmArchive(helmArchiveFile)
					if err != nil {
						return fmt.Errorf("failed to download archive: %w", err)
					}
				} else {
					return fmt.Errorf("failed to stat file at '%s': %w", helmArchiveFile, err)
				}
			}

			if err := f.extractHelm(arch, helmArchiveFile, helmFile); err != nil {
				return fmt.Errorf("failed to extract archive at '%s': %w", helmArchiveFile, err)
			}
		} else {
			return fmt.Errorf("failed to stat file at '%s': %w", helmFile, err)
		}
	}

	pr, pw, err := os.Pipe()
	if err != nil {
		panic(fmt.Errorf("failed to create pipe: %w", err))
	}

	cmd := exec.Command(helmFile, f.Args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = pw
	cmd.Dir = f.pwd
	f.logger.Printf("Starting process: %v", cmd.Args)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	} else if err := cmd.Wait(); err != nil {
		return fmt.Errorf("process failed: %w", err)
	}

	go func() {
		defer pw.Close()
	}()

	validation := bytes.Buffer{}
	tee := io.TeeReader(pr, &validation)
	pipeline := kio.Pipeline{
		Inputs: []kio.Reader{
			&kio.ByteReader{Reader: r},
			&kio.ByteReader{Reader: tee},
		},
		Filters: []kio.Filter{},
		Outputs: []kio.Writer{kio.ByteWriter{Writer: w}},
	}
	if err := pipeline.Execute(); err != nil {
		return fmt.Errorf("the YAML pipeline failed (did \"helm\" output valid YAML?): %w\n===\n%s", err, validation.String())
	}
	return nil
}

func (f *Helm) downloadHelmArchive(localHelmArchive string) error {
	url := fmt.Sprintf("https://get.helm.sh/%s", filepath.Base(localHelmArchive))

	f.logger.Printf("Downloading archive from: %s", url)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed downloading from: %w", err)
	}
	defer resp.Body.Close()

	out, err := os.Create(localHelmArchive)
	if err != nil {
		return fmt.Errorf("failed to create helm file at '%s': %w", localHelmArchive, err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write helm file to '%s': %w", localHelmArchive, err)
	}
	return nil
}

func (f *Helm) extractHelm(arch, helmArchiveFile, helmFile string) error {
	f.logger.Printf("Extracting Helm archive: %s", helmArchiveFile)

	r, err := os.Open(helmArchiveFile)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer r.Close()

	gr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return fmt.Errorf("failed to read next entry: %w", err)
			}
		}

		if hdr.Name == arch+"/helm" {
			w, err := os.Create(helmFile)
			if err != nil {
				return fmt.Errorf("failed to create '%s': %w", helmFile, err)
			} else if err := os.Chmod(helmFile, 0755); err != nil {
				return fmt.Errorf("failed to chmod '%s': %w", helmFile, err)
			}

			_, err = io.Copy(w, tr)
			if err != nil {
				return fmt.Errorf("failed to write to '%s': %w", helmFile, err)
			}
			w.Close()
			break
		}
	}
	return nil
}
