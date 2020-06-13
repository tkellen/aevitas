package render

import (
	"context"
	"fmt"
	"github.com/go-git/go-billy/v5"
	"github.com/tkellen/aevitas/pkg/resource/v1"
	audiov1 "github.com/tkellen/aevitas/pkg/resource/v1/audio"
	domainv1 "github.com/tkellen/aevitas/pkg/resource/v1/domain"
	imagev1 "github.com/tkellen/aevitas/pkg/resource/v1/image"
	textv1 "github.com/tkellen/aevitas/pkg/resource/v1/text"
	"html/template"
	"io"
)

func Do(ctx context.Context, fs billy.Filesystem, input io.Reader, target string) error {
	// Get index of all possible resources to render.
	resources, newErr := resource.NewListFromReader(input)
	if newErr != nil {
		return newErr
	}
	index := resources.Indexed()
	// Find target resource in the index.
	parent, lookupErr := index.Lookup(target)
	if lookupErr != nil {
		return lookupErr
	}
	// Traverse all direct dependencies to produce a smaller index for usage
	// when rendering this parent.
	deps, depsErr := index.Resolve(resource.List{parent}, nil)
	if depsErr != nil {
		return depsErr
	}
	renderList, err := traverse(nil, parent.ID(), parent, nil, deps.Indexed(), nil)
	if err != nil {
		return err
	}
	root := template.New("root")
	for _, item := range renderList {
		if err := item.Render(ctx, fs, root); err != nil {
			return err
		}
	}
	return nil
}

func traverse(
	accum []Element,
	name string,
	r *resource.Resource,
	t *resource.Resource,
	index resource.IndexedList,
	visited map[string]struct{},
) ([]Element, error) {
	if visited == nil {
		visited = map[string]struct{}{}
	}
	// Compute a unique ID for this resource "node" so we can see if we have
	// already collected it under this name.
	id := name+r.ID()
	if t != nil {
		id += t.ID()
	}
	// Skip nodes we've already computed.
	if _, ok := visited[id]; ok {
		return accum, nil
	}
	// Save that we've visited this node.
	visited[id] = struct{}{}
	// Collect rendering requirements for all imports.
	var imports []resource.Import
	var err error
	if imports, err = r.ExpandImports(index); err != nil {
		return nil, err
	}
	for _, imported := range imports {
		var importedR *resource.Resource
		var importedT *resource.Resource
		// If a template is associated with this import, recursively resolve its
		// rendering requirements.
		if imported.Template != (resource.Selector{}) {
			if importedT, err = imported.Template.From(index); err != nil {
				return nil, err
			}
			if accum, err = traverse(accum, importedT.ID(), importedT,nil, index, visited); err != nil {
				return nil, err
			}
		}
		// By default an import should be named the ID of the resource being
		// imported.
		importName := imported.Resource.ID()
		// If there is an import alias, use that for the name instead.
		if imported.As != "" {
			importName = imported.As
		}
		// Locate the resource this import refers to and recursively resolve its
		// rendering requirements.
		if importedR, err = imported.Resource.From(index); err != nil {
			return nil, err
		}
		if accum, err = traverse(accum, importName, importedR, importedT, index, visited); err != nil {
			return nil, err
		}
	}
	// If there is a template associated with this item, recursively resolve its
	// dependencies.
	if t != nil {
		if accum, err = traverse(accum, t.ID(), t,nil, index, visited); err != nil {
			return nil, err
		}
	}
	accum = append(accum, Element{
		Name: name,
		Resource: r,
		Template: t,
	})
	return accum, nil
}

type Element struct {
	Name string
	Template *resource.Resource
	Resource *resource.Resource
}

func (e Element) Render(ctx context.Context, fs billy.Filesystem, root *template.Template) error {
	var context interface{}
	r := e.Resource
	switch kgv := r.KGV(); kgv {
	case "image/jpeg/v1":
		asset, err := imagev1.NewJpeg(r)
		if err != nil {
			return err
		}
		if err = asset.Render(ctx, fs); err != nil {
			return err
		}
		context = asset
	case "image/png/v1":
		asset, err := imagev1.NewPng(r)
		if err != nil {
			return err
		}
		if err = asset.Render(ctx, fs); err != nil {
			return err
		}
		context = asset
	case "image/gif/v1":
		asset, err := imagev1.NewGif(r)
		if err != nil {
			return err
		}
		if err = asset.Render(ctx, fs); err != nil {
			return err
		}
		context = asset
	case "audio/mpeg/v1":
		asset, err := audiov1.NewMpeg(r)
		if err != nil {
			return err
		}
		if err = asset.Render(ctx, fs); err != nil {
			return err
		}
		context = asset
	case "text/template/v1", "text/html/v1":
		instance, err := textv1.NewHTML(r)
		if err != nil {
			return err
		}
		// If there is no associated template, just register the template body
		// so other templates can use it.
		if e.Template == nil {
			if root, err = root.New(r.Name).Parse(instance.Spec.Body); err != nil {
				return err
			}
		}
		context = instance
	case "domain/blog/v1":
		instance, err := domainv1.NewBlog(e.Resource)
		if err != nil {
			return err
		}
		if err := instance.Render(ctx, fs, root); err != nil {
			return err
		}
		context = instance
	default:
		return fmt.Errorf("%s: not registered in rendering system", kgv)
	}
	if e.Template != nil {
		instance, err := textv1.NewHTML(e.Template)
		if err != nil {
			return err
		}
		return instance.Render(root, e.Name, context)
	}
	return nil
}