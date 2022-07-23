package functions

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/arikkfir/kude/internal/stream"
	. "github.com/arikkfir/kude/internal/stream/generate"
	. "github.com/arikkfir/kude/internal/stream/sink"
	"gopkg.in/yaml.v3"
	"io"
	"io/ioutil"
	"log"
	"path/filepath"
	"strconv"
)

type CreateSecretEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Path  string `json:"path"`
}

type CreateSecret struct {
	Name      string              `json:"name" yaml:"name"`
	Namespace string              `json:"namespace" yaml:"namespace"`
	Immutable *bool               `json:"immutable" yaml:"immutable"`
	Type      string              `json:"type" yaml:"type"`
	Contents  []CreateSecretEntry `json:"contents" yaml:"contents"`
}

func (f *CreateSecret) Invoke(_ *log.Logger, pwd, _, _ string, r io.Reader, w io.Writer) error {
	if f.Name == "" {
		return fmt.Errorf("%s is required for creating secrets", "name")
	}

	allContent := bytes.Buffer{}
	data := make(map[string]string)
	for i, content := range f.Contents {
		if content.Key == "" {
			return fmt.Errorf("key is required for all entries (missing for entry %d)", i)
		}
		if content.Value == "" && content.Path == "" {
			return fmt.Errorf("value or path is required for all entries (missing for entry %d)", i)
		}
		if content.Value != "" && content.Path != "" {
			return fmt.Errorf("value and path cannot be used together in a single entry (encountered for entry %d)", i)
		}
		var value string
		if content.Value != "" {
			value = content.Value
		} else {
			path := content.Path
			if !filepath.IsAbs(content.Path) {
				path = filepath.Join(pwd, content.Path)
			}
			contents, err := ioutil.ReadFile(path)
			if err != nil {
				return fmt.Errorf("error reading file '%s': %w", path, err)
			}
			value = string(contents)
		}
		data[content.Key] = base64.StdEncoding.EncodeToString([]byte(value))
		allContent.Write([]byte(value))
	}

	// Generate an SHA hash of the contents, for a name corresponding to data contents
	// This ensures that changes in content result in a different name
	hash := sha1.New()
	hash.Write(allContent.Bytes())
	hashedName := f.Name + "-" + hex.EncodeToString(hash.Sum(nil))

	// Execute pipeline on provided resources
	s := stream.NewStream().
		Generate(FromReader(r)).
		Generate(func(ctx context.Context, c chan *yaml.Node) error {
			dataContents := make([]*yaml.Node, 0, len(data)*2)
			for k, v := range data {
				dataContents = append(
					dataContents,
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k},
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v},
				)
			}
			metadataContents := []*yaml.Node{
				{Kind: yaml.ScalarNode, Tag: "!!str", Value: "name"},
				{Kind: yaml.ScalarNode, Tag: "!!str", Value: hashedName},
				{Kind: yaml.ScalarNode, Tag: "!!str", Value: "annotations"},
				{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{
					{Kind: yaml.ScalarNode, Tag: "!!str", Value: "kude.kfirs.com/previous-name"},
					{Kind: yaml.ScalarNode, Tag: "!!str", Value: f.Name},
				}},
			}
			secretNode := &yaml.Node{
				Kind: yaml.MappingNode,
				Tag:  "!!map",
				Content: []*yaml.Node{
					{Kind: yaml.ScalarNode, Tag: "!!str", Value: "apiVersion"},
					{Kind: yaml.ScalarNode, Tag: "!!str", Value: "v1"},
					{Kind: yaml.ScalarNode, Tag: "!!str", Value: "kind"},
					{Kind: yaml.ScalarNode, Tag: "!!str", Value: "Secret"},
					{Kind: yaml.ScalarNode, Tag: "!!str", Value: "data"},
					{Kind: yaml.MappingNode, Tag: "!!map", Content: dataContents},
					{Kind: yaml.ScalarNode, Tag: "!!str", Value: "metadata"},
					{Kind: yaml.MappingNode, Tag: "!!map", Content: metadataContents},
				},
			}
			if f.Immutable != nil {
				secretNode.Content = append(
					secretNode.Content,
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "immutable"},
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(*f.Immutable)},
				)
			}
			if f.Type != "" {
				secretNode.Content = append(
					secretNode.Content,
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "type"},
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: f.Type},
				)
			}
			if f.Namespace != "" {
				metadataContents = append(
					metadataContents,
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "namespace"},
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: f.Namespace},
				)
			}
			c <- secretNode
			return nil
		}).
		Sink(ToWriter(w))
	if err := s.Execute(context.Background()); err != nil {
		return fmt.Errorf("failed executing stream: %w", err)
	}

	return nil
}
