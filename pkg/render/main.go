package render

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/go-git/go-billy/v5"
	"github.com/tkellen/aevitas/internal/logging"
	"github.com/tkellen/aevitas/pkg/resource"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"io"
	"strings"
)

type pipeline []*resource.Resource

func (p pipeline) String() string {
	counts := make(map[string]int, len(p))
	for _, item := range p {
		kgv := item.KGV()
		if _, ok := counts[kgv]; !ok {
			counts[kgv] = 0
		}
		counts[kgv] = counts[kgv] + 1
	}
	var totals []string
	for kgv, count := range counts {
		totals = append(totals, fmt.Sprintf("%s: %d", kgv, count))
	}
	return strings.Join(totals, "\n")
}

func Run(ctx context.Context, _ *logging.Streams, fs billy.Filesystem, input io.Reader) error {
	work := pipeline{}
	reader := bufio.NewReader(input)
	for {
		manifest, err := reader.ReadBytes('\n')
		if errors.Is(err, io.EOF) {
			break
		}
		resource, err := resource.New(manifest)
		if err != nil {
			return err
		}
		work = append(work, resource)
	}
	sem := semaphore.NewWeighted(int64(1))
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		for _, item := range work {
			if egCtx.Err() != nil {
				return egCtx.Err()
			}
			if err := sem.Acquire(egCtx, 1); err != nil {
				return err
			}
			item := item
			eg.Go(func() error {
				defer sem.Release(1)
				return process(egCtx, fs, item)
			})
		}
		return nil
	})
	if err := eg.Wait(); err != nil {
		return err
	}
	return nil
}

func process(ctx context.Context, baseFs billy.Filesystem, r *resource.Resource) error {
	element, err := r.Renderable()
	if err != nil {
		return err
	}
	fs, scopeErr := element.Scope(baseFs)
	if scopeErr != nil {
		return scopeErr
	}
	if !element.Current(fs) {
		return element.Render(ctx, fs)
	}
	return nil
}
