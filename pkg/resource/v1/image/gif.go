package image

import (
	"context"
	"github.com/go-git/go-billy/v5"
	json "github.com/json-iterator/go"
	"github.com/tkellen/aevitas/pkg/manifest"
	"github.com/tkellen/aevitas/pkg/resource"
	"strconv"
)

const KGVGif = "image/gif/v1"

type Gif struct {
	*manifest.Manifest
	Spec *imageSpec
}

func NewGif(m *manifest.Manifest) (*Gif, error) {
	instance := &Gif{
		Manifest: m,
		Spec:     &imageSpec{},
	}
	if err := json.Unmarshal(m.Spec, instance.Spec); err != nil {
		return nil, err
	}
	if err := instance.Validate(); err != nil {
		return nil, err
	}
	return instance, nil
}

func (img *Gif) Validate() error { return img.Spec.validate() }
func (img *Gif) Href() string    { return img.Spec.href() }
func (img *Gif) Render(ctx context.Context, r *resource.Resource) error {
	scopedDest, scopeErr := img.Spec.scope(r.Dest)
	if scopeErr != nil {
		return scopeErr
	}
	if img.Spec.current(scopedDest) {
		return nil
	}
	data, readErr := r.Bytes(ctx)
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
