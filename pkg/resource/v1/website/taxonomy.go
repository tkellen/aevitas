package website

import (
	"fmt"
	json "github.com/json-iterator/go"
	"github.com/tkellen/aevitas/pkg/manifest"
)

const KGVTaxonomy = "website/taxonomy/v1"

type Taxonomy struct {
	*manifest.Manifest
	Spec *TaxonomySpec
}

type TaxonomySpec struct {
	Path          string
	Title         string
	TitleFragment string
	Description   string
	Body          string
}

func NewTaxonomy(m *manifest.Manifest) (*Taxonomy, error) {
	instance := &Taxonomy{
		Manifest: m,
		Spec:     &TaxonomySpec{},
	}
	if err := json.Unmarshal(m.Spec, instance.Spec); err != nil {
		return nil, err
	}
	if err := instance.Validate(); err != nil {
		return nil, err
	}
	return instance, nil
}

func (t *Taxonomy) Validate() error {
	if t.Spec.Path == "" {
		return fmt.Errorf("path must be defined")
	}
	if t.Spec.Title == "" {
		return fmt.Errorf("title must be defined")
	}
	if t.Spec.Description == "" {
		return fmt.Errorf("body must be defined")
	}
	return nil
}