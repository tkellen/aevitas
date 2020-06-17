package website

import (
	"context"
	"fmt"
	json "github.com/json-iterator/go"
	"github.com/tkellen/aevitas/pkg/resource"
	"html/template"
)

const KGVPage = "website/page/v1"

type Page struct {
	resource *resource.Resource
	template *template.Template
	Spec     *PageSpec
}

type PageSpec struct {
	Path  string
	Title string
	Body string
}

func NewPage(r *resource.Resource) (*Page, error) {
	instance := &Page{resource: r}
	if err := json.Unmarshal(r.Spec, &instance.Spec); err != nil {
		return nil, fmt.Errorf("%s\n%w", r.ID(), err)
	}
	if err := instance.Validate(); err != nil {
		return nil, fmt.Errorf("%s\n%w", r.Spec, err)
	}
	var err error
	if instance.template, err = template.New("root").Parse(instance.Spec.Body); err != nil {
		return nil, err
	}
	return instance, nil
}

func (p *Page) Validate() error { return nil }

func (p *Page) Template() *template.Template { return p.template }

func (p *Page) Render(_ context.Context, r resource.Element) error {
	content, err := r.Body()
	if err != nil {
		return err
	}
	file, createErr := r.Dest().Create(p.Spec.Path)
	if createErr != nil {
		return createErr
	}
	if _, writeErr := file.Write([]byte(content)); writeErr != nil {
		return writeErr
	}
	return file.Close()
}
