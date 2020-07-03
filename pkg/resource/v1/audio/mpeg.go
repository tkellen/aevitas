package audio

import (
	"context"
	"fmt"
	"github.com/go-git/go-billy/v5"
	json "github.com/json-iterator/go"
	"github.com/tkellen/aevitas/pkg/manifest"
	"github.com/tkellen/aevitas/pkg/resource"
	"path/filepath"
)

const KGVMpeg = "audio/mpeg/v1"

type Mpeg struct {
	*manifest.Manifest
	Source billy.Filesystem
	Dest   billy.Filesystem
	Spec   *MpegSpec
}

type MpegSpec struct {
	Title string
	Path  string
}

func NewMpeg(m *manifest.Manifest) (*Mpeg, error) {
	instance := &Mpeg{
		Manifest: m,
		Spec:     &MpegSpec{},
	}
	if err := json.Unmarshal(m.Spec, instance.Spec); err != nil {
		return nil, err
	}
	if err := instance.Validate(); err != nil {
		return nil, err
	}
	return instance, nil
}

func (m *Mpeg) Validate() error {
	if m.Spec.Title == "" {
		return fmt.Errorf("title must be defined")
	}
	return nil
}

func (m *Mpeg) Render(ctx context.Context, r *resource.Resource) error {
	scopedDest, scopeErr := r.Dest.Chroot(m.Spec.Path)
	if scopeErr != nil {
		return scopeErr
	}
	filePath := m.Spec.Path
	if stat, _ := scopedDest.Stat(filePath); stat != nil && stat.Size() != 0 {
		return nil
	}
	if err := scopedDest.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}
	src, readErr := r.Bytes(ctx)
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
