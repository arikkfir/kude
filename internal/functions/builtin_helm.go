package functions

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"github.com/arikkfir/gstream/pkg"
	. "github.com/arikkfir/gstream/pkg/generate"
	. "github.com/arikkfir/gstream/pkg/sink"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

type Helm struct {
	Version string   `json:"version" yaml:"version"`
	Args    []string `json:"args" yaml:"args"`
}

func (f *Helm) Invoke(logger *log.Logger, pwd, cacheDir, tempDir string, r io.Reader, w io.Writer) error {
	arch := runtime.GOOS + "-" + runtime.GOARCH
	if f.Version == "" {
		f.Version = "3.8.1"
	} else if strings.HasPrefix(f.Version, "v") {
		f.Version = f.Version[1:]
	}

	helmFile := filepath.Join(cacheDir, "helm-v"+f.Version+"-"+arch)
	if _, err := os.Stat(helmFile); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			helmArchiveFile := filepath.Join(tempDir, "helm-v"+f.Version+"-"+arch+".tar.gz")
			if _, err := os.Stat(helmArchiveFile); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					err := f.downloadHelmArchive(logger, helmArchiveFile)
					if err != nil {
						return fmt.Errorf("failed to download archive: %w", err)
					}
				} else {
					return fmt.Errorf("failed to stat file at '%s': %w", helmArchiveFile, err)
				}
			}

			if err := f.extractHelm(logger, arch, helmArchiveFile, helmFile); err != nil {
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

	exitCh := make(chan error, 2)
	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer pw.Close()
		cmd := exec.Command(helmFile, f.Args...)
		cmd.Stderr = os.Stderr
		cmd.Stdout = pw
		cmd.Dir = pwd
		logger.Printf("Starting process: %v", cmd.Args)
		if err := cmd.Run(); err != nil {
			exitCh <- fmt.Errorf("process failed: %w", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		s := stream.NewStream().
			Generate(FromReader(r)).
			Generate(FromReader(pr)).
			Sink(ToWriter(w))
		if err := s.Execute(context.Background()); err != nil {
			exitCh <- fmt.Errorf("failed executing stream: %w", err)
		}
	}()

	wg.Wait()
	select {
	case err := <-exitCh:
		return err
	default:
		return nil
	}
}

func (f *Helm) downloadHelmArchive(logger *log.Logger, localHelmArchive string) error {
	url := fmt.Sprintf("https://get.helm.sh/%s", filepath.Base(localHelmArchive))

	logger.Printf("Downloading archive from: %s", url)
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

func (f *Helm) extractHelm(logger *log.Logger, arch, helmArchiveFile, helmFile string) error {
	logger.Printf("Extracting Helm archive: %s", helmArchiveFile)

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
