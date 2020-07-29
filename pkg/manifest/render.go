package manifest

import (
	"bytes"
	"fmt"
	"github.com/tkellen/aevitas/internal/selector"
)

type Render struct {
	// Templates allows a manifest to declare dependencies on other
	// manifests that are used to render it.
	Templates
	// Children allows a manifest to express other resources that that
	// should be rendered separately as a child of this manifest.
	Children []*Child
	// Imports allows a manifest to express static dependencies that will be
	// computed before rendering.
	Imports []*Relation
	// ImportsDynamic allows a manifest to express dynamic dependencies
	// based on how the manifest is consumed.
	ImportsDynamic []*DynamicRelation
}

func (r *Render) validate() error {
	if r.Templates != nil {
		if err := r.Templates.validate(); err != nil {
			return err
		}
	}
	for _, child := range r.Children {
		if err := child.validate(); err != nil {
			return err
		}
	}
	for _, toImport := range r.Imports {
		if err := toImport.validate(); err != nil {
			return err
		}
	}
	for _, toImport := range r.ImportsDynamic {
		if err := toImport.validate(); err != nil {
			return err
		}
	}
	return nil
}

// Templates describes an array of selectors to (template-containing)
// manifests that should be used during rendering.
type Templates []*selector.Selector

// validate does just what you think it does.
func (rw Templates) validate() error {
	for _, selector := range rw {
		if selector.IsWildcard() {
			return fmt.Errorf("wildcard selector not allowed")
		}
	}
	return nil
}

// Resolve converts selectors into manifests.
func (rw Templates) Resolve(index *Index) ([]*Manifest, error) {
	manifests := make([]*Manifest, len(rw))
	for idx, selector := range rw {
		manifest, err := index.FindOne(selector)
		if err != nil {
			return nil, err
		}
		manifests[idx] = manifest
	}
	return manifests, nil
}

// ID returns a unique identifier for a given set of templates. This is used to
// enable caching for templates.
func (rw Templates) ID() string {
	var buffer bytes.Buffer
	for _, t := range rw {
		buffer.WriteString(t.ID())
	}
	return buffer.String()
}

// Child allows a manifest to express child manifests that should be rendered
// alongside the parent.
type Child struct {
	Relation
	Templates
}

// validate does just what you think it does.
func (c *Child) validate() error {
	if err := c.Relation.validate(); err != nil {
		return err
	}
	if err := c.Templates.validate(); err != nil {
		return err
	}
	return nil
}
