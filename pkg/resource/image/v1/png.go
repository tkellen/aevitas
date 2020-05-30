package v1

import (
	"context"
	"fmt"
	"github.com/go-git/go-billy/v5"
	json "github.com/json-iterator/go"
	metav1 "github.com/tkellen/aevitas/pkg/resource/meta/v1"
	"strconv"
)

type Png struct {
	Meta metav1.Meta
	Spec imageSpec
}

func NewPng(manifest []byte) (*Png, error) {
	var instance Png
	if err := json.Unmarshal(manifest, &instance); err != nil {
		return nil, err
	}
	if err := instance.Validate(); err != nil {
		return nil, fmt.Errorf("%s\n%w", manifest, err)
	}
	return &instance, nil
}

func (img *Png) Validate() error {
	return img.Spec.Validate()
}

func (img *Png) Deps(_ context.Context) ([]string, error) {
	return []string{}, nil
}

func (img *Png) Current(fs billy.Filesystem) bool {
	return img.Spec.Current(fs)
}

func (img *Png) Scope(fs billy.Filesystem) (billy.Filesystem, error) {
	return img.Spec.Scope(fs, img.Meta.Name)
}

func (img *Png) Render(ctx context.Context, fs billy.Filesystem) error {
	data, readErr := img.Meta.DataBytes(ctx)
	if readErr != nil {
		return readErr
	}
	return img.Spec.Render(ctx, func(width int) error {
		return img.write(data, fs, width)
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
