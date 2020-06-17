package image

import (
	"context"
	"fmt"
	"github.com/disintegration/gift"
	"github.com/go-git/go-billy/v5"
	json "github.com/json-iterator/go"
	"github.com/pixiv/go-libjpeg/jpeg"
	"github.com/tkellen/aevitas/pkg/resource"
	"image"
	"strconv"
)

const KGVJpeg = "image/jpeg/v1"

type Jpeg struct {
	resource *resource.Resource
	Spec     *imageSpec
}

func NewJpeg(r *resource.Resource) (*Jpeg, error) {
	instance := &Jpeg{resource: r}
	if err := json.Unmarshal(r.Spec, &instance.Spec); err != nil {
		return nil, err
	}
	if err := instance.Validate(); err != nil {
		return nil, fmt.Errorf("%s\n%w", r.Spec, err)
	}
	return instance, nil
}

func (img *Jpeg) Validate() error { return img.Spec.validate() }

func (img *Jpeg) Render(ctx context.Context, r resource.Element) error {
	scopedDest, scopeErr := img.Spec.scope(r.Dest())
	if scopeErr != nil {
		return scopeErr
	}
	if img.Spec.current(scopedDest) {
		return nil
	}
	src, readErr := img.resource.Reader(ctx, r.Source())
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
