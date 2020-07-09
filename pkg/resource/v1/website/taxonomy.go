package website

import (
	"context"
	"fmt"
	json "github.com/json-iterator/go"
	"github.com/tkellen/aevitas/pkg/manifest"
	"github.com/tkellen/aevitas/pkg/resource"
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
		return fmt.Errorf("description must be defined")
	}
	return nil
}
func (t *Taxonomy) Content() string { return t.Spec.Description }
func (t *Taxonomy) Render(_ context.Context, r *resource.Resource) error {
	if stat, _ := r.Dest.Stat(t.Spec.Path); stat != nil && stat.Size() != 0 {
		return nil
	}
	content, err := r.Body()
	if err != nil {
		return err
	}
	file, createErr := r.Dest.Create(t.Spec.Path)
	if createErr != nil {
		return createErr
	}
	if _, writeErr := file.Write([]byte(content)); writeErr != nil {
		return writeErr
	}
	return file.Close()
}
