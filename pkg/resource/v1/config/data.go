package config

import (
	json "github.com/json-iterator/go"
	"github.com/tkellen/aevitas/pkg/manifest"
)

const KGVData = "config/data/v1"

type Data struct {
	*manifest.Manifest
	Spec *DataSpec
}

type DataSpec map[string]interface{}

func NewData(m *manifest.Manifest) (*Data, error) {
	instance := &Data{
		Manifest: m,
		Spec:     &DataSpec{},
	}
	if err := json.Unmarshal(m.Spec, instance.Spec); err != nil {
		return nil, err
	}
	return instance, nil
}
