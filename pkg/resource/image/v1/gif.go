package v1

import (
	"context"
	"encoding/json"
	"github.com/go-git/go-billy/v5"
	metav1 "github.com/tkellen/aevitas/pkg/resource/meta/v1"
	"strconv"
)

type Gif struct {
	Meta metav1.Meta
	Spec imageSpec
}

func NewGif(config []byte) (*Gif, error) {
	var instance Gif
	if err := json.Unmarshal(config, &instance); err != nil {
		return nil, err
	}
	if err := instance.Validate(); err != nil {
		return nil, err
	}
	return &instance, nil
}

func (img *Gif) Validate() error { return img.Spec.Validate() }

func (img *Gif) Scope(fs billy.Filesystem) (billy.Filesystem, error) {
	return img.Spec.Scope(fs)
}

func (img *Gif) Current(fs billy.Filesystem) bool {
	return img.Spec.Current(fs)
}

func (img *Gif) Render(ctx context.Context, fs billy.Filesystem) error {
	data, readErr := img.Meta.DataBytes(ctx)
	if readErr != nil {
		return readErr
	}
	return img.Spec.Render(ctx, func(width int) error {
		return img.write(data, fs, width)
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
