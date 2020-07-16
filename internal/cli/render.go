package cli

import (
	"fmt"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/tkellen/aevitas/pkg/manifest"
	"github.com/tkellen/aevitas/pkg/resource"
	audiov1 "github.com/tkellen/aevitas/pkg/resource/v1/audio"
	htmlv1 "github.com/tkellen/aevitas/pkg/resource/v1/html"
	imagev1 "github.com/tkellen/aevitas/pkg/resource/v1/image"
	websitev1 "github.com/tkellen/aevitas/pkg/resource/v1/website"
	"os"
)

type RenderCmd struct {
	Load        []string `name:"load" short:"l" type:"existingdir" help:"Directory containing manifests."`
	Concurrency int      `help:"Control how many parallel renders can be run" default:10`
	AssetRoot   string   `required name:"asset" short:"a" type:"existingdir" help:"Root path to assets." default:"${cwd}"`
	Output      string   `required name:"output" short:"o" help:"Path for output."`
	Selector    string   `arg required name:"selector" help:"Manifest to render."`
}

func (r *RenderCmd) Run(ctx *Context) error {
	stat, _ := ctx.Stdin.Stat()
	var manifests []*manifest.Manifest
	// Collect any manifests provided over standard in.
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		list, err := manifest.AllFromReader(ctx.Stdin)
		if err != nil {
			return err
		}
		manifests = append(manifests, list...)
	}
	// Collect manifests in provided paths.
	for _, path := range r.Load {
		list, err := manifest.AllFromDirectory(path)
		if err != nil {
			return err
		}
		manifests = append(manifests, list...)
	}
	index, indexErr := manifest.NewIndex(manifests, false)
	if indexErr != nil {
		return indexErr
	}
	// Establish registry to locate assets.
	inputRoot := osfs.New(r.AssetRoot)
	outputRoot := osfs.New(r.Output)
	factory := defaultFactory(inputRoot, outputRoot)
	ctx.Logger.Stderr.Printf("%s\n\n...rendering %s with concurrency of %d.\n", index, r.Selector, r.Concurrency)
	// Get element to render
	root, err := resource.New(r.Selector, index, factory)
	if err != nil {
		return err
	}
	return root.Render(ctx.Background, r.Concurrency)
}

func defaultFactory(
	source billy.Filesystem,
	dest billy.Filesystem,
) *resource.Factory {
	factory := resource.NewFactory(source, dest)
	factory.Register(fmt.Sprintf("%s/*/*", imagev1.KGVGif), func(m *manifest.Manifest) (interface{}, error) {
		return imagev1.NewGif(m)
	})
	factory.Register(fmt.Sprintf("%s/*/*", imagev1.KGVJpeg), func(m *manifest.Manifest) (interface{}, error) {
		return imagev1.NewJpeg(m)
	})
	factory.Register(fmt.Sprintf("%s/*/*", imagev1.KGVPng), func(m *manifest.Manifest) (interface{}, error) {
		return imagev1.NewPng(m)
	})
	factory.Register(fmt.Sprintf("%s/*/*", audiov1.KGVMpeg), func(m *manifest.Manifest) (interface{}, error) {
		return audiov1.NewMpeg(m)
	})
	factory.Register(fmt.Sprintf("%s/*/*", websitev1.KGVDomain), func(m *manifest.Manifest) (interface{}, error) {
		return websitev1.NewDomain(m)
	})
	factory.Register(fmt.Sprintf("%s/*/*", websitev1.KGVPage), func(m *manifest.Manifest) (interface{}, error) {
		return websitev1.NewPage(m)
	})
	factory.Register(fmt.Sprintf("%s/*/*", websitev1.KGVTaxonomy), func(m *manifest.Manifest) (interface{}, error) {
		return websitev1.NewTaxonomy(m)
	})
	factory.Register(fmt.Sprintf("%s/*/*", htmlv1.KGVTemplate), func(m *manifest.Manifest) (interface{}, error) {
		return htmlv1.NewTemplate(m)
	})
	return factory
}
