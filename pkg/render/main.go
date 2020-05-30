package render

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"github.com/go-git/go-billy/v5"
	"github.com/tkellen/aevitas/pkg/resource"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"io"
)

func Run(ctx context.Context, fs billy.Filesystem, input io.Reader, targets ...string) error {
	resources := resource.List{}
	reader := bufio.NewReader(input)
	for {
		manifest, err := reader.ReadBytes('\n')
		if errors.Is(err, io.EOF) {
			break
		}
		resource, err := resource.New(bytes.TrimRight(manifest, "\n"))
		if err != nil {
			return err
		}
		resources = append(resources, resource)
	}
	index := resources.Indexed()
	parents, selectErr := index.Select(targets...)
	if selectErr != nil {
		return selectErr
	}
	toRender, toRenderErr := index.Renderables(ctx, parents, nil)
	if toRenderErr != nil {
		return toRenderErr
	}
	sem := semaphore.NewWeighted(int64(10))
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		for _, item := range toRender {
			if egCtx.Err() != nil {
				return egCtx.Err()
			}
			if err := sem.Acquire(egCtx, 1); err != nil {
				return err
			}
			item := item
			eg.Go(func() error {
				defer sem.Release(1)
				scopeFs, scopeErr := item.Scope(fs)
				if scopeErr != nil {
					return scopeErr
				}
				if !item.Current(scopeFs) {
					return item.Render(ctx, fs)
				}
				return nil
			})
		}
		return nil
	})
	if err := eg.Wait(); err != nil {
		return err
	}
	return nil
}
