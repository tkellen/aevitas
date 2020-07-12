// Package resource presents a common interface for rendering any resource type.
package resource

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/go-git/go-billy/v5"
	"github.com/tkellen/aevitas/pkg/manifest"
	"html/template"
	"io"
	"io/ioutil"
)

// Instance represents a common interface for all resource types.
type Instance interface {
	ID() string
}

// Rendering represents a resource instance that can be rendered.
type Rendering interface {
	Render(context.Context, *Resource) error
}

// Templating represents a resource instance that can be used for templating.
type Templating interface {
	Content() string
}

// Resource represents a resource with all associated resources and templates
// in a form that can be rendered.
type Resource struct {
	Name string
	// Manifest describes the raw data used to instantiate the resource.
	Manifest *manifest.Manifest
	// Root refers to first resource instantiated (which triggers instantiation
	// of all related resources to support rendering).
	Root *Resource
	// Parent refers to the resource that caused this to be instantiated.
	Parent *Resource
	// Resources can express relationships to other resources through the usage
	// of "meta.include" and "meta.related" fields on their manifest. During
	// instantiation, this is populated with all included/related on manifests
	// and all those resources that with a dependency/relationship back. This
	// makes relationships visible from "both sides" regardless of where they
	// were declared.
	Relations *manifest.Relations
	// Instance holds an interface to an instantiated resource.
	Instance Instance
	// Source describes the location where backing data for the resource can be
	// found (e.g an image file).
	Source billy.Filesystem
	// Dest indicates where the resource in rendered form should be written.
	Dest billy.Filesystem
	// During rendering of the resource all resources listed here will be
	// rendered first.
	included []*Resource
	// related expresses a relationships between resources that does not require
	// rendering.
	related []Instance
	// templates is an array of template resources that will be applied to
	// produce textual output for the resource.
	templates []*Resource
	factory *Factory
	index *manifest.Index
}

// New instantiates a resource and all of its dependencies.
func New(target string, index *manifest.Index, factory *Factory) (*Resource, error) {
	root, getErr := index.Get(target)
	if getErr != nil {
		return nil, getErr
	}
	// Increase speed by filtering index to items used by root element only.
	deps, traverseErr := root.Traverse(index)
	if traverseErr != nil {
		return nil, traverseErr
	}
	// Calculate relationships for entire manifest listing.
	relations, relationsErr := append(deps, root).Relations(index)
	if relationsErr != nil {
		return nil, relationsErr
	}
	return newResource(
		root.ID(),
		root.Selector,
		deps.Indexed(),
		factory,
		relations,
		nil,
	)
}

// String returns a human readable representation of a resource.
func (r *Resource) String() string { return r.Manifest.String() }

// Body computes the content for a resource.
func (r *Resource) Body() (template.HTML, error) {
	var root *template.Template
	var body string
	var err error
	if root, err = r.buildTemplate(nil); err != nil {
		return "", err
	}
	if body, err = r.executeTemplate(root); err != nil {
		return "", err
	}
	return template.HTML(body), nil
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
func (r *Resource) Render(ctx context.Context) error {
	var err error
	for _, include := range r.included {
		if err = include.Render(ctx); err != nil {
			break
		}
	}
	if err == nil {
		if resource, ok := r.Instance.(Rendering); ok {
			err = resource.Render(ctx, r)
		}
	}
	if err != nil {
		return fmt.Errorf("%s: render: %w", r, err)
	}
	return nil
}

func (r *Resource) Related() *manifest.List {
	return (*r.Relations)[r.Manifest]
}

// newResource recursively instantiates a resources and all of its dependencies.
func newResource(
	name string,
	target *manifest.Selector,
	index *manifest.Index,
	factory *Factory,
	relations *manifest.Relations,
	root *Resource,
) (*Resource, error) {
	m, getErr := index.GetSelector(target)
	if getErr != nil {
		return nil, getErr
	}
	handler, handlerErr := factory.Handler(target)
	if handlerErr != nil {
		return nil, handlerErr
	}
	instance, err := handler.New(m)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", m, err)
	}
	el := &Resource{
		Name:      name,
		Manifest:  m,
		Relations: relations,
		Root:      root,
		Instance:  instance,
		Source:    handler.Source,
		Dest:      handler.Dest,
	}
	if root == nil {
		root = el
		el.Root = el
	}
	// Recursively instantiate included elements.
	for _, include := range m.Meta.Include {
		for _, i := range include.Expand(index) {
			dep, err := newResource(i.As, i.Resource, index, factory, relations, root)
			if err != nil {
				return nil, err
			}
			dep.Parent = el
			// If there are templates associated with this element, create
			// resources for them so they can be used during rendering.
			for _, templateSelector := range i.Templates {
				tmpl, err := el.New(templateSelector.ID(), templateSelector)
				if err != nil {
					return nil, err
				}
				tmpl.Parent = el
				dep.templates = append(dep.templates, tmpl)
			}
			el.included = append(el.included, dep)
		}
	}
	return el, nil
}

// buildTemplate recursively builds a template capable of rendering content for
// the resource.
func (r *Resource) buildTemplate(root *template.Template) (*template.Template, error) {
	if root == nil {
		root = template.New(r.Name).Funcs(template.FuncMap{
			"yield": func() (error, error) {
				return nil, errors.New("no supporting layout")
			},
		})
	} else if root.Lookup(r.Name) != nil {
		// Return early if the root template already contains the result for
		// this element.
		return root, nil
	}
	var err error
	// Add templates from all dependencies to the root template.
	for _, dep := range r.included {
		if root, err = dep.buildTemplate(root); err != nil {
			return nil, err
		}
	}
	// If there are a templates associated with this element, recursively add
	// them and their dependencies to the root template.
	for _, tmpl := range r.templates {
		if root, err = tmpl.buildTemplate(root); err != nil {
			return nil, err
		}
	}
	// If there is no template "wrapping" this element, and the element's
	// resource embeds template content, add that content and return early.
	if resource, ok := r.Instance.(Templating); ok {
		return root.New(r.Name).Parse(resource.Content());
	}
	// Render a string value for this element using the accumulated templates.
	var content string
	if content, err = r.executeTemplate(root); err != nil {
		return nil, err
	}
	// Make a rendered version of the element accessible under its name.
	return root.New(r.Name).Parse(content)
}

// executeTemplate renders the provided template using the element as context.
func (r *Resource) executeTemplate(tmpl *template.Template) (string, error) {
	if tmpl == nil {
		return "", fmt.Errorf("%s: executeTemplate with nil template", r)
	}
	var temp *template.Template
	var err error
	var buf bytes.Buffer
	// Make a clone of this template so it can be executed without preventing
	// further templates from being added by the caller.
	if temp, err = tmpl.Clone(); err != nil {
		return "", err
	}
	if err := temp.Execute(&buf, r); err != nil {
		return "", err
	}
	return r.applyTemplates(tmpl, buf.String())
}

// applyLayouts recursively applies layout templates.
func (r *Resource) applyTemplates(root *template.Template, yieldContent string) (string, error) {
	var temp *template.Template
	var err error
	var buf bytes.Buffer
	content := yieldContent
	for _, tmpl := range r.templates {
		buf.Reset()
		// Make a clone of this template so it can be executed without preventing
		// further templates from being added by the caller.
		if temp, err = root.Clone(); err != nil {
			return "", err
		}
		if err = temp.Lookup(tmpl.Name).Funcs(template.FuncMap{
			"yield": func() template.HTML {
				return template.HTML(content)
			},
		}).Execute(&buf, r); err != nil {
			return "", err
		}
		if content, err = tmpl.applyTemplates(root, buf.String()); err != nil {
			return "", err
		}
	}
	return content, nil
}
