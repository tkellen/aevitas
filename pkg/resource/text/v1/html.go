package v1

import (
	"context"
	"fmt"
	"github.com/go-git/go-billy/v5"
	json "github.com/json-iterator/go"
	metav1 "github.com/tkellen/aevitas/pkg/resource/meta/v1"
	"path/filepath"
	"time"
)

type Html struct {
	Meta metav1.Meta
	Spec HtmlSpec
}

type HtmlSpec struct {
	CreatedAt time.Time `json:"createdAt"`
	Include   []string
}

type Resource struct {
	GVK      string
	Selector map[string]string
}

func NewHtml(manifest []byte) (*Html, error) {
	var instance Html
	if err := json.Unmarshal(manifest, &instance); err != nil {
		return nil, err
	}
	if err := instance.Validate(); err != nil {
		return nil, err
	}
	return &instance, nil
}

func (h *Html) Validate() error { return nil }

func (h *Html) Scope(fs billy.Filesystem) (billy.Filesystem, error) {
	return fs, nil
}

func (h *Html) Current(fs billy.Filesystem) bool {
	if stat, _ := fs.Stat(h.file()); stat != nil && stat.Size() != 0 {
		return true
	}
	return false
}

func (h *Html) Deps(ctx context.Context) ([]string, error) {
	return h.Spec.Include, nil
	/*
		src, readErr := h.Meta.DataBytes(ctx)
		if readErr != nil {
			return nil, readErr
		}
		return regexp.MustCompile(`\[\|[^\|\]]*\|\]`).FindAllString(string(src), -1), nil
	*/
}

func (h *Html) file() string {
	year, month, _ := h.Spec.CreatedAt.Date()
	return filepath.Join("/", fmt.Sprintf("%d", year), fmt.Sprintf("%02d", month), h.Meta.Name)
}

func (h *Html) Render(ctx context.Context, fs billy.Filesystem) error {
	filePath := h.file()
	if err := fs.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}
	src, readErr := h.Meta.DataBytes(ctx)
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
