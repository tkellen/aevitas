package website

import (
	"context"
	"fmt"
	json "github.com/json-iterator/go"
	"github.com/tkellen/aevitas/pkg/manifest"
	"github.com/tkellen/aevitas/pkg/resource"
)

const KGVPage = "website/page/v1"

type Page struct {
	*manifest.Manifest
	Spec *PageSpec
}

type PageSpec struct {
	Path  string
	Title string
	Body  string
}

func NewPage(m *manifest.Manifest) (*Page, error) {
	instance := &Page{
		Manifest: m,
		Spec:     &PageSpec{},
	}
	if err := json.Unmarshal(m.Spec, instance.Spec); err != nil {
		return nil, err
	}
	if err := instance.Validate(); err != nil {
		return nil, err
	}
	return instance, nil
}

func (p *Page) Validate() error {
	if p.Spec.Path == "" {
		return fmt.Errorf("path must be defined")
	}
	if p.Spec.Title == "" {
		return fmt.Errorf("title must be defined")
	}
	if p.Spec.Body == "" {
		return fmt.Errorf("body must be defined")
	}
	return nil
}
func (p *Page) OutputPath() string { return p.Spec.Path }
func (p *Page) Content() string    { return p.Spec.Body }
func (p *Page) Synopsis() string   { return p.Spec.Body[0:50] }
func (p *Page) Render(_ context.Context, r *resource.Resource) error {
	if stat, _ := r.Dest.Stat(p.Spec.Path); stat != nil && stat.Size() != 0 {
		return nil
	}
	content, err := r.Body()
	if err != nil {
		return err
	}
	file, createErr := r.Dest.Create(p.Spec.Path)
	if createErr != nil {
		return createErr
	}
	if _, writeErr := file.Write([]byte(content)); writeErr != nil {
		return writeErr
	}
	return file.Close()
}
