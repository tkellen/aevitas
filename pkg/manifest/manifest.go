// Package manifest provides a basis for describing a resource of any type along
// with any resources it depends on.
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
)

// Manifest defines the data that is used to instantiate a resource.
type Manifest struct {
	Selector *Selector
	Meta     *Meta
	// Spec contains a json-encoded byte array holding details specific to the
	// KGV (Kind Group Version) of the manifest.
	Spec json.RawMessage
	// Raw holds a copy of the raw data provided when instantiating a manifest.
	Raw []byte
}

// Meta provides details about the resource such as where the backing data for
// it can be found.
type Meta struct {
	File    string
	Related []*Selector
	// Includes allows a manifest to express a dependency on other resources.
	Include []*Include
}

// Include describes how a manifest can depend on other manifests.
type Include struct {
	// Resource points to a resource that is required for rendering the
	// manifest this import belongs to.
	Resource *Selector
	// Templates optionally points to resources that should be used as a "view"
	// of the resource.
	Templates []*Selector
	// As provides an alternative name for referring to the imported resource.
	As string
	// An optional prefix for the resource this points to.
	BaseHref string
	// A filter to limit the matches on a wildcard selector for resource
	Filter *Filter
}

func (i *Include) ID() string {
	var buffer bytes.Buffer
	buffer.WriteString(i.As)
	for _, t := range i.Templates {
		buffer.WriteString(t.ID())
	}
	return buffer.String()
}

// Filter describes how manifest dependencies can be filtered.
type Filter struct {
	Related *Selector
}

// New creates a manifest from a json-encoded byte array or a yaml-front-matter
// having byte array. If frontmatter is found, the content below it is assigned
// to `.Spec.Body` (overwriting any content that may be there).
func New(manifest []byte) (*Manifest, error) {
	var delim = []byte("---")
	var err error
	body := append([]byte{}, manifest...)
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
	var m Manifest
	if err = json.Unmarshal(body, &m); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}
	m.Raw = manifest
	if m.Meta == nil {
		m.Meta = &Meta{}
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// NewFromFile creates a manifest from a source file.
func NewFromFile(filepath string) (*Manifest, error) {
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
	manifest, newErr := New(data)
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
	process := errgroup.Group{}
	for {
		raw, err := reader.ReadBytes('\n')
		if errors.Is(err, io.EOF) {
			break
		}
		if len(bytes.TrimSpace(raw)) == 0 {
			continue
		}
		process.Go(func() error {
			manifest, err := New(bytes.TrimRight(raw, "\n"))
			if err != nil {
				return fmt.Errorf("%s: %w", raw, err)
			}
			queue <- manifest
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
	if err := process.Wait(); err != nil {
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
	process, processCtx := errgroup.WithContext(context.Background())
	sem := semaphore.NewWeighted(10)
	process.Go(func() error {
		return filepath.Walk(dir, func(path string, f os.FileInfo, err error) error {
			if f.IsDir() {
				return nil
			}
			if err := sem.Acquire(processCtx, 1); err != nil {
				return err
			}
			process.Go(func() error {
				defer sem.Release(1)
				manifest, err := NewFromFile(path)
				if err != nil {
					return err
				}
				queue <- manifest
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
	if err := process.Wait(); err != nil {
		return nil, err
	}
	close(queue)
	collector.Wait()
	return manifests, nil
}

// Validate does just what you think it does.
func (m *Manifest) Validate() error {
	for _, i := range m.Meta.Include {
		for _, tmpl := range i.Templates {
			if tmpl.NameIsWildcard() {
				return fmt.Errorf("template selector cannot be wildcard")
			}
		}
		if i.As != "" && i.Resource != nil && i.Resource.NameIsWildcard() {
			return fmt.Errorf("wildcard selectors cannot be aliased")
		}
	}
	return nil
}

// String returns the ID of the resource and the entire manifest that it was
// instantiated with.
func (m *Manifest) String() string { return fmt.Sprintf("%s: %s", m.Selector.ID(), m.Raw) }

// UnmarshalJSON does just what you think it does.
func (m *Manifest) UnmarshalJSON(data []byte) error {
	var temp struct {
		Kind      string
		Group     string
		Version   string
		Name      string
		Namespace string
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
		Selector: &s,
		Meta:     temp.Meta,
		Spec:     temp.Spec,
		Raw:      data,
	}
	return nil
}

// Get list of required selectors for every related and included resource.
func (m *Manifest) Required() []*Selector {
	var selectors []*Selector
	// Collect all selectors found in m.Meta
	if len(m.Meta.Include) > 0 {
		for _, include := range m.Meta.Include {
			selectors = append(append(selectors, include.Templates...), include.Resource)
		}
	}
	if m.Meta.Related != nil {
		selectors = append(selectors, m.Meta.Related...)
	}
	return selectors
}

// EachInclude calls an iterator function for every valid included resource.
func (m *Manifest) EachInclude(index *Index, fn func(*Include) error) error {
	if m.Meta.Include == nil {
		return nil
	}
	for _, include := range m.Meta.Include {
		if include.Resource.NameIsWildcard() {
			// Error ignored because wildcard selector is valid with no matches.
			manifests, _ := index.Find(include.Resource)
			for _, manifest := range manifests {
				// If filtering, skip resources that don't match.
				if include.Filter != nil && include.Filter.Related != nil {
					keep := false
					for _, item := range manifest.Meta.Related {
						if keep = include.Filter.Related.Matches(*item); keep {
							break
						}
					}
					if !keep {
						continue
					}
				}
				if err := fn(&Include{
					Resource:  manifest.Selector,
					BaseHref:  include.BaseHref,
					Templates: include.Templates,
					As:        manifest.Selector.ID(),
				}); err != nil {
					return err
				}
			}
			continue
		}
		if err := fn(include); err != nil {
			return err
		}
	}
	return nil
}
