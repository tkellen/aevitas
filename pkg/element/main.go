// Package element describes a resource and all of the dependent resources that
// are needed to render it.
package element

import (
	"bytes"
	"context"
	"fmt"
	"github.com/go-git/go-billy/v5"
	"github.com/tkellen/aevitas/pkg/resource"
	"github.com/tkellen/aevitas/pkg/selector"
	audiov1 "github.com/tkellen/aevitas/pkg/v1/audio"
	htmlv1 "github.com/tkellen/aevitas/pkg/v1/html"
	imagev1 "github.com/tkellen/aevitas/pkg/v1/image"
	websitev1 "github.com/tkellen/aevitas/pkg/v1/website"
	"html/template"
)

func new(rsrc *resource.Resource) (resource.Renderable, error) {
	switch kgv := rsrc.KGV(); kgv {
	case imagev1.KGVJpeg:
		return imagev1.NewJpeg(rsrc)
	case imagev1.KGVPng:
		return imagev1.NewPng(rsrc)
	case imagev1.KGVGif:
		return imagev1.NewGif(rsrc)
	case audiov1.KGVMpeg:
		return audiov1.NewMpeg(rsrc)
	case websitev1.KGVDomain:
		return websitev1.NewDomain(rsrc)
	case websitev1.KGVPage:
		return websitev1.NewPage(rsrc)
	case htmlv1.KGVFragment:
		return htmlv1.NewFragment(rsrc)
	default:
		return nil, fmt.Errorf("%s: not registered in rendering system", kgv)
	}
}

// Element describes an instance of a resource that can be rendered.
type Element struct {
	name         string
	resource     *resource.Resource
	template     *Element
	instance     resource.Renderable
	source       billy.Filesystem
	dest         billy.Filesystem
	deps         []*Element
	rendered     bool
	rootTmpl     *template.Template
}

// New creates a new element for rendering.
func New(
	name string,
	r selector.Selector,
	t selector.Selector,
	index resource.IndexedList,
	source billy.Filesystem,
	dest billy.Filesystem,
) (*Element, error) {
	var rsrc *resource.Resource
	var tmpl *Element
	var err error
	// Find resource in index based on import selector.
	if r == (selector.Selector{}) {
		return nil, fmt.Errorf("element must have a backing resource")
	}
	if rsrc, err = index.GetSelector(r); err != nil {
		return nil, err
	}
	if name == "" && rsrc != nil {
		name = rsrc.ID()
	}
	// Recursively collect dependencies required to render this element
	var deps []*Element
	for _, entry := range rsrc.Imports {
		for _, imported := range entry.Expand(index) {
			resolved, err := New(
				imported.As,
				imported.Resource,
				imported.Template,
				index,
				source,
				dest,
			)
			if err != nil {
				return nil, err
			}
			deps = append(deps, resolved)
		}
	}
	// If there is a template associated with this resource, create an element
	// for that too, it will need to be "rendered" first so it can be used to
	// render this element.
	if t != (selector.Selector{}) {
		if tmpl, err = New(t.ID(), t, selector.Selector{}, index, source, dest); err != nil {
			return nil, err
		}
	}
	instance, instanceErr := new(rsrc)
	if instanceErr != nil {
		return nil, instanceErr
	}
	return &Element{
		instance: instance,
		name:     name,
		resource: rsrc,
		template: tmpl,
		source:   source,
		dest:     dest,
		deps:     deps,
	}, nil
}

// String returns a human readable representation of the element.
func (e *Element) String() string {
	template := e.template
	if template != nil {
		return fmt.Sprintf("%s-with-%s", e.name, template.String())
	}
	return fmt.Sprintf("%s", e.resource.ID())
}

// Name returns the name of the element.
func (e *Element) Name() string { return e.name }

// Deps returns all dependencies of this element.
func (e *Element) Imports() []resource.Element {
	var elements []resource.Element
	for _, element := range e.deps {
		elements = append(elements, element)
	}
	return elements
}

// Resource returns the underlying resource for this element.
func (e *Element) Resource() *resource.Resource { return e.resource }

// Resource returns the underlying resource instance for this element.
func (e *Element) Instance() resource.Renderable { return e.instance }

// Source describes where external data for this element should be found.
func (e *Element) Source() billy.Filesystem { return e.source }

// Dest describes where data from rendering this element should be placed.
func (e *Element) Dest() billy.Filesystem { return e.dest }

// Body returns the result of rendering templates associated with the element.
func (e *Element) Body() (template.HTML, error) {
	content, err := e.content(nil)
	if err != nil {
		return "", err
	}
	return template.HTML(content), nil
}

// Render ensures this element and all of its dependencies are rendered.
func (e *Element) Render(ctx context.Context) error {
	if e.rendered {
		return nil
	}
	for _, dep := range e.deps {
		if err := dep.Render(ctx); err != nil {
			return err
		}
	}
	if e.template != nil {
		if err := e.template.Render(ctx); err != nil {
			return err
		}
	}
	e.rendered = true
	return e.instance.Render(ctx, e)
}

func (e *Element) content(root *template.Template) (string, error) {
	var buf bytes.Buffer
	var err error
	if root == nil {
		if root, err = e.tmpl(nil); err != nil {
			return "", err
		}
	}
	if root, err = root.Clone(); err != nil {
		return "", err
	}
	if err := root.Execute(&buf, e); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (e *Element) tmpl(root *template.Template) (*template.Template, error) {
	type hasTemplate interface {
		Template() *template.Template
	}
	if e.rootTmpl != nil {
		return e.rootTmpl, nil
	}
	var err error
	if root == nil {
		root = template.New(e.name)
	}
	// Add templates from all dependencies to the root template.
	for _, dep := range e.deps {
		if root, err = dep.tmpl(root); err != nil {
			return nil, err
		}
	}
	// If this element is itself a template, add it to the templates.
	tmpl, elementIsTemplate := e.instance.(hasTemplate)
	// If there is a template, recursively add all of its requirements as well.
	if e.template != nil {
		if root, err = e.template.tmpl(root); err != nil {
			return nil, err
		}
		var content string
		if elementIsTemplate {
			if content, err = (&Element{
				name:     e.resource.ID(),
				instance: e.instance,
				resource: e.resource,
				deps:     e.deps,
			}).content(root); err != nil {
				return nil, err
			}
		} else {
			if content, err = e.content(root); err != nil {
				return nil, err
			}
		}
		// Make the rendered version of the element accessible under its name.
		if root, err = root.New(e.name).Parse(content); err != nil {
			return nil, err
		}
	} else if elementIsTemplate {
		if root, err = root.AddParseTree(e.name, tmpl.Template().Tree); err != nil {
			return nil, err
		}
	}
	e.rootTmpl = root
	return root, nil
}