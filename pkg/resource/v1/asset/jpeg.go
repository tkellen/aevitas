package asset

import (
	"context"
	"github.com/disintegration/gift"
	"github.com/go-git/go-billy/v5"
	"github.com/pixiv/go-libjpeg/jpeg"
	"github.com/tkellen/aevitas/pkg/manifest"
	"image"
	"strconv"
)

const KGVJpeg = "asset/jpeg/v1"

type Jpeg struct {
	*manifest.Manifest
	Spec *imageSpec
}

func NewJpeg(m *manifest.Manifest) (*Jpeg, error) {
	spec, err := newImageSpec(m)
	if err != nil {
		return nil, err
	}
	return &Jpeg{
		Manifest: m,
		Spec:     spec,
	}, nil
}

func (img *Jpeg) Render(ctx context.Context, source billy.Filesystem, dest billy.Filesystem) error {
	scopedDest, scopeErr := dest.Chroot(img.Manifest.Meta.HrefBase)
	if scopeErr != nil {
		return scopeErr
	}
	if img.Spec.current(scopedDest) {
		return nil
	}
	src, readErr := reader(img.Manifest, source)
	if readErr != nil {
		return readErr
	}
	data, decodeErr := jpeg.Decode(src, &jpeg.DecoderOptions{})
	if decodeErr != nil {
		return decodeErr
	}
	return img.Spec.render(ctx, func(width int) error {
		return img.write(data, scopedDest, width)
	})
}

func (img *Jpeg) write(src image.Image, fs billy.Filesystem, width int) error {
	filePath := strconv.Itoa(width)
	file, openErr := fs.Create(filePath)
	if openErr != nil {
		return openErr
	}
	defer file.Close()
	g := gift.New(
		gift.Resize(width, 0, gift.LanczosResampling),
		gift.UnsharpMask(.25, 8, 0.065),
	)
	resized := image.NewRGBA(g.Bounds(src.Bounds()))
	g.Draw(resized, src)
	if err := jpeg.Encode(file, resized, &jpeg.EncoderOptions{Quality: 85}); err != nil {
		file.Close()
		fs.Remove(filePath)
		return err
	}
	return file.Close()
}
