package resource

import (
	"bytes"
	"fmt"
	"github.com/orcaman/concurrent-map"
	"github.com/tkellen/aevitas/pkg/manifest"
	"html/template"
	"path"
	"reflect"
	"strconv"
)

// Resource describes a manifest with all of its selectors resolved in a form
// that can be used for rendering.
type Resource struct {
	*manifest.Manifest
	// Root refers to the top level resource.
	Root *Resource
	// Parent refers to the resource that caused this to be instantiated.
	Parent *Resource
	// children holds a reference to all resources which should be rendered as
	// a child of this one.
	children []*Resource
	// Templates holds the context needed to render this resource to text/html.
	templates []*Resource
	// Imports describes all resources required for rendering this resource.
	imports []*AsImported
	instance *instance
	// index provides a mechanism for resources dynamically create more
	index          *manifest.Index
	factory        *Factory
	resourceCache  cmap.ConcurrentMap
	parsedTemplate *template.Template
}

func NewResource(index *manifest.Index, factory *Factory, root *manifest.Manifest) (*Resource, error) {
	return (&Resource{
		index:         index,
		factory:       factory,
		resourceCache: cmap.New(),
	}).New(root, nil, true)
}

// collectChildren prevents cyclic dependencies
func (r *Resource) New(self *manifest.Manifest, templates manifest.Templates, collectChildren bool) (*Resource, error) {
	if len(templates) == 0 {
		templates = self.Render.Templates
	}
	resource := &Resource{
		Manifest:      self,
		Root:          r.Root,
		Parent:        r,
		index:         r.index,
		factory:       r.factory,
		resourceCache: r.resourceCache,
	}
	if resource.Root == nil {
		resource.Root = resource
		resource.Parent = nil
	}
	// Check if this manifest has already been instantiated as a resource with
	// these templates. If it has, use the descendents from it. This saves
	// substantial rendering time.
	cID := self.Selector.ID() + templates.ID() + strconv.FormatBool(collectChildren)
	if cached, ok := r.resourceCache.Get(cID); ok {
		asResource := cached.(*Resource)
		resource.instance = asResource.instance
		resource.templates = asResource.templates
		resource.children = asResource.children
		resource.imports = asResource.imports
		return resource, nil
	}
	// Instantiate underlying custom type.
	instance, newInstanceErr := newInstance(r.factory, self)
	if newInstanceErr != nil {
		return nil, fmt.Errorf("%s: %w", r, newInstanceErr)
	}
	resource.instance = instance
	// Populate all templates.
	resolvedTemplates, templateResolveErr := templates.Resolve(r.index)
	if templateResolveErr != nil {
		return nil, templateResolveErr
	}
	for _, template := range resolvedTemplates {
		t, newErr := resource.New(template, nil, false)
		if newErr != nil {
			return nil, newErr
		}
		resource.templates = append(resource.templates, t)
	}
	// Populate all children.
	if collectChildren {
		children, err := self.ResolveChildren(r.index)
		if err != nil {
			return nil, err
		}
		for _, child := range children {
			childAsResource, newErr := resource.New(child.Manifest, child.TemplateOverride, true)
			if newErr != nil {
				return nil, newErr
			}
			resource.children = append(resource.children, childAsResource)
		}
	}
	// Populate all imports.
	staticImports, resolveErr := self.ResolveStaticImports(r.index)
	if resolveErr != nil {
		return nil, resolveErr
	}
	var newImportedErr error
	if resource.imports, newImportedErr = newImported(resource, staticImports); newImportedErr != nil {
		return nil, newImportedErr
	}
	// Save resource to cache.
	r.resourceCache.Set(cID, resource)
	return resource, nil
}

func (r *Resource) Parents() []*Resource {
	var parents []*Resource
	node := r.Parent
	for node != nil {
		parents = append(parents, node)
		node = node.Parent
	}
	return parents
}

func (r *Resource) Titles() []string {
	var parts []string
	for _, node := range r.Parents() {
		if node.Meta.TitleBase != "" {
			parts = append(parts,node.Meta.TitleBase)
		}
	}
	return append([]string{r.Title()}, parts...)
}

// HrefBase computes the base href from all parents.
func (r *Resource) HrefBase() string {
	var parts []string
	parents := r.Parents()
	for i := len(parents)-1; i >= 0; i-- {
		parts = append(parts, parents[i].Meta.HrefBase)
	}
	return path.Join(parts...)
}

// Href computes a link to the resource allowing the parent resource scope it.
func (r *Resource) Href() string {
	return path.Join(r.HrefBase(), r.Manifest.Href())
}

// HrefBaseCanonical combines the href base from root and current resources
// only.
func (r *Resource) HrefBaseCanonical() string {
	return path.Join(r.Root.Meta.HrefBase, r.Meta.HrefBase)
}

// HrefCanonical computes the canonical reference to a resource ignoring all
// resources that have wrapped/scoped it except the root.
func (r *Resource) HrefCanonical() string {
	return path.Join(r.Root.Meta.HrefBase, r.Manifest.Href())
}

func (r *Resource) Prev() (*Resource, error) {
	return r.navigate("prev", nil)
}

func (r *Resource) Next() (*Resource, error) {
	return r.navigate("next", nil)
}

func (r *Resource) PrevInScope() (*Resource, error) {
	return r.navigate("prev", r.Parent.Manifest)
}

func (r *Resource) NextInScope() (*Resource, error) {
	return r.navigate("next", r.Parent.Manifest)
}

func (r *Resource) Spec() interface{} {
	return reflect.ValueOf(r.instance.self).Elem().FieldByName("Spec").Interface()
}

func (r *Resource) navigate(dir string, scope *manifest.Manifest) (*Resource, error) {
	var match *manifest.Manifest
	index := r.index
	if scope != nil {
		var err error
		if index, err = index.RelatedIndex(scope); err != nil {
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
	return r.New(match, nil, false)
}

func (r *Resource) Content() (template.HTML, error) {
	yield, bodyErr := r.body(nil, nil, "")
	if bodyErr != nil {
		return "", fmt.Errorf("%s: %w", r, bodyErr)
	}
	for _, tmpl := range r.templates {
		dynamicImportManifests, resolveErr := tmpl.ResolveDynamicImports(r.index, r.Manifest)
		if resolveErr != nil {
			return "", resolveErr
		}
		dynamicImports, err := newImported(r, dynamicImportManifests)
		if err != nil {
			return "", err
		}
		var bodyErr error
		if yield, bodyErr = tmpl.body(r, dynamicImports, yield); bodyErr != nil {
			return "", fmt.Errorf("%s: %w", r, bodyErr)
		}
	}
	return yield, nil
}

func (r *Resource) template(fns template.FuncMap) (*template.Template, error) {
	if r.parsedTemplate == nil {
		var err error
		if r.parsedTemplate, err = template.New("").Funcs(fns).Parse(r.Manifest.Body); err != nil {
			return nil, err
		}
	}
	return r.parsedTemplate, nil
}

func (r *Resource) body(context interface{}, relatedImports []*AsImported, yield template.HTML) (template.HTML, error) {
	var buf bytes.Buffer
	if context == nil {
		context = r
	}
	fns := map[string]interface{}{}
	for _, ai := range append(r.imports, relatedImports...) {
		fns[ai.Name] = ai.toTemplateFunc()
	}
	fns["yield"] = func() template.HTML { return yield }
	tmpl, tmplErr := r.template(fns)
	if tmplErr != nil {
		return "", tmplErr
	}
	if err := tmpl.Funcs(fns).Execute(&buf, context); err != nil {
		return "", fmt.Errorf("%s: %w", r, err)
	}
	return template.HTML(buf.String()), nil
}

// AsImported holds a reference to an imported resource.
type AsImported struct {
	Name       string
	Single     bool
	IsTemplate bool
	Instance   instance
	Resources  []*Resource
}

func newImported(parent *Resource, imported []*manifest.AsImported) ([]*AsImported, error) {
	var imports []*AsImported
	for _, imported := range imported {
		var importedResources []*Resource
		for _, item := range imported.Manifests {
			importedResource, newErr := parent.New(item, nil, false)
			if newErr != nil {
				return nil, newErr
			}
			importedResources = append(importedResources, importedResource)
		}
		imports = append(imports, &AsImported{
			Name:       imported.Name,
			Single:     imported.Single,
			IsTemplate: imported.IsTemplate,
			Resources:  importedResources,
		})
	}
	return imports, nil
}

func (ai *AsImported) toTemplateFunc() interface{} {
	if !ai.Single {
		return func() (interface{}, error) {
			return ai.Resources, nil
		}
	}
	if ai.IsTemplate {
		return func(context interface{}) (template.HTML, error) {
			if len(ai.Resources) == 0 {
				return "", fmt.Errorf("%s not found", ai.Name)
			}
			return ai.Resources[0].body(context, nil, "")
		}
	}
	return func() (interface{}, error) {
		if len(ai.Resources) == 0 {
			return "", fmt.Errorf("%s not found", ai.Name)
		}
		return ai.Resources[0], nil
	}
}
