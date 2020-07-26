package manifest

import (
	"bytes"
	"fmt"
	"time"
)

// Meta provides details about a resource.
type Meta struct {
	// File is a temporary hack to make this system work with memorybox.
	File string
	// Live controls if a manifest is collected during production builds.
	Live bool
	// PublishAt controls if a manifest is collected during production builds.
	// If present, current date/time must be greater than the machine that runs
	// the build. It also provides the basis for ordering manifests.
	PublishAt PublishAt
	// An optional prefix for the resource this points to.
	BaseHref string
	// RenderTemplates allows a manifest to express a dependency on other resources
	// that are used to render it.
	RenderTemplates
	// Relations allows expressing relationships with other manifests.
	Relations []*Relation
	// RenderAsChild allows a manifest to express other resources that that
	// should be rendered separately as a child of this manifest.
	RenderAsChild []*Child
	// Embed allows a manifest to express resources that will be rendered and
	// included in the output of rendering this one.
	Embed map[string]*Child
	// GenerateManifests defines a template that will add generated manifests to
	// the index when this manifest is inserted.
	GenerateManifests []*Generator
}

// Validate does just what you think it does.
func (m *Meta) Validate() error {
	for _, related := range m.Relations {
		if err := related.Validate(); err != nil {
			return err
		}
	}
	if m.RenderTemplates != nil {
		if err := m.RenderTemplates.Validate(); err != nil {
			return err
		}
	}
	for _, child := range m.RenderAsChild {
		if err := child.Validate(); err != nil {
			return err
		}
	}
	for _, generator := range m.GenerateManifests {
		if err := generator.Validate(); err != nil {
			return fmt.Errorf("%s: %w", generator.Name, err)
		}
	}
	for _, embed := range m.Embed {
		if err := embed.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// PublishAt describes in a granular fashion when a given manifest should be
// published. This deconstructed form is meant to ease filtering and relating to
// manifests by portions of their publication date from within manifest files.
type PublishAt struct {
	Year    int
	Month   time.Month
	Day     int
	Hours   int
	Minutes int
	Seconds int
}

// Date returns a native time object from the deconstructed form.
func (pa *PublishAt) Date() time.Time {
	// TODO: benchmark memoizing
	return time.Date(pa.Year, pa.Month, pa.Day, pa.Hours, pa.Minutes, pa.Seconds, 0, time.UTC)
}

// RenderTemplates describes an array of selectors to (template-containing) manifests
// that should be used during rendering.
type RenderTemplates []*Selector

// Validate does just what you think it does.
func (rw *RenderTemplates) Validate() error {
	for _, selector := range *rw {
		if selector.IsWildcard() {
			return fmt.Errorf("wildcard selector not allowed")
		}
	}
	return nil
}

// NavigationScopedByParent indicates if a manifest should be "scoped" by a parent
// manifest during rendering. For example, a post that is viewed from within the
// context of a topic should show next or previous links for posts within that
// topic. This makes that possible.
type NavigationScopedByParent bool

// RenderTarget describes a manifest and its associated rendering requirements.
type RenderTarget struct {
	Name     string
	Selector *Selector
	RenderTemplates
	NavigationScopedByParent
}

// Child allows a manifest to express child manifests that should be rendered
// alongside the parent.
type Child struct {
	Relation
	RenderTemplates
	NavigationScopedByParent
}

// Validate does just what you think it does.
func (c *Child) Validate() error {
	if err := c.Relation.Validate(); err != nil {
		return err
	}
	if err := c.RenderTemplates.Validate(); err != nil {
		return err
	}
	return nil
}

// ID returns a unique identifier for a given Child. This is used to enable
// find-grained caching in the resource layer.
func (c *Child) ID() string {
	var buffer bytes.Buffer
	buffer.WriteString(c.Selector.ID())
	for _, t := range c.RenderTemplates {
		buffer.WriteString(t.ID())
	}
	return buffer.String()
}

// Expand resolves the relationships within a child creating one RenderTarget
// for each matching manifest.
func (c *Child) Expand(index *Index, name string) ([]*RenderTarget, error) {
	selectors, resolveErr := c.Relation.Resolve(index)
	if resolveErr != nil {
		return nil, resolveErr
	}
	expanded := make([]*RenderTarget, len(selectors))
	for idx, selector := range selectors {
		expanded[idx] = &RenderTarget{
			Name:                     name,
			Selector:                 selector,
			RenderTemplates:          c.RenderTemplates,
			NavigationScopedByParent: c.NavigationScopedByParent,
		}
	}
	return expanded, nil
}
