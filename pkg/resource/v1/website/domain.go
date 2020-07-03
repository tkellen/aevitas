package website

import (
	"fmt"
	json "github.com/json-iterator/go"
	"github.com/tkellen/aevitas/pkg/manifest"
)

const KGVDomain = "website/domain/v1"

type Domain struct {
	*manifest.Manifest
	Spec *DomainSpec
}

type DomainSpec struct {
	Title       string
	Description string
	Author      string
	Pages       []string
	Host        string
}

type SectionSpec struct {
	Name string
	Slug string
}

func NewDomain(m *manifest.Manifest) (*Domain, error) {
	instance := &Domain{
		Manifest: m,
		Spec:     &DomainSpec{},
	}
	if err := json.Unmarshal(m.Spec, instance.Spec); err != nil {
		return nil, err
	}
	if err := instance.Validate(); err != nil {
		return nil, err
	}
	return instance, nil
}

func (d *Domain) Validate() error {
	if d.Spec.Title == "" {
		return fmt.Errorf("title must be defined")
	}
	if d.Spec.Description == "" {
		return fmt.Errorf("description must be defined")
	}
	if d.Spec.Author == "" {
		return fmt.Errorf("author must be defined")
	}
	return nil
}
