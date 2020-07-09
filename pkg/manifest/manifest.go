// Package manifest provides a basis for describing a resource of any type along
// with any resources it depends on.
package manifest

import (
	"bytes"
	"fmt"
	"github.com/ghodss/yaml"
	json "github.com/json-iterator/go"
	"github.com/tidwall/sjson"
	"io/ioutil"
	"path"
)

// Manifest defines the data that is used to instantiate a resource.
type Manifest struct {
	*Selector
	Meta *Meta
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
	Related *SelectorList
	// Includes allows a manifest to express a dependency on other resources.
	Include []*Include
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

// New creates a manifest from a json-encoded byte array or a yaml-front-matter
// having byte array. If frontmatter is found, the content below it is assigned
// to `.Spec.Body` (overwriting any content that may be there).
func New(manifest []byte) (*Manifest, error) {
	var delim = []byte("---")
	var err error
	var m Manifest
	// Process front-matter, if any.
	if bytes.HasPrefix(manifest, delim) {
		parts := bytes.SplitN(manifest, delim, 3)
		if manifest, err = yaml.YAMLToJSON(parts[1]); err != nil {
			return nil, err
		}
		if len(parts[2]) > 0 {
			if manifest, err = sjson.SetBytes(manifest, "spec.body", parts[2]); err != nil {
				return nil, err
			}
		}
	}
	if err = json.Unmarshal(manifest, &m); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}
	if err = m.Validate(); err != nil {
		return nil, fmt.Errorf("%s: validate failed: %w", m, err)
	}
	if m.Meta == nil {
		m.Meta = &Meta{}
	}
	return &m, nil
}

// Validate ensures a resource has the minimum required fields.
func (m *Manifest) Validate() error {
	if len(m.Spec) == 0 {
		return fmt.Errorf("spec must be defined")
	}
	return nil
}

// Get list of manifests for of every related and included resource.
func (m *Manifest) Related(index *Index) (List, error) {
	related := List{}
	if len(m.Meta.Include) > 0 {
		for _, include := range m.Meta.Include {
			for _, selector := range append(include.Layouts, include.Template, include.Resource) {
				if selector != nil {
					manifests, findErr := index.Find(SelectorList{selector})
					if findErr != nil {
						return nil, findErr
					}
					related = append(related, manifests...)
				}
			}
		}
	}
	if m.Meta.Related != nil {
		for _, selector := range *m.Meta.Related {
			manifests, findErr := index.Find(SelectorList{selector})
			if findErr != nil {
				return nil, findErr
			}
			related = append(related, manifests...)
		}
	}
	return related, nil
}

// String returns the ID of the resource and the entire manifest that it was
// instantiated with.
func (m *Manifest) String() string { return fmt.Sprintf("%s: %s", m.ID(), m.Raw) }

// KGV is a convenience function that returns the KGV of the resource selector.
func (m *Manifest) KGV() string { return m.Selector.KGV() }

// NKGV is a convenience function that returns the NKGV of the resource selector.
func (m *Manifest) NKGV() string { return m.Selector.NKGV() }

// ID is a convenience function that returns the ID of the resource selector.
func (m *Manifest) ID() string { return m.Selector.ID() }

func (m *Manifest) Traverse(index *Index) (List, error) {
	if index == nil {

	}
	return index.traverse(List{m}, nil)
}

// UnmarshalJSON handles the fact that embedding a struct (in this case
// selector.Selector) makes golang use the wrong unmarshalling implementation
// for the outer struct.
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
	s := &Selector{
		Kind:      temp.Kind,
		Group:     temp.Group,
		Version:   temp.Version,
		Name:      temp.Name,
		Namespace: temp.Namespace,
	}
	if err := s.Validate(); err != nil {
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
