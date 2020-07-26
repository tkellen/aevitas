// Package resource presents a common interface for rendering any resource type.
package resource

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/go-git/go-billy/v5"
	"github.com/orcaman/concurrent-map"
	"github.com/tkellen/aevitas/pkg/manifest"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"html/template"
	"os"
	"path"
	"reflect"
	"strings"
)

// Resource represents a manifest in a form that can be rendered.
type Resource struct {
	// Root is the top of the resource graph (the first resource instantiated).
	Root *Resource
	// Parent refers to the resource that caused this to be instantiated.
	Parent *Resource
	// NavigationScopedByParent indicates if navigation of this resource should
	// be limited in scope by the parent that created it. e.g. next/previous
	// posts by the topic that "owns" them.
	NavigationScopedByParent bool
	// id is a unique identifier for the resource.
	id string
	// manifest describes the raw data used to instantiate the resource.
	manifest *manifest.Manifest
	// renderAsChild defines resources that will be rendered with this resource as
	// the parent.
	renderAsChild []*Resource
	// embeds defines resources that will be made available by name during the
	// rendering of this resource.
	embeds []*Resource
	// instance holds the concrete instance of this resource.
	instance interface{}
	// renderTemplates defines resources that will be applied in to produce
	// text based output for the resource (typically html).
	renderTemplates []*Resource
	// factory is a reference to the config that determines how to instantiate
	// a concrete resource.
	factory       *Factory
	source        billy.Filesystem
	dest          billy.Filesystem
	index         *manifest.Index
	resourceCache cmap.ConcurrentMap
	templateCache cmap.ConcurrentMap
}

// String returns a human readable representation of a resource.
func (r *Resource) String() string {
	if r.id != r.manifest.Selector.ID() {
		return fmt.Sprintf("%s (%s)", r.manifest.Selector.ID(), r.id)
	}
	return r.id
}

// New instantiates a new resource from an existing one.
func (r *Resource) New(
	name string,
	target *manifest.Selector,
	renderTemplates []*manifest.Selector,
) (*Resource, error) {
	m, getErr := r.index.FindOne(target)
	if getErr != nil {
		return nil, getErr
	}
	// Check if a resource like this has already been instantiated. If it has,
	// make a copy using the correct parent. This saves substantial time during
	// rendering.
	cID := cacheID(m, renderTemplates)
	if cached, ok := r.resourceCache.Get(cID); ok {
		return cached.(*Resource).copyWithParent(name, r), nil
	}
	handler, handlerErr := r.factory.Handler(m)
	if handlerErr != nil {
		return nil, handlerErr
	}
	instance, err := handler.New(m)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", m, err)
	}
	el := &Resource{
		Parent:        r,
		Root:          r.Root,
		factory:       r.factory,
		source:        handler.source,
		dest:          handler.dest,
		id:            name,
		instance:      instance,
		manifest:      m,
		index:         r.index,
		resourceCache: r.resourceCache,
		templateCache: r.templateCache,
	}
	if el.Root == nil {
		el.Root = el
		el.Parent = nil
	}
	// If there is no further metadata, the instantiation of this resource is
	// complete.
	if m.Meta == nil {
		return el, nil
	}
	// If explicit selectors were not supplied for rendering this resource,
	// use the ones in the manifest.
	if renderTemplates == nil {
		renderTemplates = m.Meta.RenderTemplates
	}
	// Instantiate the resources this resource will be rendered with.
	for _, templateSelector := range renderTemplates {
		tmpl, err := el.New(templateSelector.ID(), templateSelector, nil)
		if err != nil {
			return nil, err
		}
		el.renderTemplates = append(el.renderTemplates, tmpl)
	}
	// Instantiate all embedded resources.
	if err := m.EachEmbed(r.index, func(rt *manifest.RenderTarget) error {
		dep, err := el.New(rt.Name, rt.Selector, rt.RenderTemplates)
		if err != nil {
			return err
		}
		el.embeds = append(el.embeds, dep)
		return nil
	}); err != nil {
		return nil, err
	}
	// Instantiate resources declared as renderAsChild of this one.
	if err := m.EachChild(r.index, func(rt *manifest.RenderTarget) error {
		dep, err := el.New(rt.Selector.ID(), rt.Selector, rt.RenderTemplates)
		if err != nil {
			return err
		}
		dep.NavigationScopedByParent = bool(rt.NavigationScopedByParent)
		el.renderAsChild = append(el.renderAsChild, dep)
		return nil
	}); err != nil {
		return nil, err
	}
	// Save this resource in cache.
	r.resourceCache.Set(cID, el)
	return el, nil
}

// Render recursively renders all resources associated with this resource.
func (r *Resource) Render(ctx context.Context, concurrency int64) error {
	sem := semaphore.NewWeighted(concurrency)
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		// Attempt to render the underlying instance. This handles the case
		// where the resource is an asset like an image or video or audio file.
		return r.renderInstance(ctx)
	})
	eg.Go(func() error {
		// Render every child of this resource.
		for _, child := range r.renderAsChild {
			if err := sem.Acquire(egCtx, 1); err != nil {
				return err
			}
			child := child
			eg.Go(func() error {
				defer sem.Release(1)
				return child.Render(egCtx, concurrency)
			})
		}
		return nil
	})
	eg.Go(func() error {
		// Attempt to render the underlying instance of every embedded resource.
		// This handles the case where the resource is an asset like an image or
		// video or audio file.
		for _, embed := range r.embeds {
			if err := sem.Acquire(egCtx, 1); err != nil {
				return err
			}
			embed := embed
			eg.Go(func() error {
				defer sem.Release(1)
				return embed.renderInstance(ctx)
			})
		}
		return nil
	})
	eg.Go(func() error {
		// Attempt to render a textual representation of this resource. This handles
		// the case where the resource is a piece of content.
		return r.renderContent()
	})
	return eg.Wait()
}

func (r *Resource) renderInstance(ctx context.Context) error {
	instance, ok := r.instance.(asRenderable)
	if !ok {
		return nil
	}
	dest := r.dest
	baseHref := r.Root.manifest.BaseHref()
	if baseHref != "" {
		var err error
		if dest, err = dest.Chroot(baseHref); err != nil {
			return err
		}
	}
	return instance.Render(ctx, r.source, dest)
}

func (r *Resource) renderContent() error {
	if _, ok := r.instance.(asContent); !ok {
		return nil
	}
	filePath, hrefErr := r.Href()
	if hrefErr != nil {
		return hrefErr
	}
	fmt.Fprintf(os.Stdout, "rendering %s\n", filePath)
	if stat, _ := r.dest.Stat(filePath); stat != nil && stat.Size() != 0 {
		return nil
	}
	content, bodyErr := r.Body()
	if bodyErr != nil {
		return bodyErr
	}
	file, createErr := r.dest.Create(filePath)
	if createErr != nil {
		return createErr
	}
	if _, writeErr := file.Write([]byte(content)); writeErr != nil {
		return writeErr
	}
	return file.Close()
}

// Spec provides a shorthand for renderTemplates to consume the spec of an underlying
// instance with `{{ .Spec... }}` as opposed to {{ .instance.Spec... }}`.
func (r *Resource) Spec() interface{} {
	return reflect.ValueOf(r.instance).Elem().FieldByName("Spec").Interface()
}

func (r *Resource) BaseHref(parents ...*Resource) string {
	hrefs := []string{r.manifest.BaseHref()}
	var parent *Resource
	if len(parents) == 0 && r.Root != r {
		parent = r.Root
	} else {
		parent = parents[0]
	}
	for parent != nil && parent.manifest != nil {
		if segment := parent.manifest.BaseHref(); segment != "" {
			hrefs = append([]string{segment}, hrefs...)
		}
		parent = parent.Parent
	}
	return path.Join(hrefs...)
}

// Href computes a link to the resource allowing the parent resource scope it.
func (r *Resource) Href() (string, error) {
	target := r
	if linkable, ok := target.instance.(asLinkable); ok {
		return path.Join(r.BaseHref(r.Parent), linkable.Href()), nil
	} else {
		return "", fmt.Errorf("%s does not define a href", r)
	}
}

// HrefCanonical computes that canonical reference to a resource ignoring all
// resources that have wrapped/scoped it except the root.
func (r *Resource) HrefCanonical() (string, error) {
	target := r
	if linkable, ok := target.instance.(asLinkable); ok {
		return path.Join(r.BaseHref(r.Root), linkable.Href()), nil
	} else {
		return "", fmt.Errorf("%s does not define a href", r)
	}
}

// Body computes the content for a resource using all associated renderTemplates.
func (r *Resource) Body() (string, error) {
	root, buildErr := r.template(nil)
	if buildErr != nil {
		return "", buildErr
	}
	body, err := executeTemplate(root, r)
	if err != nil {
		return "", err
	}
	return body, nil
}

func (r *Resource) Title() (string, error) {
	var titles []string
	parent := r
	for parent != nil {
		titled, ok := parent.instance.(asTitled)
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

func (r *Resource) Prev() (*Resource, error) { return r.navigate("prev") }
func (r *Resource) Next() (*Resource, error) { return r.navigate("next") }

func (r *Resource) Related(target string, parentForMatches ...*Resource) ([]*Resource, error) {
	parent := r
	if len(parentForMatches) == 1 {
		parent = parentForMatches[0]
	}
	var output []*Resource
	// Fail if an invalid selector is used.
	s, err := manifest.NewSelector(target)
	if err != nil {
		return nil, err
	}
	matches, findErr := r.index.FindManyWithRelation(s, r.manifest.Selector)
	// If the search finds no relations, return an empty set, we're done.
	if findErr != nil {
		return output, nil
	}
	// If there were matches, collect by instantiating resources for each.
	for _, match := range matches {
		resource, err := NewFromManifest(match, parent)
		if err != nil {
			return nil, err
		}
		output = append(output, resource)
	}
	return output, err
}

func (r *Resource) navigate(dir string) (*Resource, error) {
	var err error
	var match *manifest.Manifest
	index := r.index
	if r.NavigationScopedByParent {
		if index, err = r.index.RelatedIndex(r.Parent.manifest); err != nil {
			return nil, nil
		}
	}
	if dir == "next" {
		match = index.Next(r.manifest)
	}
	if dir == "prev" {
		match = index.Prev(r.manifest)
	}
	if match == nil {
		return nil, nil
	}
	return NewFromManifest(match, r)
}

// template recursively collects all renderTemplates needed to render content for the
// resource.
func (r *Resource) template(root *template.Template) (*template.Template, error) {
	if root == nil {
		root = template.New(r.id).Funcs(template.FuncMap{
			"yield": func() (error, error) {
				return nil, errors.New("no yield content available yet")
			},
		})
	} else if root.Lookup(r.id) != nil {
		// If the same template is used more than once during the rendering of
		// a given resource, don't compute it twice.
		return root.Lookup(r.id), nil
	}
	var err error
	// Compute output for all embeds so they can be used within the template.
	for _, dep := range r.embeds {
		if root, err = dep.template(root); err != nil {
			return nil, err
		}
	}
	// If the resource embeds content, assign that content as the "yield" body
	// so renderTemplates can wrap it.
	var yield string
	if instance, ok := r.instance.(asContent); ok {
		yield = instance.Body()
	}
	// If this resource is viewed through a template, render those renderTemplates
	// with this resource as context.
	for _, tmpl := range r.renderTemplates {
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
	return root.New(r.id).Parse(yield)
}

// copyWithParent makes a copy of the current resource and gives it a different
// parent.
func (r *Resource) copyWithParent(id string, parent *Resource) *Resource {
	return &Resource{
		Root:                     r.Root,
		Parent:                   parent,
		NavigationScopedByParent: r.NavigationScopedByParent,
		id:                       id,
		manifest:                 r.manifest,
		renderAsChild:            r.renderAsChild,
		embeds:                   r.embeds,
		instance:                 r.instance,
		renderTemplates:          r.renderTemplates,
		factory:                  r.factory,
		source:                   r.source,
		dest:                     r.dest,
		index:                    r.index,
		resourceCache:            r.resourceCache,
		templateCache:            r.templateCache,
	}
}

// New instantiates a resource and all of its dependencies.
func New(target string, index *manifest.Index, factory *Factory) (*Resource, error) {
	if err := index.ComputeRelations(); err != nil {
		return nil, err
	}
	selector, selectorErr := manifest.NewSelector(target)
	if selectorErr != nil {
		return nil, selectorErr
	}
	root, getErr := index.FindOne(selector)
	if getErr != nil {
		return nil, getErr
	}
	return (&Resource{
		factory:       factory,
		index:         index,
		resourceCache: cmap.New(),
		templateCache: cmap.New(),
	}).New(root.Selector.ID(), root.Selector, nil)
}

func NewFromManifest(source *manifest.Manifest, parent *Resource) (*Resource, error) {
	if source == nil {
		return nil, nil
	}
	resource, err := parent.New(source.Selector.ID(), source.Selector, source.Meta.RenderTemplates)
	if err != nil {
		return nil, err
	}
	resource.Parent = parent
	return resource, nil
}

func executeTemplate(root *template.Template, context *Resource) (string, error) {
	var cacheID string
	if context.Parent != nil {
		cacheID = context.Parent.id + context.id + root.Name()
		if cached, ok := context.templateCache.Get(cacheID); ok {
			return cached.(string), nil
		}
	}
	var buf bytes.Buffer
	temp, err := root.Clone()
	if err != nil {
		return "", err
	}
	if err := temp.Execute(&buf, context); err != nil {
		return "", err
	}
	result := buf.String()
	if cacheID != "" {
		context.templateCache.Set(cacheID, result)
	}
	return result, nil
}

// cacheID returns a unique identifier for a given manifest and associated
// selectors pointing to other manifests used to render it. This allows the
// expensive process of recursively instantiating resources to be sidestepped.
func cacheID(m *manifest.Manifest, renderTemplates []*manifest.Selector) string {
	var buffer bytes.Buffer
	buffer.WriteString(m.Selector.ID())
	if renderTemplates == nil || len(renderTemplates) == 0 {
		renderTemplates = m.Meta.RenderTemplates
	}
	for _, t := range renderTemplates {
		buffer.WriteString(t.ID())
	}
	for _, child := range m.Meta.RenderAsChild {
		buffer.WriteString(child.ID())
	}
	for _, embed := range m.Meta.Embed {
		buffer.WriteString(embed.ID())
	}
	return buffer.String()
}

// asRenderable represents a resource instance that can be rendered.
type asRenderable interface {
	Render(context.Context, billy.Filesystem, billy.Filesystem) error
}

// asContent represents a resource instance that has content.
type asContent interface{ Body() string }

// asLinkable represents a resource instance that can be linked to.
type asLinkable interface{ Href() string }

// asTitled represents a resource instance that has a title fragment.
type asTitled interface{ Title() string }
