package image

import (
	"context"
	"fmt"
	"github.com/go-git/go-billy/v5"
	json "github.com/json-iterator/go"
	"github.com/tkellen/aevitas/pkg/resource"
	"strconv"
)

const KGVGif = "image/gif/v1"

type Gif struct {
	resource *resource.Resource
	Spec     *imageSpec
}

func NewGif(r *resource.Resource) (*Gif, error) {
	instance := &Gif{resource: r}
	if err := json.Unmarshal(r.Spec, &instance.Spec); err != nil {
		return nil, err
	}
	if err := instance.Validate(); err != nil {
		return nil, fmt.Errorf("%s\n%w", r.Spec, err)
	}
	return instance, nil
}

func (img *Gif) Validate() error { return img.Spec.validate() }

func (img *Gif) Render(ctx context.Context, r resource.Element) error {
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
