package internal

import (
	"context"
	"fmt"
	"github.com/hashicorp/go-getter/v2"
	"io/ioutil"
	"os"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
)

type resourceReader struct {
	pwd string
	url string
}

func (rr *resourceReader) Read() ([]*kyaml.RNode, error) {
	ctx := context.Background()

	path, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	// We only use "ioutil.TempDir" to create a temporary name, but we might need it as a file (we don't yet know)
	// So we delete it right after...
	os.RemoveAll(path)

	client := getter.Client{}
	var result *getter.GetResult

	req := getter.Request{Src: rr.url, Dst: path, Pwd: rr.pwd, Copy: true, GetMode: getter.ModeAny}
	if result, err = client.Get(ctx, &req); err != nil {
		return nil, fmt.Errorf("failed to download '%s': %w", rr.url, err)
	}

	agg := resourceAggregator{}
	//TODO: defer os.RemoveAll(path)

	if err := agg.Add(result.Dst); err != nil {
		return nil, fmt.Errorf("failed to aggregate resources of '%s': %w", rr.url, err)
	} else {
		return agg.resources, nil
	}
}
