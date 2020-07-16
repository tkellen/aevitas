// Package resource presents a common interface for rendering any resource type.
package resource

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/go-git/go-billy/v5"
	"github.com/tkellen/aevitas/pkg/manifest"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"html/template"
	"io"
	"io/ioutil"
	"path"
	"reflect"
	"strings"
)

// asRenderable represents a resource instance that can be rendered.
type asRenderable interface {
	Render(context.Context, *Resource) error
}

// asContent represents a resource instance that can be used for templating.
type asContent interface {
	Content() string
}

// asLinkable represents a resource instance that can be linked to.
type asLinkable interface {
	Href() string
}

// asTitled represents a resource instance that has a title fragment.
type asTitled interface {
	Title() string
}

// Resource represents a resource with all associated resources and templates
// in a form that can be rendered.
type Resource struct {
	Name string
	// Manifest describes the raw data used to instantiate the resource.
	Manifest *manifest.Manifest
	// Root refers to first resource instantiated (which triggers instantiation
	// of all relatedTo resources to support rendering).
	Root *Resource
	// Parent refers to the resource that caused this to be instantiated.
	Parent *Resource
	// Instance holds an interface to an instantiated resource.
	Instance interface{}
	// Source describes the location where backing data for the resource can be
	// found (e.g an image file).
	Source billy.Filesystem
	// Dest indicates where the resource in rendered form should be written.
	Dest billy.Filesystem
	// BaseHref provides a prefix that will be added to hrefs for the resource
	// instance this wraps.
	BaseHref string
	// Included defines sub-resources that must be rendered to to support the
	// rendering of this resource.
	included []*Resource
	// Templates defines resources that will be applied in order produce text
	// based output for the resource (typically html).
	templates []*Resource
	// Resources can express relations to other resources through the usage
	// of "meta.include" and "meta.related" fields on their manifest. During
	// instantiation, this is populated with all included/related resources
	// and all those resources that with a dependency/relationship back. This
	// makes relationships visible from "both sides" regardless of where they
	// were declared.
	relations *manifest.Relations
	factory   *Factory
	index     *manifest.Index
}

// New instantiates a resource and all of its dependencies.
func New(target string, index *manifest.Index, factory *Factory) (*Resource, error) {
	root, getErr := index.Get(target)
	if getErr != nil {
		return nil, getErr
	}
	// Increase speed by filtering items to those used by root element only.
	deps, traverseErr := index.Traverse(root)
	if traverseErr != nil {
		return nil, traverseErr
	}
	subIndex, indexErr := manifest.NewIndex(deps, true)
	if indexErr != nil {
		return nil, indexErr
	}
	return newResource(
		root.Selector.ID(),
		root.Selector,
		subIndex,
		factory,
		"",
		nil,
	)
}

// New instantiates a new resource from an existing one.
func (r *Resource) New(name string, target *manifest.Selector, baseHref string) (*Resource, error) {
	return newResource(name, target, r.index, r.factory, baseHref, r.Root)
}

// String returns a human readable representation of a resource.
func (r *Resource) String() string {
	if r.Name != r.Manifest.Selector.ID() {
		return fmt.Sprintf("%s (%s)", r.Manifest.Selector.ID(), r.Name)
	}
	return r.Name
}

// Reader returns a io.ReadCloser for the data the resource points to.
func (r *Resource) Reader(_ context.Context) (io.ReadCloser, error) {
	return r.Source.Open(fmt.Sprintf("%s", r.Manifest.Meta.File))
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

// Render recursively renders all resources associated with this resource.
func (r *Resource) Render(ctx context.Context, concurrency int) error {
	sem := semaphore.NewWeighted(int64(concurrency))
	process, processCtx := errgroup.WithContext(ctx)
	process.Go(func() error {
		for _, include := range r.included {
			if err := sem.Acquire(processCtx, 1); err != nil {
				return err
			}
			include := include
			process.Go(func() error {
				defer sem.Release(1)
				return include.Render(processCtx, concurrency)
			})
		}
		return nil
	})
	if err := process.Wait(); err != nil {
		return fmt.Errorf("%s: render: %w", r, err)
	}
	if resource, ok := r.Instance.(asRenderable); ok {
		return resource.Render(ctx, r)
	}
	return nil
}

// Body computes the content for a resource using all associated templates.
func (r *Resource) Body() (string, error) {
	root, buildErr := r.template(nil)
	if buildErr != nil {
		return "", buildErr
	}
	return executeTemplate(root, r)
}

func (r *Resource) Spec() interface{} {
	return reflect.ValueOf(r.Instance).Elem().FieldByName("Spec").Interface()
}

func (r *Resource) Title() (string, error) {
	var titles []string
	parent := r
	for parent != nil {
		titled, ok := parent.Instance.(asTitled)
		if ok && titled.Title() != "" {
			titles = append(titles, titled.Title())
		}
		parent = parent.Parent
	}
	if len(titles) == 0 {
		return "", fmt.Errorf("%s has no title", r)
	}
	return strings.Join(titles, " | "), nil
}

func (r *Resource) HrefCanonical(lookup ...string) (string, error) {
	target := r
	if len(lookup) > 1 {
		return "", fmt.Errorf("only one target allowed")
	}
	if len(lookup) == 1 {
		var err error
		if target, err = r.Get(lookup[0]); err != nil {
			return "", err
		}
	}
	if linkable, ok := target.Instance.(asLinkable); ok {
		return linkable.Href(), nil
	} else {
		return "", fmt.Errorf("%s does not define a href", r)
	}
}

func (r *Resource) Href(lookup ...string) (string, error) {
	target := r
	if len(lookup) > 1 {
		return "", fmt.Errorf("only one target allowed")
	}
	if len(lookup) == 1 {
		var err error
		if target, err = r.Get(lookup[0]); err != nil {
			return "", err
		}
	}
	if linkable, ok := target.Instance.(asLinkable); ok {
		return path.Join(r.BaseHref, linkable.Href()), nil
	} else {
		return "", fmt.Errorf("%s does not define a href", r)
	}
}

func (r *Resource) Related(target string) ([]*Resource, error) {
	s, err := manifest.NewSelector(target)
	if err != nil {
		return nil, err
	}
	related, ok := r.index.Relations[r.Manifest]
	if !ok {
		return nil, fmt.Errorf("no relations found for %s %s\n", r.Manifest.Selector, r.index.Relations)
	}
	relatedIndex, indexErr := related.Indexed(false)
	if indexErr != nil {
		return nil, indexErr
	}
	manifests, findErr := relatedIndex.Find(&s)
	var output []*Resource
	if findErr != nil {
		// If the selector has wildcards we can't know for sure if any records
		// should exist or not.
		if s.NameIsWildcard() {
			return output, nil
		}
		return nil, findErr
	}
	for _, item := range manifests {
		resource, err := r.New(item.Selector.ID(), item.Selector, "")
		if err != nil {
			return nil, err
		}
		output = append(output, resource)
	}
	return output, err
}

func (r *Resource) Get(target string) (*Resource, error) {
	related, err := r.Related(target)
	if err != nil {
		return nil, fmt.Errorf("relation: %w", err)
	}
	if len(related) > 1 {
		return nil, fmt.Errorf("%s matched %d resources", target, len(related))
	}
	return related[0], nil
}

// newResource recursively instantiates a resources and all of its dependencies.
func newResource(
	name string,
	target *manifest.Selector,
	index *manifest.Index,
	factory *Factory,
	baseHref string,
	root *Resource,
) (*Resource, error) {
	m, getErr := index.GetSelector(target)
	if getErr != nil {
		return nil, getErr
	}
	handler, handlerErr := factory.Handler(m)
	if handlerErr != nil {
		return nil, handlerErr
	}
	instance, err := handler.New(m)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", m, err)
	}
	dest, destErr := handler.Dest.Chroot(baseHref)
	if destErr != nil {
		return nil, destErr
	}
	el := &Resource{
		Name:     name,
		Manifest: m,
		BaseHref: baseHref,
		Root:     root,
		Instance: instance,
		Source:   handler.Source,
		Dest:     dest,
		index:    index,
		factory:  factory,
	}
	if root == nil {
		root = el
		el.Root = el
	}
	// Recursively instantiate included elements.
	if err := m.EachInclude(index, func(include *manifest.Include) error {
		dep, err := el.New(include.As, include.Resource, include.BaseHref)
		if err != nil {
			return err
		}
		dep.Parent = el
		// If there are templates associated with this element, create
		// resources for them so they can be used during rendering.
		for _, templateSelector := range include.Templates {
			tmpl, err := el.New(templateSelector.ID(), templateSelector, "")
			if err != nil {
				return err
			}
			tmpl.Parent = el
			dep.templates = append(dep.templates, tmpl)
		}
		el.included = append(el.included, dep)
		return nil
	}); err != nil {
		return nil, err
	}
	return el, nil
}

// template recursively collects all templates needed to render content for the
// resource.
func (r *Resource) template(root *template.Template) (*template.Template, error) {
	if root == nil {
		root = template.New(r.Name).Funcs(template.FuncMap{
			"yield": func() (error, error) {
				return nil, errors.New("no yield content available yet")
			},
		})
	} else if root.Lookup(r.Name) != nil {
		// If the same template is used more than once during the rendering of
		// a given resource, don't compute it twice.
		return root.Lookup(r.Name), nil
	}
	var err error
	// Compute templates for all dependencies so their rendered output can be
	// used to render this resource.
	for _, dep := range r.included {
		if root, err = dep.template(root); err != nil {
			return nil, err
		}
	}
	// If the resource embeds content, assign that content as the "yield" body
	// so templates can wrap it.
	var yield string
	if resource, ok := r.Instance.(asContent); ok {
		yield = resource.Content()
	}
	// If this resource is viewed through a template, render those templates
	// with this resource as context.
	for _, tmpl := range r.templates {
		if root, err = tmpl.template(root); err != nil {
			return nil, err
		}
		if yield, err = executeTemplate(root.Funcs(template.FuncMap{
			"yield": func() template.HTML {
				return template.HTML(yield)
			},
		}), r); err != nil {
			return nil, err
		}
	}
	// Save the output.
	return root.New(r.Name).Parse(yield)
}

func executeTemplate(root *template.Template, context *Resource) (string, error) {
	var buf bytes.Buffer
	temp, err := root.Clone()
	if err != nil {
		return "", err
	}
	if err := temp.Execute(&buf, context); err != nil {
		return "", err
	}
	return buf.String(), nil
}
