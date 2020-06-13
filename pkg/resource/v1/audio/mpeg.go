package audio

import (
	"context"
	"fmt"
	"github.com/go-git/go-billy/v5"
	json "github.com/json-iterator/go"
	"github.com/tkellen/aevitas/pkg/resource/v1"
	"path/filepath"
)

type Mpeg struct {
	Resource *resource.Resource
	Spec     MpegSpec
}

type MpegSpec struct {
	Title string
}

func NewMpeg(r *resource.Resource) (*Mpeg, error) {
	instance := &Mpeg{Resource: r}
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
func (m *Mpeg) scope(fs billy.Filesystem) (billy.Filesystem, error) {
	return fs.Chroot(filepath.Join("asset", "audio", "field-recording"))
}
func (m *Mpeg) current(fs billy.Filesystem) bool {
	if stat, _ := fs.Stat(m.file()); stat != nil && stat.Size() != 0 {
		return true
	}
	return false
}
func (m *Mpeg) file() string                                { return m.Resource.Name }
func (m *Mpeg) Content(ctx context.Context) ([]byte, error) { return m.Resource.Bytes(ctx) }
func (m *Mpeg) Render(ctx context.Context, fs billy.Filesystem) error {
	filePath := m.file()
	if err := fs.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}
	src, readErr := m.Resource.Bytes(ctx)
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