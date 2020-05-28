package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-git/go-billy/v5"
	metav1 "github.com/tkellen/aevitas/pkg/resource/meta/v1"
	"path/filepath"
	"strings"
	"time"
)

type GoingSlowly struct {
	Meta metav1.Meta
	Spec GoingSlowlySpec
}

type GoingSlowlySpec struct {
	Slug      string
	CreatedAt time.Time `json:"createdAt"`
}

func NewGoingSlowly(manifest []byte) (*GoingSlowly, error) {
	var instance GoingSlowly
	if err := json.Unmarshal(manifest, &instance); err != nil {
		return nil, err
	}
	if err := instance.Validate(); err != nil {
		return nil, err
	}
	return &instance, nil
}

func (gs *GoingSlowly) Validate() error {
	if gs.Spec.Slug == "" {
		return fmt.Errorf("slug must be defined")
	}
	return nil
}

func (gs *GoingSlowly) Scope(fs billy.Filesystem) (billy.Filesystem, error) {
	return fs.Chroot("journal.goingslowly.com")
}

func (gs *GoingSlowly) Current(fs billy.Filesystem) bool {
	if stat, _ := fs.Stat(gs.file()); stat != nil && stat.Size() != 0 {
		return true
	}
	return false
}

func (gs *GoingSlowly) file() string {
	year, month, _ := gs.Spec.CreatedAt.Date()
	return filepath.Join("/", fmt.Sprintf("%d", year), fmt.Sprintf("%02d", month), strings.TrimPrefix(gs.Spec.Slug, fmt.Sprintf("/%d/%02d", year, month)))
}

func (gs *GoingSlowly) Render(ctx context.Context, fs billy.Filesystem) error {
	filePath := gs.file()
	if err := fs.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}
	src, readErr := gs.Meta.DataBytes(ctx)
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
