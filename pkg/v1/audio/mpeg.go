package audio

import (
	"context"
	"fmt"
	json "github.com/json-iterator/go"
	"github.com/tkellen/aevitas/pkg/resource"
	"path/filepath"
)

const KGVMpeg = "audio/mpeg/v1"

type Mpeg struct {
	resource *resource.Resource
	Spec     MpegSpec
}

type MpegSpec struct {
	Title string
	Path  string
}

func NewMpeg(r *resource.Resource) (*Mpeg, error) {
	instance := &Mpeg{resource: r}
	if err := json.Unmarshal(r.Spec, &instance.Spec); err != nil {
		return nil, err
	}
	if err := instance.Validate(); err != nil {
		return nil, fmt.Errorf("%s\n%w", r.Manifest, err)
	}
	return instance, nil
}

func (m *Mpeg) Validate() error {
	if m.Spec.Title == "" {
		return fmt.Errorf("spec.title must be defined")
	}
	return nil
}

func (m *Mpeg) Render(ctx context.Context, r resource.Element) error {
	scopedDest, scopeErr := r.Dest().Chroot(m.Spec.Path)
	if scopeErr != nil {
		return scopeErr
	}
	filePath := m.resource.Name
	if err := scopedDest.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}
	src, readErr := m.resource.Bytes(ctx, r.Source())
	if readErr != nil {
		return readErr
	}
	file, openErr := scopedDest.Create(filePath)
	if openErr != nil {
		return openErr
	}
	file.Write(src)
	return file.Close()
}
