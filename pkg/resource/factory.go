package resource

import (
	"fmt"
	"github.com/go-git/go-billy/v5"
	"github.com/tkellen/aevitas/pkg/manifest"
	"strings"
)

// Handler provides support for instantiating resources of any type. When golang
// supports generics this will likely go away.
type Factory struct {
	Handlers      []*Handler
	DefaultSource billy.Filesystem
	DefaultDest   billy.Filesystem
}

// Handler represents a method of instantiating a specific resource type.
type Handler struct {
	Selector manifest.Selector
	New      func(m *manifest.Manifest) (interface{}, error)
	Source   billy.Filesystem
	Dest     billy.Filesystem
}

// NewFactory creates a registry
func NewFactory(defaultSource billy.Filesystem, defaultDest billy.Filesystem) *Factory {
	return &Factory{
		DefaultSource: defaultSource,
		DefaultDest:   defaultDest,
	}
}

func (r *Factory) String() string {
	var details []string
	for _, h := range r.Handlers {
		details = append(details, fmt.Sprintf("%s", h.Selector))
	}
	return strings.Join(details, "\n")
}

func (r *Factory) Register(target string, fn func(m *manifest.Manifest) (interface{}, error)) error {
	s, err := manifest.NewSelector(target)
	if err != nil {
		return err
	}
	r.Handlers = append(r.Handlers, &Handler{
		Selector: s,
		// expose per-selector source customization?
		Source: r.DefaultSource,
		Dest:   r.DefaultDest,
		New:    fn,
	})
	return nil
}

func (r *Factory) Handler(target *manifest.Manifest) (*Handler, error) {
	var factory *Handler
	for _, h := range r.Handlers {
		if h.Selector.KGV() == target.Selector.KGV() {
			factory = h
		}
	}
	if factory == nil {
		return nil, fmt.Errorf("%s: no registered factory", target)
	}
	return factory, nil
}
