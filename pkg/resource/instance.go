package resource

import (
	"github.com/go-git/go-billy/v5"
	"github.com/tkellen/aevitas/pkg/manifest"
)

type instance struct {
	self    interface{}
	asAsset asset
	source  billy.Filesystem
	dest    billy.Filesystem
}

func newInstance(factory *Factory, m *manifest.Manifest) (*instance, error) {
	handler, handlerErr := factory.Handler(m)
	if handlerErr != nil {
		return nil, handlerErr
	}
	instantiated, newErr := handler.New(m)
	if newErr != nil {
		return nil, newErr
	}
	asset, _ := instantiated.(asset)
	return &instance{
		self:    instantiated,
		asAsset: asset,
		source:  handler.source,
		dest:    handler.dest,
	}, nil
}
