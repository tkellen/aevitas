package cli

import (
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/tkellen/aevitas/pkg/manifest"
	"github.com/tkellen/aevitas/pkg/resource"
	"github.com/vbauerster/mpb/v5"
	"github.com/vbauerster/mpb/v5/decor"
	"golang.org/x/sync/errgroup"
	"os"
	"sync"
	"time"
)

type RenderCmd struct {
	Load        []string `name:"load" short:"l" type:"existingdir" help:"Directory containing manifests."`
	Concurrency int64    `help:"Control how many parallel renders can be run" default:10`
	AssetRoot   string   `required name:"asset" short:"a" type:"existingdir" help:"Tree path to assets." default:"${cwd}"`
	Output      string   `required name:"output" short:"o" help:"Path for output."`
	Selector    string   `arg required name:"selector" help:"manifest to render."`
}

func progress(wg sync.WaitGroup, ui *mpb.Progress, name string) func(count int, progress <-chan struct{}) {
	wg.Add(1)
	return func(count int, progress <-chan struct{}) {
		bar := ui.AddBar(
			int64(count),
			mpb.PrependDecorators(
				decor.Name(" "+name),
			),
			mpb.AppendDecorators(
				decor.Counters(count, "%d / %d ", decor.WCSyncSpace),
			),
		)
		for range progress {
			bar.Increment()
		}
	}
}

func (r *RenderCmd) Run(ctx *Context) error {
	stat, _ := ctx.Stdin.Stat()
	var wg sync.WaitGroup
	// pass &wg (optional), so p will wait for it eventually
	ui := mpb.New(
		mpb.WithWidth(180),
		mpb.WithRefreshRate(180*time.Millisecond),
		mpb.WithWaitGroup(&wg),
	)
	eg := errgroup.Group{}
	queue := make(chan *manifest.Manifest)
	// Collect manifests provided over standard in.
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		eg.Go(func() error {
			list, err := manifest.NewFromReader(ctx.Stdin, progress(wg, ui, "reading stdin"))
			if err != nil {
				return err
			}
			for _, manifest := range list {
				queue <- manifest
			}
			return nil
		})
	}
	// Collect manifests in provided paths.
	eg.Go(func() error {
		list, err := manifest.NewFromDirs(r.Load, progress(wg, ui, "reading files"))
		if err != nil {
			return err
		}
		for _, manifest := range list {
			queue <- manifest
		}
		return nil
	})
	collect := errgroup.Group{}
	var manifests []*manifest.Manifest
	collect.Go(func() error {
		for manifest := range queue {
			manifests = append(manifests, manifest)
		}
		return nil
	})
	if err := eg.Wait(); err != nil {
		return err
	}
	close(queue)
	if err := collect.Wait(); err != nil {
		return err
	}
	// Index manifests.
	index := manifest.NewIndex()
	if err := index.Insert(manifests...); err != nil {
		return err
	}
	if err := index.Collate(); err != nil {
		return err
	}
	// Establish registry to locate assets.
	inputRoot := osfs.New(r.AssetRoot)
	outputRoot := osfs.New(r.Output)
	factory := resource.DefaultFactory(inputRoot, outputRoot)
	rt, rsErr := resource.NewTree(r.Selector, index, factory)
	if rsErr != nil {
		return rsErr
	}
	if err := rt.Render(
		ctx.Background,
		r.Concurrency,
		progress(wg, ui, "render assets"),
		progress(wg, ui, "render pages "),
	); err != nil {
		return err
	}
	wg.Wait()
	return nil
}

