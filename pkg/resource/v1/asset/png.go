package asset

import (
	"context"
	"github.com/go-git/go-billy/v5"
	json "github.com/json-iterator/go"
	"github.com/tkellen/aevitas/pkg/manifest"
	"strconv"
)

const KGVPng = "asset/png/v1"

type Png struct {
	*manifest.Manifest
	Source billy.Filesystem
	Dest   billy.Filesystem
	Spec   *imageSpec
}

func NewPng(m *manifest.Manifest) (*Png, error) {
	instance := &Png{
		Manifest: m,
		Spec:     &imageSpec{},
	}
	if err := json.Unmarshal(m.Spec, instance.Spec); err != nil {
		return nil, err
	}
	if instance.Spec.Description == "" {
		instance.Spec.Description = instance.Spec.Title
	}
	if instance.Spec.Body == "" {
		instance.Spec.Body = instance.Spec.Description
	}
	if instance.Spec.Href == "" {
		instance.Spec.Href = "index.html"
	}
	if err := instance.Validate(); err != nil {
		return nil, err
	}
	return instance, nil
}

func (img *Png) Validate() error { return img.Spec.validate() }
func (img *Png) Href() string    { return img.Spec.Href }
func (img *Png) Body() string    { return img.Spec.Body }
func (img *Png) Render(ctx context.Context, source billy.Filesystem, dest billy.Filesystem) error {
	scopedDest, scopeErr := dest.Chroot(img.Manifest.Meta.BaseHref)
	if scopeErr != nil {
		return scopeErr
	}
	if img.Spec.current(scopedDest) {
		return nil
	}
	data, readErr := bytes(img.Manifest, source)
	if readErr != nil {
		return readErr
	}
	return img.Spec.render(ctx, func(width int) error {
		return img.write(data, scopedDest, width)
	})
}

func (img *Png) write(src []byte, fs billy.Filesystem, width int) error {
	filePath := strconv.Itoa(width)
	file, openErr := fs.Create(filePath)
	if openErr != nil {
		return openErr
	}
	if _, err := file.Write(src); err != nil {
		return err
	}
	return file.Close()
}
