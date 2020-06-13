package image

import (
	"context"
	"fmt"
	"github.com/go-git/go-billy/v5"
	json "github.com/json-iterator/go"
	"github.com/tkellen/aevitas/pkg/resource/v1"
	"strconv"
)

type Gif struct {
	Resource *resource.Resource
	Spec     *imageSpec
}

func NewGif(r *resource.Resource) (*Gif, error) {
	instance := &Gif{Resource: r}
	if err := json.Unmarshal(r.Spec, &instance.Spec); err != nil {
		return nil, err
	}
	if err := instance.Validate(); err != nil {
		return nil, fmt.Errorf("%s\n%w", r.Spec, err)
	}
	return instance, nil
}

func (img *Gif) Validate() error {
	return img.Spec.Validate()
}

func (img *Gif) Render(ctx context.Context, fs billy.Filesystem) error {
	scopeFs, scopeErr := img.Spec.Scope(fs)
	if scopeErr != nil {
		return scopeErr
	}
	if img.Spec.Current(scopeFs) {
		return nil
	}
	data, readErr := img.Resource.Bytes(ctx)
	if readErr != nil {
		return readErr
	}
	return img.Spec.Render(ctx, func(width int) error {
		return img.write(data, scopeFs, width)
	})
}

func (img *Gif) Content(ctx context.Context) ([]byte, error) { return img.Resource.Bytes(ctx) }

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
