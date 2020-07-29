package resource

import (
	"context"
	"github.com/go-git/go-billy/v5"
	"github.com/tkellen/aevitas/internal/selector"
	"github.com/tkellen/aevitas/pkg/manifest"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

// asset represents a instance that can be rendered.
type asset interface {
	Render(context.Context, billy.Filesystem, billy.Filesystem) error
}

type Tree struct {
	Root     *Resource
	toRender []*Resource
	assets   []*Resource
}

func NewTree(target string, index *manifest.Index, factory *Factory) (*Tree, error) {
	selector, selectorErr := selector.New(target)
	if selectorErr != nil {
		return nil, selectorErr
	}
	rootManifest, getErr := index.FindOne(selector)
	if getErr != nil {
		return nil, getErr
	}
	root, newErr := NewResource(index, factory, rootManifest)
	if newErr != nil {
		return nil, newErr
	}
	resources := flatten(root)
	return &Tree{
		Root:     root,
		toRender: resources,
		assets:   assets(resources),
	}, nil
}

func (t *Tree) Render(
	ctx context.Context,
	concurrency int64,
	watchAssets func(int, <-chan struct{}),
	watchPages func(int, <-chan struct{}),
) error {
	assetCount := len(t.assets)
	assetsProgress := make(chan struct{})
	eg, egCtx := errgroup.WithContext(ctx)
	if assetCount > 0 {
		go watchAssets(assetCount, assetsProgress)
		assetSem := semaphore.NewWeighted(concurrency)
		eg.Go(func() error {
			for _, item := range t.assets {
				if item.instance.asAsset == nil {
					continue
				}
				instance := item.instance
				if err := assetSem.Acquire(egCtx, 1); err != nil {
					return err
				}
				item := item
				eg.Go(func() error {
					defer func() {
						assetSem.Release(1)
						assetsProgress <- struct{}{}
					}()
					dest, err := instance.dest.Chroot(item.Root.HrefBase())
					if err != nil {
						return err
					}
					return instance.asAsset.Render(egCtx, instance.source, dest)
				})
			}
			return nil
		})
	}
	pagesProgress := make(chan struct{})
	if len(t.toRender) > 0 {
		go watchPages(len(t.toRender), pagesProgress)
		pageSem := semaphore.NewWeighted(concurrency)
		eg.Go(func() error {
			for _, item := range t.toRender {
				if err := pageSem.Acquire(egCtx, 1); err != nil {
					return err
				}
				item := item
				eg.Go(func() error {
					defer func() {
						pageSem.Release(1)
						pagesProgress <- struct{}{}
					}()
					return t.render(ctx, item)
				})
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}
	close(assetsProgress)
	close(pagesProgress)
	return nil
}

func (t *Tree) render(_ context.Context, target *Resource) error {
	instance := target.instance
	href := target.Href()
	if stat, _ := instance.dest.Stat(href); stat != nil && stat.Size() != 0 {
		return nil
	}
	content, contentErr := target.Content()
	if contentErr != nil {
		return contentErr
	}
	file, createErr := instance.dest.Create(href)
	if createErr != nil {
		return createErr
	}
	if _, writeErr := file.Write([]byte(content)); writeErr != nil {
		return writeErr
	}
	return file.Close()
}

// flatten recursively produces a flat array of all resource children.
func flatten(root *Resource) []*Resource {
	if root == nil {
		return []*Resource{}
	}
	accum := []*Resource{root}
	for _, child := range root.children {
		accum = append(accum, flatten(child)...)
	}
	return accum
}

// assets filters an array of resources to those which are assets.
func assets(resources []*Resource) []*Resource {
	seen := map[*manifest.Manifest]struct{}{}
	var assets []*Resource
	for _, resource := range resources {
		if _, ok := seen[resource.Manifest]; ok {
			continue
		}
		if resource.instance.asAsset != nil {
			assets = append(assets, resource)
		}
	}
	return assets
}
