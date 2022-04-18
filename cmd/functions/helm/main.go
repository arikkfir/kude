package main

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"github.com/arikkfir/kude/pkg"
	"github.com/spf13/viper"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"strings"
)

func downloadHelmArchive(localHelmArchive string) error {
	url := fmt.Sprintf("https://get.helm.sh/%s", filepath.Base(localHelmArchive))

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download helm: %w", err)
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
	r, err := os.Open(helmArchiveFile)
	if err != nil {
		return fmt.Errorf("failed to open helm archive '%s': %w", helmArchiveFile, err)
	}
	defer r.Close()

	gr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader for helm archive '%s': %w", helmArchiveFile, err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return fmt.Errorf("failed to read next entry from helm archive '%s': %w", helmArchiveFile, err)
			}
		}

		if hdr.Name == arch+"/helm" {
			w, err := os.Create(helmFile)
			if err != nil {
				return fmt.Errorf("failed to create helm file '%s': %w", helmFile, err)
			} else if err := os.Chmod(helmFile, 0755); err != nil {
				return fmt.Errorf("failed to chmod helm file '%s': %w", helmFile, err)
			}

			_, err = io.Copy(w, tr)
			if err != nil {
				return fmt.Errorf("failed to write helm file '%s': %w", helmFile, err)
			}
			w.Close()
			break
		}
	}
	return nil
}

func main() {
	viper.SetDefault("workspace", "/workspace/temp")
	viper.SetDefault("arch", "linux-amd64")
	viper.SetDefault("helm-version", "v3.8.1")
	pkg.Configure()

	root := viper.GetString("workspace")
	arch := viper.GetString("arch")

	helmVersion := viper.GetString("helm-version")
	if helmVersion == "" {
		panic(fmt.Errorf("helm version is not set"))
	} else if strings.HasPrefix(helmVersion, "v") {
		helmVersion = helmVersion[1:]
	}

	helmFile := filepath.Join(root, "helm")
	if _, err := os.Stat(helmFile); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			helmArchiveFile := filepath.Join(root, "helm-v"+helmVersion+"-"+arch+".tar.gz")
			if _, err := os.Stat(helmArchiveFile); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					err := downloadHelmArchive(helmArchiveFile)
					if err != nil {
						panic(err)
					}
				} else {
					panic(err)
				}
			}

			if err := extractHelm(arch, helmArchiveFile, helmFile); err != nil {
				panic(err)
			}
		} else {
			panic(err)
		}
	}

	pr, pw, err := os.Pipe()
	if err != nil {
		panic(err)
	}

	var args = viper.GetStringSlice("args")
	cmd := exec.Command(helmFile, args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = pw
	if viper.IsSet("path") {
		cmd.Dir = filepath.Join(root, viper.GetString("path"))
	}
	err = cmd.Run()
	if err != nil {
		panic(err)
	}
	pw.Close()

	pipeline := kio.Pipeline{
		Inputs: []kio.Reader{
			&kio.ByteReader{Reader: os.Stdin},
			&kio.ByteReader{Reader: pr},
		},
		Filters: []kio.Filter{},
		Outputs: []kio.Writer{kio.ByteWriter{Writer: os.Stdout}},
	}
	if err := pipeline.Execute(); err != nil {
		panic(err)
	}
}
