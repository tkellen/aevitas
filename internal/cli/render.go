package cli

import (
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/tkellen/aevitas/internal/render"
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
	Progress    bool     `help:"Show progress during render operation"`
	AssetRoot   string   `required name:"asset" short:"a" type:"existingdir" help:"RenderTree path to assets." default:"${cwd}"`
	Output      string   `required name:"output" short:"o" help:"Path for output."`
	Selector    string   `arg required name:"selector" help:"manifest to render."`
}

func progress(ui *mpb.Progress, name string) func(count int, progress <-chan struct{}) {
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
	ui := mpb.New(
		mpb.WithWidth(180),
		mpb.WithRefreshRate(180*time.Millisecond),
		mpb.WithWaitGroup(&sync.WaitGroup{}),
	)
	bars := map[string]func(count int, progress <-chan struct{}){}
	if r.Progress {
		bars["stdin"] = progress(ui, "reading stdin")
		bars["file"] = progress(ui, "reading files")
		bars["asset"] = progress(ui, "render assets")
		bars["page"] = progress(ui, "render pages ")
	}
	eg := errgroup.Group{}
	queue := make(chan *manifest.Manifest)
	// Collect manifests provided over standard in.
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		eg.Go(func() error {
			list, err := manifest.NewFromReader(ctx.Stdin, bars["stdin"])
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
		list, err := manifest.NewFromDirs(r.Load, bars["file"])
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
	t, tErr := render.NewTree(r.Selector, index, factory)
	if tErr != nil {
		return tErr
	}
	if err := t.Render(
		ctx.Background,
		r.Concurrency,
		bars["asset"],
		bars["page"],
	); err != nil {
		return err
	}
	if r.Progress {
		ui.Wait()
	}
	return nil
}
