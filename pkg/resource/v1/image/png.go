package image

import (
	"context"
	"github.com/go-git/go-billy/v5"
	json "github.com/json-iterator/go"
	"github.com/tkellen/aevitas/pkg/manifest"
	"github.com/tkellen/aevitas/pkg/resource"
	"strconv"
)

const KGVPng = "image/png/v1"

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
	if err := instance.Validate(); err != nil {
		return nil, err
	}
	return instance, nil
}

func (img *Png) Validate() error { return img.Spec.validate() }
func (img *Png) Href() string    { return img.Spec.href() }
func (img *Png) Render(ctx context.Context, r *resource.Resource) error {
	scopedDest, scopeErr := img.Spec.scope(r.Dest)
	if scopeErr != nil {
		return scopeErr
	}
	if img.Spec.current(scopedDest) {
		return nil
	}
	data, readErr := r.Bytes(ctx)
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
