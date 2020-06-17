// Package resource defines a manifest that can be used to describe any piece
// of data.
package resource

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-git/go-billy/v5"
	"github.com/tkellen/aevitas/pkg/selector"
	"html/template"
	"io"
	"io/ioutil"
	"sort"
	"strings"
)

// Element describes the public API of the element package to avoid circular
// references.
type Element interface {
	Name() string
	Instance() Renderable
	Resource() *Resource
	Imports() []Element
	Body() (template.HTML, error)
	Source() billy.Filesystem
	Dest() billy.Filesystem
}

// Renderable describes a minimal public API that is common to all resource types.
type Renderable interface {
	Render(context.Context, Element) error
}

// resource defines any piece of data.
type Resource struct {
	selector.Selector
	Meta Meta
	// Spec contains details defined by the resource's NKGV (Kind Group Version).
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

// NewSelector creates a new resource from a json-encoded byte array.
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

// Imported returns all imports for the resource.
func (r *Resource) Imported() []Import {
	return r.Imports
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

// Traverse return a selector.List of resource selectors the resource depends on. It
// combines all imports and the templates specified to render them.
func (r *Resource) Deps() selector.List {
	var result selector.List
	if r.Imports == nil {
		return result
	}
	for _, item := range r.Imports {
		if item.Resource != (selector.Selector{}) {
			result = append(result, item.Resource)
		}
		if item.Template != (selector.Selector{}) {
			result = append(result, item.Template)
		}
	}
	return result
}

// String returns the ID of the resource and the entire manifest that it was
// instantiated with.
func (r *Resource) String() string { return fmt.Sprintf("%s: %s", r.ID(), r.Manifest) }

// KGV is a convenience function that returns the KGV of the resource selector.
func (r *Resource) KGV() string { return r.Selector.KGV() }

// NKGV is a convenience function that returns the NKGV of the resource selector.
func (r *Resource) NKGV() string { return r.Selector.NKGV() }

// ID is a convenience function that returns the ID of the resource selector.
func (r *Resource) ID() string { return r.Selector.ID() }

// Reader returns a io.ReadCloser for the data the resource points to.
func (r *Resource) Reader(_ context.Context, fs billy.Filesystem) (io.ReadCloser, error) {
	return fs.Open(fmt.Sprintf("%s", r.Meta.File))
}

// Bytes returns a byte array for the data the resource points to.
func (r *Resource) Bytes(ctx context.Context, fs billy.Filesystem) ([]byte, error) {
	reader, fetchErr := r.Reader(ctx, fs)
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

// Import describes how a resource can depend on another.
type Import struct {
	// resource points to a resource that is required for rendering the
	// resource this import belongs to.
	Resource selector.Selector
	// template optionally points to a resource that should be used to
	// render the resource being imported.
	Template selector.Selector
	// As provides an alternative name for referring to the imported resource.
	As string
}

// NewImport produces an import entry and validates it.
func NewImport(resource string, template string, as string) (*Import, error) {
	instance := &Import{As: as}
	if resource != "" {
		selector, err := selector.New(resource)
		if err != nil {
			return nil, err
		}
		instance.Resource = selector
	}
	if template != "" {
		selector, err := selector.New(template)
		if err != nil {
			return nil, err
		}
		instance.Template = selector
	}
	if err := instance.Validate(); err != nil {
		return nil, err
	}
	return instance, nil
}

// Validate does just what you think it does.
func (i *Import) Validate() error {
	if i.Template != (selector.Selector{}) && i.Template.IsWildcard() {
		return fmt.Errorf("template cannot be wildcard selector")
	}
	if i.As != "" && i.Resource != (selector.Selector{}) && i.Resource.IsWildcard() {
		return fmt.Errorf("wildcard selectors cannot be aliased")
	}
	return nil
}

// Expand turns an import with a wildcard selector into a static array of fully
// specified import selectors by matching against a supplied index.
func (i Import) Expand(index IndexedList) []Import {
	if i.Resource.IsWildcard() {
		shard, ok := index[i.Resource.NKGV()]
		if ok {
			imports := make([]Import, len(shard.Resources))
			for idx, resource := range shard.Resources {
				imports[idx] = Import{
					Template: i.Template,
					Resource: resource.Selector,
					As:       resource.ID(),
				}
			}
			return imports
		}
		return []Import{}
	}
	return []Import{i}
}

// UnmarshalJSON converts string selectors in an import payload to instantiated
// Selectors.
func (i *Import) UnmarshalJSON(data []byte) error {
	var entry struct {
		Resource string
		Template string
		As       string
	}
	if err := json.Unmarshal(data, &entry); err != nil {
		return err
	}
	imported, err := NewImport(entry.Resource, entry.Template, entry.As)
	if err != nil {
		return err
	}
	*i = *imported
	return nil
}


// List holds an array of resources.
type List []*Resource

// NewListFromReader creates a List from a provided reader taking the assumption
// that the reader contains one resource per line.
func NewListFromReader(input io.Reader) (List, error) {
	resources := List{}
	reader := bufio.NewReader(input)
	for {
		manifest, err := reader.ReadBytes('\n')
		if errors.Is(err, io.EOF) {
			break
		}
		resource, err := New(bytes.TrimRight(manifest, "\n"))
		if err != nil {
			return nil, err
		}
		resources = append(resources, resource)
	}
	return resources, nil
}

// String produces a human readable representation of what a List contains.
func (l List) String() string {
	return l.Indexed().String()
}

// IndexedList converts a List into an IndexedList.
func (l List) Indexed() IndexedList {
	var shard *IndexShard
	index := IndexedList{}
	for _, rsrc := range l {
		nkgv := rsrc.NKGV()
		if _, ok := index[nkgv]; !ok {
			index[nkgv] = &IndexShard{
				Resources: List{},
				ById:      map[string]*Resource{},
			}
		}
		shard = index[nkgv]
		shard.Resources = append(shard.Resources, rsrc)
		shard.ById[rsrc.ID()] = rsrc
	}
	return index
}

// IndexedList is a map of resources indexed by NKGV and ID for fast lookups.
type IndexedList map[string]*IndexShard
type IndexShard struct {
	Resources []*Resource
	ById      map[string]*Resource
}

// String returns the count for each unique key/group/version in an IndexedList.
func (il IndexedList) String() string {
	format := "%-45s%v"
	totals := []string{fmt.Sprintf(format, "INDEX SHARD", "COUNT")}
	var shards []string
	for shard := range il {
		shards = append(shards, shard)
	}
	sort.Strings(shards)
	for _, shard := range shards {
		totals = append(totals, fmt.Sprintf(format, shard, len(il[shard].Resources)))
	}
	return strings.Join(totals, "\n")
}

// Get finds a single resource using a string target.
func (il IndexedList) Get(target string) (*Resource, error) {
	s, selectorErr := selector.New(target)
	if selectorErr != nil {
		return nil, selectorErr
	}
	return il.GetSelector(s)
}

// GetSelector finds a single resource using a selector.
func (il IndexedList) GetSelector(target selector.Selector) (*Resource, error) {
	nkgv := target.NKGV()
	id := target.ID()
	shard, ok := il[nkgv]
	if !ok {
		return nil, fmt.Errorf("no resources in shard %s", target)
	}
	resource, found := shard.ById[id]
	if !found {
		return nil, fmt.Errorf("%s not found\n%s", id, il)
	}
	return resource, nil
}

// Filter recursively produces a full list of resource imports for a supplied
// array of parents.
func (il IndexedList) Traverse(parents List, visited map[string]struct{}) (List, error) {
	var result List
	if visited == nil {
		visited = map[string]struct{}{}
	}
	for _, item := range parents {
		id := item.ID()
		// Increase speed (and prevent infinite recursion on cyclic deps) by
		// remembering each resource that has been visited and skipping if it
		// is seen more than once.
		if _, ok := visited[id]; ok {
			continue
		} else {
			visited[id] = struct{}{}
		}
		// Save parent resource in the results.
		result = append(result, item)
		// Find resource for each dependency.
		children, selectErr := il.Find(item.Deps())
		if selectErr != nil {
			return nil, fmt.Errorf("%s: dependency %w", id, selectErr)
		}
		// Recurse through all child dependencies.
		deps, err := il.Traverse(children, visited)
		if err != nil {
			return nil, err
		}
		result = append(result, deps...)
	}
	return result, nil
}

// Find produces a List that contains resources whose IDs match the provided
// selectors.
func (il IndexedList) Find(selectors selector.List) (List, error) {
	matches := List{}
	// Save references to KGVs that have been entirely collected so they aren't
	// collected more than once.
	collectEntireKGV := map[string]struct{}{}
	// Sorting ensures that wildcard NKGV selectors appear first.
	sort.Sort(selectors)
	for _, s := range selectors {
		entireKgv := s.IsWildcard()
		nkgv := s.NKGV()
		if shard, ok := il[nkgv]; ok {
			// If a selector targets an entire NKGV, append all of its resources
			// without iterating them individually.
			if entireKgv {
				if _, ok := collectEntireKGV[nkgv]; !ok {
					matches = append(matches, shard.Resources...)
					collectEntireKGV[nkgv] = struct{}{}
				}
				continue
			}
			// If selector does not target entire NKGV, but this NKGV has been
			// entirely selected already, skip this.
			if _, ok := collectEntireKGV[nkgv]; ok {
				continue
			}
			// Otherwise, look for a match by ID in the NKGV shard.
			if match, ok := shard.ById[s.ID()]; ok {
				matches = append(matches, match)
				continue
			}
		}
		return nil, fmt.Errorf("not present in index: %s\n%s", s, il)
	}
	return matches, nil
}