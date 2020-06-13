package image

import (
	"context"
	"fmt"
	"github.com/disintegration/gift"
	"github.com/go-git/go-billy/v5"
	json "github.com/json-iterator/go"
	"github.com/pixiv/go-libjpeg/jpeg"
	"github.com/tkellen/aevitas/pkg/resource/v1"
	"image"
	"strconv"
)

type Jpeg struct {
	Resource *resource.Resource
	Spec     *imageSpec
}

func NewJpeg(r *resource.Resource) (*Jpeg, error) {
	instance := &Jpeg{Resource: r}
	if err := json.Unmarshal(r.Spec, &instance.Spec); err != nil {
		return nil, err
	}
	if err := instance.Validate(); err != nil {
		return nil, fmt.Errorf("%s\n%w", r.Spec, err)
	}
	return instance, nil
}

func (img *Jpeg) Validate() error {
	return img.Spec.Validate()
}

func (img *Jpeg) Current(fs billy.Filesystem) bool {
	return img.Spec.Current(fs)
}

func (img *Jpeg) Render(ctx context.Context, fs billy.Filesystem) error {
	scopeFs, scopeErr := img.Spec.Scope(fs)
	if scopeErr != nil {
		return scopeErr
	}
	if img.Spec.Current(scopeFs) {
		return nil
	}
	src, readErr := img.Resource.Reader(ctx)
	if readErr != nil {
		return readErr
	}
	image, decodeErr := jpeg.Decode(src, &jpeg.DecoderOptions{})
	if decodeErr != nil {
		return decodeErr
	}
	return img.Spec.Render(ctx, func(width int) error {
		return img.write(image, scopeFs, width)
	})
}

func (img *Jpeg) Content(ctx context.Context) ([]byte, error) { return img.Resource.Bytes(ctx) }

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
