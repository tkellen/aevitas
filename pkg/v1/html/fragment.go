package html

import (
	"context"
	"fmt"
	json "github.com/json-iterator/go"
	"github.com/tkellen/aevitas/pkg/resource"
	"html/template"
)

const KGVFragment = "html/fragment/v1"

type Fragment struct {
	resource *resource.Resource
	template *template.Template
	Spec     *FragmentSpec
}

type FragmentSpec struct {
	Body  string
}

func NewFragment(r *resource.Resource) (*Fragment, error) {
	instance := &Fragment{resource: r}
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

func (f *Fragment) Validate() error { return nil }

func (f *Fragment) Template() *template.Template { return f.template }

func (f *Fragment) Render(_ context.Context, _ resource.Element) error { return nil }