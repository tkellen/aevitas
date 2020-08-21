package resource

import (
	"fmt"
	"github.com/go-git/go-billy/v5"
	"github.com/tkellen/aevitas/internal/selector"
	"github.com/tkellen/aevitas/pkg/manifest"
	assetv1 "github.com/tkellen/aevitas/pkg/resource/v1/asset"
	"strings"
)

// Handler provides support for instantiating resources of any type. When golang
// supports generics this will likely go away.
type Factory struct {
	handlers      []*Handler
	defaultSource billy.Filesystem
	defaultDest   billy.Filesystem
}

// Handler represents a method of instantiating a specific resource type.
type Handler struct {
	selector *selector.Selector
	new      func(m *manifest.Manifest) (interface{}, error)
	source   billy.Filesystem
	dest     billy.Filesystem
}

func (h *Handler) New(m *manifest.Manifest) (interface{}, error) {
	return h.new(m)
}

// NewFactory creates a registry
func NewFactory(defaultSource billy.Filesystem, defaultDest billy.Filesystem) *Factory {
	return &Factory{
		defaultSource: defaultSource,
		defaultDest:   defaultDest,
	}
}

func (r *Factory) String() string {
	var details []string
	for _, h := range r.handlers {
		details = append(details, fmt.Sprintf("%s", h.selector))
	}
	return strings.Join(details, "\n")
}

func (r *Factory) Register(target string, fn func(m *manifest.Manifest) (interface{}, error)) error {
	s, err := selector.New(target)
	if err != nil {
		return err
	}
	r.handlers = append(r.handlers, &Handler{
		selector: s,
		// expose per-selector source customization?
		source: r.defaultSource,
		dest:   r.defaultDest,
		new:    fn,
	})
	return nil
}

func (r *Factory) Handler(target *manifest.Manifest) (*Handler, error) {
	var factory *Handler
	for _, h := range r.handlers {
		if h.selector.KGV == target.Selector.KGV {
			factory = h
		}
	}
	if factory == nil {
		return nil, fmt.Errorf("%s: no registered factory", target.Selector)
	}
	return factory, nil
}

func DefaultFactory(
	source billy.Filesystem,
	dest billy.Filesystem,
) *Factory {
	factory := NewFactory(source, dest)
	factory.Register(fmt.Sprintf("%s/*/*", "html/template/v1"), func(m *manifest.Manifest) (interface{}, error) {
		return m, nil
	})
	factory.Register(fmt.Sprintf("%s/*/*", "website/content/v1"), func(m *manifest.Manifest) (interface{}, error) {
		return m, nil
	})
	factory.Register(fmt.Sprintf("%s/*/*", assetv1.KGVGif), func(m *manifest.Manifest) (interface{}, error) {
		return assetv1.NewGif(m)
	})
	factory.Register(fmt.Sprintf("%s/*/*", assetv1.KGVJpeg), func(m *manifest.Manifest) (interface{}, error) {
		return assetv1.NewJpeg(m)
	})
	factory.Register(fmt.Sprintf("%s/*/*", assetv1.KGVPng), func(m *manifest.Manifest) (interface{}, error) {
		return assetv1.NewPng(m)
	})
	factory.Register(fmt.Sprintf("%s/*/*", assetv1.KGVMpeg), func(m *manifest.Manifest) (interface{}, error) {
		return assetv1.NewMpeg(m)
	})
	return factory
}
