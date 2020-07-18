package html

import (
	json "github.com/json-iterator/go"
	"github.com/tkellen/aevitas/pkg/manifest"
)

const KGVTemplate = "html/template/v1"

type Template struct {
	*manifest.Manifest
	Spec *TemplateSpec
}
type TemplateSpec struct {
	Body string
}

func NewTemplate(m *manifest.Manifest) (*Template, error) {
	instance := &Template{
		Manifest: m,
		Spec:     &TemplateSpec{},
	}
	if err := json.Unmarshal(m.Spec, instance.Spec); err != nil {
		return nil, err
	}
	if err := instance.Validate(); err != nil {
		return nil, err
	}
	return instance, nil
}
func (f *Template) Validate() error { return nil }
func (f *Template) Body() string { return f.Spec.Body }
