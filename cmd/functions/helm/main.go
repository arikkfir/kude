package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"github.com/arikkfir/kude/pkg"
	"github.com/spf13/viper"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"strings"
)

func downloadHelmArchive(localHelmArchive string) error {
	url := fmt.Sprintf("https://get.helm.sh/%s", filepath.Base(localHelmArchive))

	log.Printf("Downloading archive from: %s", url)
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

func extractHelm(arch, helmArchiveFile, helmFile string) error {
	log.Printf("Extracting Helm archive: %s", helmArchiveFile)

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

func main() {
	viper.SetDefault("arch", "linux-amd64")
	viper.SetDefault("helm-version", "v3.8.1")
	pkg.Configure()

	arch := viper.GetString("arch")

	helmVersion := viper.GetString("helm-version")
	if helmVersion == "" {
		panic(fmt.Errorf("version has not been provided"))
	} else if strings.HasPrefix(helmVersion, "v") {
		helmVersion = helmVersion[1:]
	}

	helmFile := "/workspace/temp/helm"
	if _, err := os.Stat(helmFile); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			helmArchiveFile := filepath.Join("/workspace/temp", "helm-v"+helmVersion+"-"+arch+".tar.gz")
			if _, err := os.Stat(helmArchiveFile); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					err := downloadHelmArchive(helmArchiveFile)
					if err != nil {
						panic(fmt.Errorf("failed to download archive: %w", err))
					}
				} else {
					panic(fmt.Errorf("failed to stat file at '%s': %w", helmArchiveFile, err))
				}
			}

			if err := extractHelm(arch, helmArchiveFile, helmFile); err != nil {
				panic(fmt.Errorf("failed to extract archive at '%s': %w", helmArchiveFile, err))
			}
		} else {
			panic(fmt.Errorf("failed to stat file at '%s': %w", helmFile, err))
		}
	}

	pr, pw, err := os.Pipe()
	if err != nil {
		panic(fmt.Errorf("failed to create pipe: %w", err))
	}

	cmd := exec.Command(helmFile, viper.GetStringSlice("args")...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = pw
	log.Printf("Starting process: %v", cmd.Args)
	if err := cmd.Start(); err != nil {
		panic(fmt.Errorf("failed to start process: %w", err))
	}

	go func() {
		defer pw.Close()
		if err := cmd.Wait(); err != nil {
			panic(fmt.Errorf("process failed: %w", err))
		}
	}()

	validation := bytes.Buffer{}
	tee := io.TeeReader(pr, &validation)
	pipeline := kio.Pipeline{
		Inputs: []kio.Reader{
			&kio.ByteReader{Reader: os.Stdin},
			&kio.ByteReader{Reader: tee},
		},
		Filters: []kio.Filter{},
		Outputs: []kio.Writer{kio.ByteWriter{Writer: os.Stdout}},
	}
	if err := pipeline.Execute(); err != nil {
		if err := cmd.Wait(); err != nil {
			log.Printf("process failed: %v", err)
		}
		panic(fmt.Errorf("the YAML pipeline failed (did \"helm\" output valid YAML?): %w\n===\n%s", err, validation.String()))
	}
}
