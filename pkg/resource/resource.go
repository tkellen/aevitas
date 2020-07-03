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
	Name     string
	Manifest *manifest.Manifest
	Template *Resource
	Layouts  []*Resource
	Root     *Resource
	Parent   *Resource
	Include  []*Resource
	Related  []*Resource
	Instance Instance
	Source   billy.Filesystem
	Dest     billy.Filesystem
}

// New instantiates a resource and all of its dependencies.
func New(target string, index manifest.IndexedList, factory *Factory) (*Resource, error) {
	root, getErr := index.Get(target)
	if getErr != nil {
		return nil, getErr
	}
	// Increase speed by filtering index to items used by root element only.
	deps, traverseErr := root.Traverse(index)
	if traverseErr != nil {
		return nil, traverseErr
	}
	return newResource(root.ID(), root.Selector, deps.Indexed(), factory, nil)
}

// String returns a human readable representation of a resource.
func (e *Resource) String() string {
	name := e.Name
	if e.Template != nil {
		name = fmt.Sprintf("%s-t(%s)", name, e.Template)
	}
	for _, layout := range e.Layouts {
		name = fmt.Sprintf("%s-l(%s)", name, layout)
	}
	return name
}

// Body computes the content for a resource.
func (e *Resource) Body() (template.HTML, error) {
	var root *template.Template
	var body string
	var err error
	if root, err = e.buildTemplate(nil); err != nil {
		return "", err
	}
	if body, err = e.executeTemplate(root); err != nil {
		return "", err
	}
	return template.HTML(body), nil
}

// Reader returns a io.ReadCloser for the data the resource points to.
func (e *Resource) Reader(_ context.Context) (io.ReadCloser, error) {
	return e.Source.Open(fmt.Sprintf("%s", e.Manifest.Meta.File))
}

// Bytes returns a byte array for the data the resource points to.
func (e *Resource) Bytes(ctx context.Context) ([]byte, error) {
	reader, fetchErr := e.Reader(ctx)
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
func (e *Resource) Render(ctx context.Context) error {
	var err error
	for _, include := range e.Include {
		if err = include.Render(ctx); err != nil {
			break
		}
	}
	if err == nil {
		if resource, ok := e.Instance.(Rendering); ok {
			err = resource.Render(ctx, e)
		}
	}
	if err != nil {
		return fmt.Errorf("%s: render: %w", e, err)
	}
	return nil
}

// newResource recursively instantiates a resources and all of its dependencies.
func newResource(
	name string,
	target *manifest.Selector,
	index manifest.IndexedList,
	factory *Factory,
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
		Name:     name,
		Manifest: m,
		Root:     root,
		Instance: instance,
		Source:   handler.Source,
		Dest:     handler.Dest,
	}
	if root == nil {
		root = el
		el.Root = el
	}
	// Recursively instantiate included elements.
	for _, include := range m.Meta.Include {
		for _, i := range include.Expand(index) {
			dep, err := newResource(i.As, i.Resource, index, factory, root)
			if err != nil {
				return nil, err
			}
			dep.Parent = el
			// If there is a template associated with this element, create
			// an element for it so it can be used during rendering.
			if i.Template != nil {
				tmpl, tmplErr := newResource(i.Template.ID(), i.Template, index, factory, root)
				if tmplErr != nil {
					return nil, err
				}
				tmpl.Parent = el
				dep.Template = tmpl
			}
			// If there are layouts associated with this element, create
			// elements for them so they can be used during rendering.
			for _, layoutSelector := range i.Layouts {
				layout, err := newResource(layoutSelector.ID(), layoutSelector, index, factory, root)
				if err != nil {
					return nil, err
				}
				layout.Parent = el
				dep.Layouts = append(dep.Layouts, layout)
			}
			el.Include = append(el.Include, dep)
		}
	}
	// Recursively instantiate related elements.
	for _, selector := range m.Meta.Related {
		for _, related := range selector.Expand(index) {
			resource, err := newResource(related.ID(), related, index, factory, root)
			if err != nil {
				return nil, err
			}
			resource.Parent = el
			el.Related = append(el.Related, resource)
		}
	}
	return el, nil
}

// buildTemplate recursively builds a template capable of rendering content for
// the resource.
func (e *Resource) buildTemplate(root *template.Template) (*template.Template, error) {
	if root == nil {
		root = template.New(e.Name).Funcs(template.FuncMap{
			"yield": func() (error, error) {
				return nil, errors.New("no supporting layout")
			},
		})
	} else if root.Lookup(e.Name) != nil {
		// Return early if the root template already contains the result for
		// this element.
		return root, nil
	}
	var err error
	// Add templates from all dependencies to the root template.
	for _, dep := range e.Include {
		if root, err = dep.buildTemplate(root); err != nil {
			return nil, err
		}
	}
	// If there is a template associated with this element, recursively add it
	// and its dependencies to the root template.
	if e.Template != nil {
		if root, err = e.Template.buildTemplate(root); err != nil {
			return nil, err
		}
	}
	// If there are a layouts associated with this element, recursively add them
	// and their dependencies to the root template.
	for _, layout := range e.Layouts {
		if root, err = layout.buildTemplate(root); err != nil {
			return nil, err
		}
	}
	// If there is no template "wrapping" this element, and the element's
	// resource embeds template content, add that content and return early.
	if resource, ok := e.Instance.(Templating); ok && e.Template == nil {
		return root.New(e.Name).Parse(resource.Content())
	}
	// Render a string value for this element using the accumulated templates.
	var content string
	if content, err = e.executeTemplate(root); err != nil {
		return nil, err
	}
	// Make a rendered version of the element accessible under its name.
	return root.New(e.Name).Parse(content)
}

// executeTemplate renders the provided template using the element as context.
func (e *Resource) executeTemplate(tmpl *template.Template) (string, error) {
	if tmpl == nil {
		return "", fmt.Errorf("%s: executeTemplate with nil template", e)
	}
	var temp *template.Template
	var err error
	var buf bytes.Buffer
	// Make a clone of this template so it can be executed without preventing
	// further templates from being added by the caller.
	if temp, err = tmpl.Clone(); err != nil {
		return "", err
	}
	if err := temp.Execute(&buf, e); err != nil {
		return "", err
	}
	return e.applyLayouts(tmpl, buf.String())
}

// applyLayouts recursively applies layout templates.
func (e *Resource) applyLayouts(tmpl *template.Template, yieldContent string) (string, error) {
	var err error
	var buf bytes.Buffer
	content := yieldContent
	for _, layout := range e.Layouts {
		buf.Reset()
		if err = tmpl.Lookup(layout.Name).Funcs(template.FuncMap{
			"yield": func() template.HTML {
				return template.HTML(content)
			},
		}).Execute(&buf, e); err != nil {
			return "", err
		}
		if content, err = layout.applyLayouts(tmpl, buf.String()); err != nil {
			return "", err
		}
	}
	return content, nil
}
