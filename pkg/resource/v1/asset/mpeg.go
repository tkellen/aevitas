package asset

import (
	"context"
	"github.com/go-git/go-billy/v5"
	"github.com/tkellen/aevitas/pkg/manifest"
	"path/filepath"
)

const KGVMpeg = "asset/mpeg/v1"

type Mpeg struct {
	*manifest.Manifest
}

func NewMpeg(m *manifest.Manifest) (*Mpeg, error) {
	instance := &Mpeg{
		Manifest: m,
	}
	return instance, nil
}

func (m *Mpeg) Render(ctx context.Context, source billy.Filesystem, dest billy.Filesystem) error {
	scopedDest, scopeErr := dest.Chroot(m.Meta.HrefPrefix)
	if scopeErr != nil {
		return scopeErr
	}
	filePath := m.Selector.Name
	if stat, _ := scopedDest.Stat(filePath); stat != nil && stat.Size() != 0 {
		return nil
	}
	if err := scopedDest.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}
	src, readErr := bytes(m.Manifest, source)
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
