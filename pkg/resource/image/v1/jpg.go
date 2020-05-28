package v1

import (
	"context"
	"encoding/json"
	"github.com/disintegration/gift"
	"github.com/go-git/go-billy/v5"
	"github.com/pixiv/go-libjpeg/jpeg"
	metav1 "github.com/tkellen/aevitas/pkg/resource/meta/v1"
	"image"
	"strconv"
)

type Jpeg struct {
	Meta metav1.Meta
	Spec *imageSpec
}

func NewJpeg(manifest []byte) (*Jpeg, error) {
	var instance Jpeg
	if err := json.Unmarshal(manifest, &instance); err != nil {
		return nil, err
	}
	if err := instance.Validate(); err != nil {
		return nil, err
	}
	return &instance, nil
}

func (img *Jpeg) Validate() error { return img.Spec.Validate() }

func (img *Jpeg) Scope(fs billy.Filesystem) (billy.Filesystem, error) {
	return img.Spec.Scope(fs)
}

func (img *Jpeg) Current(fs billy.Filesystem) bool {
	return img.Spec.Current(fs)
}

func (img *Jpeg) Render(ctx context.Context, fs billy.Filesystem) error {
	src, readErr := img.Meta.DataReader(ctx)
	if readErr != nil {
		return readErr
	}
	image, decodeErr := jpeg.Decode(src, &jpeg.DecoderOptions{})
	if decodeErr != nil {
		return decodeErr
	}
	return img.Spec.Render(ctx, func(width int) error {
		return img.write(image, fs, width)
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
