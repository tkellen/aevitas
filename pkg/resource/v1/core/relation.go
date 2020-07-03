package core

import (
	json "github.com/json-iterator/go"
	"github.com/tkellen/aevitas/pkg/manifest"
)

const KGVRelation = "core/Relation/v1"

type Relation struct {
	*manifest.Manifest
	Spec *RelationSpec
}
type RelationSpec struct {
	Body string
}

func NewRelation(m *manifest.Manifest) (*Relation, error) {
	instance := &Relation{
		Manifest: m,
		Spec:     &RelationSpec{},
	}
	if err := json.Unmarshal(m.Spec, instance.Spec); err != nil {
		return nil, err
	}
	if err := instance.Validate(); err != nil {
		return nil, err
	}
	return instance, nil
}
func (f *Relation) Validate() error { return nil }
func (f *Relation) Content() string { return f.Spec.Body }
