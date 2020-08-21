package render

import (
	"bytes"
	"context"
	"github.com/tkellen/aevitas/pkg/manifest"
	"github.com/tkellen/aevitas/pkg/resource"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"io/ioutil"
	"os"
	"path/filepath"
)

type Tree struct {
	Root     *resource.Resource
	toRender []*resource.Resource
	assets   []*resource.Resource
	cacheDir string
}

func NewTree(target string, index *manifest.Index, factory *resource.Factory) (*Tree, error) {
	root, newErr := resource.New(index, target, factory)
	if newErr != nil {
		return nil, newErr
	}
	resources := root.Flatten()
	return &Tree{
		Root:     root,
		toRender: resources,
		assets:   assets(resources),
		cacheDir: ".cache",
	}, nil
}

func (t *Tree) Render(
	ctx context.Context,
	concurrency int64,
	watchAssets func(int, <-chan struct{}),
	watchPages func(int, <-chan struct{}),
) error {
	if err := os.MkdirAll(t.cacheDir, 0755); err != nil {
		return err
	}
	assetCount := len(t.assets)
	assetsProgress := make(chan struct{})
	eg, egCtx := errgroup.WithContext(ctx)
	if assetCount > 0 {
		if watchAssets != nil {
			go watchAssets(assetCount, assetsProgress)
		}
		assetSem := semaphore.NewWeighted(concurrency)
		eg.Go(func() error {
			for _, item := range t.assets {
				instance := item.Instance()
				if err := assetSem.Acquire(egCtx, 1); err != nil {
					return err
				}
				eg.Go(func() error {
					defer func() {
						assetSem.Release(1)
						if watchAssets != nil {
							assetsProgress <- struct{}{}
						}
					}()
					return instance.AsAsset.Render(egCtx, instance.Source, instance.Dest)
				})
			}
			return nil
		})
	}
	pagesProgress := make(chan struct{})
	if len(t.toRender) > 0 {
		if watchPages != nil {
			go watchPages(len(t.toRender), pagesProgress)
		}
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
						if watchPages != nil {
							pagesProgress <- struct{}{}
						}
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

func (t *Tree) isCached(target *resource.Resource, outputPath string) bool {
	cachePath := filepath.Join(t.cacheDir, target.ID())
	cachedContent, cacheErr := ioutil.ReadFile(cachePath)
	currentContent, currentErr := ioutil.ReadFile(outputPath)
	if cacheErr != nil || currentErr != nil {
		return false
	}
	// if the cached and current content match, the content is up to date
	if bytes.Equal(cachedContent, currentContent) {
		return true
	}
	// if we have an older cached copy, put it back
	if err := ioutil.WriteFile(outputPath, cachedContent, 0644); err == nil {
		return true
	}
	// if writing the older cached copy failed for some reason, trigger regen
	return false
}

func (t *Tree) cache(target *resource.Resource, content []byte) error {
	cachePath := filepath.Join(t.cacheDir, target.ID())
	return ioutil.WriteFile(cachePath, content, 0644)
}

func (t *Tree) render(_ context.Context, target *resource.Resource) error {
	// skip resources that have no output
	if target.Href() == "" || target.Href() == "/" {
		return nil
	}
	outputPath := filepath.Join(target.Instance().Dest.Root(), target.Href())
	if t.isCached(target, outputPath) {
		return nil
	}
	content, contentErr := target.Render()
	if contentErr != nil {
		return contentErr
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return err
	}
	contentBytes := []byte(content)
	file, createErr := os.Create(outputPath)
	if createErr != nil {
		return createErr
	}
	if _, writeErr := file.Write(contentBytes); writeErr != nil {
		return writeErr
	}
	if err := t.cache(target, contentBytes); err != nil {
		return err
	}
	return file.Close()
}

// assets filters an array of resources to those which are assets.
func assets(resources []*resource.Resource) []*resource.Resource {
	seen := map[*manifest.Manifest]struct{}{}
	var assets []*resource.Resource
	for _, resource := range resources {
		if _, ok := seen[resource.Manifest]; ok {
			continue
		}
		if resource.Instance().AsAsset != nil {
			assets = append(assets, resource)
		}
	}
	return assets
}
