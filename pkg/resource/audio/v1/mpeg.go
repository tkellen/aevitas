package v1

import (
	"context"
	"fmt"
	"github.com/go-git/go-billy/v5"
	json "github.com/json-iterator/go"
	metav1 "github.com/tkellen/aevitas/pkg/resource/meta/v1"
	"path/filepath"
)

type Mpeg struct {
	Meta metav1.Meta
	Spec MpegSpec
}

type MpegSpec struct {
	Name string
}

func NewMpeg(manifest []byte) (*Mpeg, error) {
	var instance Mpeg
	if err := json.Unmarshal(manifest, &instance); err != nil {
		return nil, err
	}
	if err := instance.Validate(); err != nil {
		return nil, fmt.Errorf("%s\n%w", manifest, err)
	}
	return &instance, nil
}

func (m *Mpeg) Validate() error {
	if m.Spec.Name == "" {
		return fmt.Errorf("spec.name must be defined")
	}
	return nil
}

func (m *Mpeg) Deps(_ context.Context) ([]string, error) { return []string{}, nil }

func (m *Mpeg) Scope(fs billy.Filesystem) (billy.Filesystem, error) {
	return fs.Chroot(filepath.Join("asset", "audio", "field-recording"))
}

func (m *Mpeg) Current(fs billy.Filesystem) bool {
	if stat, _ := fs.Stat(m.file()); stat != nil && stat.Size() != 0 {
		return true
	}
	return false
}

func (m *Mpeg) file() string {
	return m.Meta.Name
}

func (m *Mpeg) Render(ctx context.Context, fs billy.Filesystem) error {
	filePath := m.file()
	if err := fs.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}
	src, readErr := m.Meta.DataBytes(ctx)
	if readErr != nil {
		return readErr
	}
	dest, openErr := fs.Create(filePath)
	if openErr != nil {
		return openErr
	}
	dest.Write(src)
	return dest.Close()
}
