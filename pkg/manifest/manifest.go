// Package manifest provides a basis for describing a resource of any type.
package manifest

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/ghodss/yaml"
	json "github.com/json-iterator/go"
	"github.com/tidwall/sjson"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"time"
)

// manifest represents data that is used to instantiate a resource.
type Manifest struct {
	Selector *Selector
	// Meta provides optional additional information about the resource.
	Meta *Meta
	// Spec contains a json-encoded byte array holding details specific to the
	// KGN (Kind Group Version) of the manifest.
	Spec json.RawMessage
	// Raw holds a copy of the raw data provided when instantiating a manifest.
	Raw []byte
	// Json holds a copy of the entire manifest as a json object regardless of
	// the format of the source.
	Json json.RawMessage
	// source indicates where the manifest originated.
	source string
}

// UnmarshalJSON does just what you think it does.
func (m *Manifest) UnmarshalJSON(data []byte) error {
	var temp struct {
		Kind      string
		Group     string
		Version   string
		Name      string
		Namespace string
		Live      bool
		PublishAt time.Time
		BaseHref  string
		Meta      *Meta
		Spec      json.RawMessage
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return fmt.Errorf("json unmarshal: %w", err)
	}
	s, err := NewSelector(fmt.Sprintf("%s/%s/%s/%s/%s", temp.Kind, temp.Group, temp.Version, temp.Namespace, temp.Name))
	if err != nil {
		return err
	}
	*m = Manifest{
		Selector: s,
		Meta:     temp.Meta,
		Spec:     temp.Spec,
		Raw:      data,
	}
	return nil
}

// Validate does just what you think it does.
func (m *Manifest) Validate() error {
	if m.Meta != nil {
		return m.Meta.Validate()
	}
	return nil
}

// String returns an identifier referencing where the manifest originated.
func (m *Manifest) String() string {
	return fmt.Sprintf("%s (%s)", m.Selector, m.source)
}

// BaseHref returns the underlying base href.
func (m *Manifest) BaseHref() string {
	if m.Meta == nil {
		return ""
	}
	return m.Meta.BaseHref
}

// EachEmbed executes a supplied function once for every embed of this manifest.
// TODO: DRY with EachChild
func (m *Manifest) EachEmbed(index *Index, fn func(*RenderTarget) error) error {
	for name, child := range m.Meta.Embed {
		expanded, err := child.Expand(index, name)
		if err != nil {
			return err
		}
		for _, item := range expanded {
			if err := fn(item); err != nil {
				return err
			}
		}
	}
	return nil
}

// EachChild executes a supplied function once for every child of this manifest.
// TODO: DRY with EachEmbed
func (m *Manifest) EachChild(index *Index, fn func(*RenderTarget) error) error {
	for _, child := range m.Meta.RenderAsChild {
		expanded, err := child.Expand(index, child.Selector.ID())
		if err != nil {
			return err
		}
		for _, item := range expanded {
			if err := fn(item); err != nil {
				return err
			}
		}
	}
	return nil
}

// relations resolves an array of selectors for all related and embedded
// manifests.
func (m *Manifest) relations(index *Index) ([]*Selector, error) {
	if m.Meta == nil {
		return nil, nil
	}
	var selectors []*Selector
	// Collect selectors inside embeds.
	for _, child := range m.Meta.Embed {
		relatedSelectors, err := child.Resolve(index)
		if err != nil {
			return nil, err
		}
		selectors = append(selectors, relatedSelectors...)
	}
	// Collect selectors inside relations.
	for _, related := range m.Meta.Relations {
		relatedSelectors, err := related.Resolve(index)
		if err != nil {
			return nil, err
		}
		selectors = append(selectors, relatedSelectors...)
	}
	return selectors, nil
}

// IsLive determines if the manifest is considered published.
func (m *Manifest) IsLive() bool {
	if m.Meta == nil {
		return false
	}
	if !m.Meta.Live {
		return false
	}
	if !m.Meta.PublishAt.Date().IsZero() {
		return time.Now().After(m.Meta.PublishAt.Date())
	}
	return true
}

// Greater determines if the receiver manifest was published after the compared.
// In the absence of a publish date, fall back to alphanumeric sorting by ID.
func (m *Manifest) Greater(compare *Manifest) bool {
	if compare == nil || compare.Meta == nil {
		return false
	}
	if m.Meta.PublishAt.Date().IsZero() || compare.Meta.PublishAt.Date().IsZero() {
		return m.Selector.ID() > compare.Selector.ID()
	}
	return m.Meta.PublishAt.Date().Before(compare.Meta.PublishAt.Date())
}

// Less determines if the receiver manifest was published before the compared.
// In the absence of a publish date, fall back to alphanumeric sorting by ID.
func (m *Manifest) Less(compare *Manifest) bool {
	if compare == nil || compare.Meta == nil {
		return false
	}
	if m.Meta.PublishAt.Date().IsZero() || compare.Meta.PublishAt.Date().IsZero() {
		return m.Selector.ID() < compare.Selector.ID()
	}
	return m.Meta.PublishAt.Date().After(compare.Meta.PublishAt.Date())
}

// Equal determines if the receiver manifest was published at the same time as
// the compared. In the absence of a publish date, fall back to alphanumeric
// sorting by ID.
func (m *Manifest) Equal(compare *Manifest) bool {
	if compare == nil || compare.Meta == nil {
		return false
	}
	if m.Meta.PublishAt.Date().IsZero() || compare.Meta.PublishAt.Date().IsZero() {
		return true
	}
	return m.Meta.PublishAt.Date().Equal(compare.Meta.PublishAt.Date())
}

// Generate produces an array of manifests for all manifests that may be
// configured to be generated from within the receiver manifest.
func (m *Manifest) Generate() ([]*Manifest, error) {
	var manifests []*Manifest
	if m.Meta == nil || m.Meta.GenerateManifests == nil {
		return manifests, nil
	}
	for _, generator := range m.Meta.GenerateManifests {
		created, err := NewFromGenerator(generator, m)
		if err != nil {
			return nil, fmt.Errorf("%s: generateManifests: %s: %w", m, generator.Name, err)
		}
		manifests = append(manifests, created...)
	}
	return manifests, nil
}

// New creates a manifest from a json-encoded byte array or a yaml-front-matter
// having byte array. If front-matter is found, the content below it is assigned
// to `.Spec.Body` (overwriting any content that may be there).
func New(data []byte, source string) ([]*Manifest, error) {
	var delim = []byte("---")
	var err error
	body := append([]byte{}, data...)
	// Process front-matter, if any.
	if bytes.HasPrefix(body, delim) {
		parts := bytes.SplitN(body, delim, 3)
		if body, err = yaml.YAMLToJSON(parts[1]); err != nil {
			return nil, err
		}
		if len(parts[2]) > 0 {
			if body, err = sjson.SetBytes(body, "spec.body", parts[2]); err != nil {
				return nil, err
			}
		}
	}
	var manifest *Manifest
	if err = json.Unmarshal(body, &manifest); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}
	manifest.Raw = data
	manifest.Json = body
	if err := manifest.Validate(); err != nil {
		return nil, err
	}
	manifest.source = source
	// generate manifests embedded in this one
	generated, generateErr := manifest.Generate()
	if generateErr != nil {
		return nil, generateErr
	}
	if len(generated) > 0 {
		return append(generated, manifest), nil
	}
	return []*Manifest{manifest}, nil
}

// NewFromFile creates a manifest from a source file.
func NewFromFile(filepath string) ([]*Manifest, error) {
	data, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	if path.Ext(filepath) == ".yml" {
		data, err = yaml.YAMLToJSON(data)
		if err != nil {
			return nil, fmt.Errorf("%s: yaml to json failure: %w", filepath, err)
		}
	}
	manifest, newErr := New(data, filepath)
	if newErr != nil {
		return nil, fmt.Errorf("%s: %w", filepath, newErr)
	}
	return manifest, nil
}

// NewFromReader creates an array of manifests from a provided reader taking the
// assumption that the reader contains one manifest per line.
func NewFromReader(input io.Reader) ([]*Manifest, error) {
	reader := bufio.NewReader(input)
	queue := make(chan *Manifest)
	eg := errgroup.Group{}
	for {
		raw, err := reader.ReadBytes('\n')
		if errors.Is(err, io.EOF) {
			break
		}
		if len(bytes.TrimSpace(raw)) == 0 {
			continue
		}
		eg.Go(func() error {
			manifests, err := New(bytes.TrimRight(raw, "\n"), "stream")
			if err != nil {
				return fmt.Errorf("%s: %w", raw, err)
			}
			for _, manifest := range manifests {
				queue <- manifest
			}
			return nil
		})
	}
	collector := errgroup.Group{}
	var manifests []*Manifest
	collector.Go(func() error {
		for manifest := range queue {
			manifests = append(manifests, manifest)
		}
		return nil
	})
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	close(queue)
	collector.Wait()
	return manifests, nil
}

// NewFromDirectory creates an array of manifests from a provided directory by
// traversing every file in every directory from a specified parent.
func NewFromDirectory(dir string) ([]*Manifest, error) {
	queue := make(chan *Manifest)
	eg, egCtx := errgroup.WithContext(context.Background())
	sem := semaphore.NewWeighted(10)
	eg.Go(func() error {
		return filepath.Walk(dir, func(path string, f os.FileInfo, err error) error {
			if f.IsDir() {
				return nil
			}
			if err := sem.Acquire(egCtx, 1); err != nil {
				return err
			}
			eg.Go(func() error {
				defer sem.Release(1)
				manifests, err := NewFromFile(path)
				if err != nil {
					return err
				}
				for _, manifest := range manifests {
					queue <- manifest
				}
				return nil

			})
			return nil
		})
	})
	collector := errgroup.Group{}
	var manifests []*Manifest
	collector.Go(func() error {
		for manifest := range queue {
			manifests = append(manifests, manifest)
		}
		return nil
	})
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	close(queue)
	collector.Wait()
	return manifests, nil
}
