package resource

import (
	"context"
	"fmt"
	"github.com/go-git/go-billy/v5"
	"github.com/tkellen/aevitas/pkg/manifest"
)

// Asset represents a instance that can be rendered.
type Asset interface {
	Render(context.Context, billy.Filesystem, billy.Filesystem) error
}

type Instance struct {
	Self    interface{}
	AsAsset Asset
	Source  billy.Filesystem
	Dest    billy.Filesystem
}

func newInstance(factory *Factory, m *manifest.Manifest) (*Instance, error) {
	handler, handlerErr := factory.Handler(m)
	if handlerErr != nil {
		return nil, handlerErr
	}
	instantiated, newErr := handler.New(m)
	if newErr != nil {
		return nil, fmt.Errorf("instantiating: %w", newErr)
	}
	asset, _ := instantiated.(Asset)
	return &Instance{
		Self:    instantiated,
		AsAsset: asset,
		Source:  handler.source,
		Dest:    handler.dest,
	}, nil
}
