package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-git/go-billy/v5"
	imagev1 "github.com/tkellen/aevitas/pkg/resource/image/v1"
	journalv1 "github.com/tkellen/aevitas/pkg/resource/journal/v1"
)

type Renderable interface {
	Render(context.Context, billy.Filesystem) error
	Current(billy.Filesystem) bool
	Scope(billy.Filesystem) (billy.Filesystem, error)
}

type Resource struct {
	Group    string
	Version  string
	Kind     string
	Manifest []byte
}

func New(manifest []byte) (*Resource, error) {
	var r Resource
	if err := json.Unmarshal(manifest, &r); err != nil {
		return nil, fmt.Errorf("%w: %s", err, manifest)
	}
	if err := r.Validate(); err != nil {
		return nil, err
	}
	r.Manifest = manifest
	return &r, nil
}

func (r *Resource) Validate() error {
	if r.Kind == "" || r.Group == "" || r.Version == "" {
		return fmt.Errorf("manifest must contain group, version and kind: %s", r.Manifest)
	}
	return nil
}

func (r *Resource) String() string {
	return fmt.Sprintf("%s: %s", r.KGV(), r.Manifest)
}

func (r *Resource) KGV() string {
	return r.Kind + "/" + r.Group + "/" + r.Version
}

func (r *Resource) Renderable() (Renderable, error) {
	kgv := r.KGV()
	switch kgv {
	case "image/jpg/v1":
		return imagev1.NewJpeg(r.Manifest)
	case "image/png/v1":
		return imagev1.NewPng(r.Manifest)
	case "image/gif/v1":
		return imagev1.NewGif(r.Manifest)
	case "journal/goingslowly/v1":
		return journalv1.NewGoingSlowly(r.Manifest)
	}
	return nil, fmt.Errorf("%s not renderable: %s", kgv, r)
}
