package runner

import (
	"context"
	"fmt"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/tkellen/aevitas/pkg/manifest"
	"github.com/tkellen/aevitas/pkg/resource"
	audiov1 "github.com/tkellen/aevitas/pkg/resource/v1/audio"
	corev1 "github.com/tkellen/aevitas/pkg/resource/v1/core"
	imagev1 "github.com/tkellen/aevitas/pkg/resource/v1/image"
	websitev1 "github.com/tkellen/aevitas/pkg/resource/v1/website"
)

func DefaultFactory(
	source billy.Filesystem,
	dest billy.Filesystem,
) *resource.Factory {
	factory := resource.NewFactory(source, dest)
	factory.Register(fmt.Sprintf("*/%s/*", imagev1.KGVGif), func(m *manifest.Manifest) (resource.Instance, error) {
		return imagev1.NewGif(m)
	})
	factory.Register(fmt.Sprintf("*/%s/*", imagev1.KGVJpeg), func(m *manifest.Manifest) (resource.Instance, error) {
		return imagev1.NewJpeg(m)
	})
	factory.Register(fmt.Sprintf("*/%s/*", imagev1.KGVPng), func(m *manifest.Manifest) (resource.Instance, error) {
		return imagev1.NewPng(m)
	})
	factory.Register(fmt.Sprintf("*/%s/*", audiov1.KGVMpeg), func(m *manifest.Manifest) (resource.Instance, error) {
		return audiov1.NewMpeg(m)
	})
	factory.Register(fmt.Sprintf("*/%s/*", websitev1.KGVDomain), func(m *manifest.Manifest) (resource.Instance, error) {
		return websitev1.NewDomain(m)
	})
	factory.Register(fmt.Sprintf("*/%s/*", websitev1.KGVPage), func(m *manifest.Manifest) (resource.Instance, error) {
		return websitev1.NewPage(m)
	})
	factory.Register(fmt.Sprintf("*/%s/*", websitev1.KGVTaxonomy), func(m *manifest.Manifest) (resource.Instance, error) {
		return websitev1.NewTaxonomy(m)
	})
	factory.Register(fmt.Sprintf("*/%s/*", corev1.KGVTemplate), func(m *manifest.Manifest) (resource.Instance, error) {
		return corev1.NewTemplate(m)
	})
	factory.Register(fmt.Sprintf("*/%s/*", corev1.KGVRelation), func(m *manifest.Manifest) (resource.Instance, error) {
		return corev1.NewRelation(m)
	})
	return factory
}

func Render(ctx context.Context, target string) error {
	manifests, err := manifest.NewListFromDirectory("../../testdata/manifests")
	if err != nil {
		return err
	}
	source := osfs.New("../../testdata/asset")
	dest := osfs.New("../../build/test")
	reg := DefaultFactory(source, dest)
	root, rootErr := resource.New(target, manifests.Indexed(), reg)
	if rootErr != nil {
		return rootErr
	}
	return root.Render(ctx)
}
