package resource

import (
	"encoding/hex"
	"fmt"
	"github.com/lestrrat-go/strftime"
	hash "github.com/minio/sha256-simd"
	"github.com/tkellen/aevitas/internal/selector"
	"github.com/tkellen/aevitas/pkg/manifest"
	"html/template"
	"path"
	"reflect"
)

// Resource represents a manifest with all related manifests associated in a
// manner that allows rendering.
type Resource struct {
	*manifest.Manifest
	scope      *manifest.Manifest
	titles     []string
	hrefRoot   string
	href       string
	template   *Template
	children   []*Resource
	index      *manifest.Index
	factory    *Factory
	instance   *Instance
	cacheID    string
	associated map[string]interface{}
}

func New(index *manifest.Index, target string, factory *Factory) (*Resource, error) {
	selector, selectorErr := selector.New(target)
	if selectorErr != nil {
		return nil, selectorErr
	}
	root, getErr := index.FindOne(selector)
	if getErr != nil {
		return nil, getErr
	}
	return (&Resource{
		Manifest: nil,
		index:    index,
		factory:  factory,
	}).new(root, nil, "", "")
}

func (r *Resource) newStub(self *manifest.Manifest, scope *manifest.Manifest) (*Resource, error) {
	instance, err := newInstance(r.factory, self)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", self, err)
	}
	return &Resource{
		Manifest:   self,
		scope:      scope,
		children:   []*Resource{},
		titles:     r.titles,
		hrefRoot:   r.hrefRoot,
		index:      r.index,
		factory:    r.factory,
		instance:   instance,
		associated: map[string]interface{}{},
	}, nil
}

func (r *Resource) new(
	self *manifest.Manifest,
	scope *manifest.Manifest,
	titlePrefix string,
	hrefPrefix string,
) (*Resource, error) {
	parent, err := r.newStub(self, scope)
	if err != nil {
		return nil, err
	}
	if titlePrefix != "" {
		parent.titles = append([]string{titlePrefix}, r.titles...)
	}
	if hrefPrefix != "" {
		parent.hrefRoot = path.Join(r.hrefRoot, hrefPrefix)
	}
	// Instantiate a template to give this resource the ability to be rendered.
	template, templateErr := NewTemplate(parent)
	if templateErr != nil {
		return nil, templateErr
	}
	parent.template = template
	// A cacheID must be computed before children are populated so they can
	// inherit it.
	digest := hash.Sum256([]byte(r.ID() + template.id))
	parent.cacheID = hex.EncodeToString(digest[:])
	// Recursively collect all children of this resource.
	for _, item := range self.Meta.Children {
		var childGroup []*Resource
		resolvedChildren, relationErr := item.Relation.Resolve(r.index)
		if relationErr != nil {
			return nil, relationErr
		}
		for _, match := range resolvedChildren {
			var scope *manifest.Manifest
			// If there is a prefix associated with this child, the parent is
			// contributing to the "scope" of the manifest.
			if item.HrefPrefix != "" {
				scope = parent.Manifest
			}
			child, err := parent.new(match, scope, item.TitlePrefix, item.HrefPrefix)
			if err != nil {
				return nil, err
			}
			parent.children = append(parent.children, child)
			childGroup = append(childGroup, child)
		}
		// Make named children accessible to templates (the template instance
		// contributes imports and dynamic imports from other templates at
		// body time.
		if item.Name != "" {
			parent.associated[item.Name] = func() []*Resource {
				return childGroup
			}
		}
	}
	return parent, nil
}

// ID returns a unique identifier for the resource that computes a hash of
// every dependent resource used during rendering. This allows fine-grained
// cache busting on repeated renders.
func (r *Resource) ID() string { return r.cacheID }

// Instance returns the underlying instance of the specific type of resource
// this represents. This should ideally make use of go generics. Will it ever
// happen? Only time can tell.
func (r *Resource) Instance() *Instance { return r.instance }

// Associated gives templates that consume this resource access to imports and
// named children.
func (r *Resource) Associated() map[string]interface{} { return r.associated }

// Titles gives templates that consume this resource access to the full stack
// of title segments that contributed to the title of the page. The templating
// system allows users to determine how the titles should be combined.
func (r *Resource) Titles() []string { return append([]string{r.Title()}, r.titles...) }

// Scope gives templates that consume this resource access to the manifest (if
// any) that has been used to "scope" this resource. E.g. a topic can scope a
// resource such that navigation is limited to other resource that share the
// topic.
func (r *Resource) Scope() *manifest.Manifest { return r.scope }

// Href computes a reference to the resource that includes any prefixes which
// were added by parents that scoped it. For example, a post may specify
// `/yyyy/mm/post-slug` as a href. When the post is rendered as a child of a
// topic, for example, the topic contributes `/topic/name/` as a prefix.
func (r *Resource) Href() string { return path.Join(r.hrefRoot, r.Manifest.Href()) }

// HrefCanonical returns an un-scoped reference to the underlying resource.
func (r *Resource) HrefCanonical() string { return r.Manifest.Href() }

// Spec gives templates access to fields on a resource that are custom to a
// specific type.
func (r *Resource) Spec() (interface{}, error) {
	return reflect.ValueOf(r.instance.Self).Elem().FieldByName("Spec").Interface(), nil
}

// Prev returns the previous entry (by publish date, then by selector name) for
// this resource based on the underlying manifest kind/group/version/namespace.
// If a scoping manifest is present it will limit navigation to manifests that
// are related to the scope.
func (r *Resource) Prev() (*Resource, error) { return r.navigate("prev") }

// Prev returns the previous entry (by publish date, then by selector name) for
// this resource based on the underlying manifest kind/group/version/namespace.
// If a scoping manifest is present it will limit navigation to manifests that
// are related to the scope.
func (r *Resource) Next() (*Resource, error) { return r.navigate("next") }

// YearsPastAndFuture returns all resources that were published in different
// years on the same month and day as this resource. This is limited to
// resources that match the kind/group/version/namespace of the resource.
func (r *Resource) YearsPastAndFuture() ([]*Resource, error) {
	var matches []*Resource
	if r.Meta.PublishAt == nil {
		return matches, nil
	}
	for _, match := range r.index.SameMonthDay(r.Manifest) {
		if match.Meta.PublishAt == nil || match.Meta.PublishAt.Year == 0 || match.Meta.PublishAt.Month == 0 {
			continue
		}
		if match.Meta.PublishAt.Year != r.Meta.PublishAt.Year {
			resource, err := r.newStub(match, nil)
			if err != nil {
				return nil, err
			}
			matches = append(matches, resource)
		}
	}
	return matches, nil
}

// Flatten generates a flat array of resources by recursively collecting all
// children from this resource down.
func (r *Resource) Flatten() []*Resource {
	accum := []*Resource{r}
	for _, child := range r.children {
		accum = append(accum, child.Flatten()...)
	}
	return accum
}

// Render produces textual output for this resource.
func (r *Resource) Render() (template.HTML, error) {
	result, err := r.template.render(nil, "")
	if err != nil {
		return "", err
	}
	return result, nil
}

// Date is a shitty abstraction that allows the publish date to be formatted for
// human readability.
func (r *Resource) Date(format string) (string, error) {
	parse, timeErr := strftime.New(format)
	if timeErr != nil {
		return "", timeErr
	}
	return parse.FormatString(r.PublishAt()), nil
}

// navigate is a generic method used to locate next/previous/same date resources.
func (r *Resource) navigate(dir string) (*Resource, error) {
	var match *manifest.Manifest
	index := r.index
	if r.scope != nil {
		var err error
		if index, err = index.RelatedIndex(r.scope); err != nil {
			return nil, err
		}
	}
	if dir == "next" {
		match = index.Next(r.Manifest)
	}
	if dir == "prev" {
		match = index.Prev(r.Manifest)
	}
	if match == nil {
		return nil, nil
	}
	// Return a stub because at least so far, we do not need the full render
	// of the resource to satisfy usages of next/previous.
	return r.newStub(match, r.scope)
}
