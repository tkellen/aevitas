package website

import (
	"fmt"
	json "github.com/json-iterator/go"
	"github.com/tkellen/aevitas/pkg/resource"
)

const KGVTaxonomy = "website/taxonomy/v1"

type Taxonomy struct {
	resource *resource.Resource
	Spec     *TaxonomySpec
}

type TaxonomySpec struct {
	Title         string
	TitleFragment string
	Path          string
	Plural        string
	Singular      string
	Description   string
}

func NewTaxonomy(r *resource.Resource) (*Taxonomy, error) {
	instance := &Taxonomy{resource: r}
	if err := json.Unmarshal(r.Spec, &instance.Spec); err != nil {
		return nil, err
	}
	if err := instance.Validate(); err != nil {
		return nil, fmt.Errorf("%s\n%w", r.Spec, err)
	}
	return instance, nil
}

func (t *Taxonomy) Validate() error { return nil }

