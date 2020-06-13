package image

import (
	"context"
	"fmt"
	"github.com/go-git/go-billy/v5"
	json "github.com/json-iterator/go"
	"github.com/tkellen/aevitas/pkg/resource/v1"
	"strconv"
)

type Png struct {
	Resource *resource.Resource
	Spec     *imageSpec
}

func NewPng(r *resource.Resource) (*Png, error) {
	instance := &Png{Resource: r}
	if err := json.Unmarshal(r.Spec, &instance.Spec); err != nil {
		return nil, err
	}
	if err := instance.Validate(); err != nil {
		return nil, fmt.Errorf("%s\n%w", r.Spec, err)
	}
	return instance, nil
}

func (img *Png) Validate() error {
	return img.Spec.Validate()
}

func (img *Png) Current(fs billy.Filesystem) bool {
	return img.Spec.Current(fs)
}

func (img *Png) Render(ctx context.Context, fs billy.Filesystem) error {
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

func (img *Png) Content(ctx context.Context) ([]byte, error) { return img.Resource.Bytes(ctx) }

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
