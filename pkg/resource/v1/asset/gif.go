package asset

import (
	"context"
	"github.com/go-git/go-billy/v5"
	"github.com/tkellen/aevitas/pkg/manifest"
	"strconv"
)

const KGVGif = "asset/gif/v1"

type Gif struct {
	*manifest.Manifest
	Spec *imageSpec
}

func NewGif(m *manifest.Manifest) (*Gif, error) {
	spec, err := newImageSpec(m)
	if err != nil {
		return nil, err
	}
	return &Gif{
		Manifest: m,
		Spec:     spec,
	}, nil
}

func (img *Gif) Render(ctx context.Context, source billy.Filesystem, dest billy.Filesystem) error {
	scopedDest, scopeErr := dest.Chroot(img.Manifest.Meta.HrefBase)
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

func (img *Gif) write(src []byte, fs billy.Filesystem, width int) error {
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
