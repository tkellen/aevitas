package image

import (
	"context"
	"fmt"
	"github.com/go-git/go-billy/v5"
	json "github.com/json-iterator/go"
	"github.com/tkellen/aevitas/pkg/resource"
	"strconv"
)

const KGVPng = "image/png/v1"

type Png struct {
	resource *resource.Resource
	Spec     *imageSpec
}

func NewPng(r *resource.Resource) (*Png, error) {
	instance := &Png{resource: r}
	if err := json.Unmarshal(r.Spec, &instance.Spec); err != nil {
		return nil, err
	}
	if err := instance.Validate(); err != nil {
		return nil, fmt.Errorf("%s\n%w", r.Spec, err)
	}
	return instance, nil
}

func (img *Png) Validate() error { return img.Spec.validate() }

func (img *Png) Render(ctx context.Context, r resource.Element) error {
	scopedDest, scopeErr := img.Spec.scope(r.Dest())
	if scopeErr != nil {
		return scopeErr
	}
	if img.Spec.current(scopedDest) {
		return nil
	}
	data, readErr := img.resource.Bytes(ctx, r.Source())
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

