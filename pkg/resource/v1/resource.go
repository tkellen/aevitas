// Package resource defines a minimal structure that manifests must satisfy to
// be considered a resource. It also provides facilities for indexing and
// traversing a dependency tree of resources.
package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
)

// Resource defines a manifest that can be used to describe any piece of data.
// Each resource in a given implementation using this system must have a unique
// (composite) ID composed of its kind, group, version and name.
type Resource struct {
	Selector
	Meta Meta
	// Spec contains details defined by the resource's KGV (Kind Group Version).
	Spec json.RawMessage
	// Imports allows a resource to express a dependency on other resources.
	Imports []Import
	// Manifest holds a copy of the raw data provided when instantiating a
	// resource.
	Manifest []byte
}

// Meta provides details about the resource such as where the backing data for
// it can be found.
type Meta struct {
	File string
}

// New creates a new resource from a json-encoded byte array.
func New(manifest []byte) (*Resource, error) {
	var r Resource
	if err := json.Unmarshal(manifest, &r); err != nil {
		return nil, fmt.Errorf("%w: %s", err, manifest)
	}
	if err := r.Validate(); err != nil {
		return nil, fmt.Errorf("%s\n%w", manifest, err)
	}
	r.Manifest = manifest
	return &r, nil
}

// Deps return a list of resource selectors a resource depends on. It
// is an array of all imports and the templates specified to render them.
func (r *Resource) Deps() SelectorList {
	var result SelectorList
	if r.Imports == nil {
		return result
	}
	for _, item := range r.Imports {
		if item.Resource != (Selector{}) {
			result = append(result, item.Resource)
		}
		if item.Template != (Selector{}) {
			result = append(result, item.Template)
		}
	}
	return result
}

// Validate ensures a resource has the minimum required fields.
func (r *Resource) Validate() error {
	if r.Kind == "" || r.Group == "" || r.Version == "" {
		return fmt.Errorf("group, version and kind must be set")
	}
	if len(r.Spec) == 0 {
		return fmt.Errorf("spec must be defined")
	}
	return nil
}

// String returns the ID of the resource and the entire manifest that it was
// instantiated with.
func (r *Resource) String() string { return fmt.Sprintf("%s: %s", r.ID(), r.Manifest) }

// KGV is a convenience function that returns the KGV of the resource selector.
func (r *Resource) KGV() string { return r.Selector.KGV() }

// ID is a convenience function that returns the ID of the resource selector.
func (r *Resource) ID() string { return r.Selector.ID() }

// Reader returns a io.ReadCloser for the data the resource points to.
func (r *Resource) Reader(_ context.Context) (io.ReadCloser, error) {
	return os.Open(fmt.Sprintf("/home/tkellen/memorybox/%s", r.Meta.File))
}

// Bytes returns a byte array for the data the resource points to.
func (r *Resource) Bytes(ctx context.Context) ([]byte, error) {
	reader, fetchErr := r.Reader(ctx)
	if fetchErr != nil {
		return nil, fetchErr
	}
	defer reader.Close()
	data, readErr := ioutil.ReadAll(reader)
	if readErr != nil {
		return nil, readErr
	}
	return data, nil
}

// ExpandImports produces a static list of imports by replacing all wildcard
// imports with literal imports by matching it against the supplied index.
func (r *Resource) ExpandImports(index IndexedList) ([]Import, error) {
	var imports []Import
	for _, imported := range r.Imports {
		expandedImports, err := imported.Expand(index)
		if err != nil {
			return nil, err
		}
		imports = append(imports, expandedImports...)
	}
	return imports, nil
}