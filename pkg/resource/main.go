package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-git/go-billy/v5"
	audiov1 "github.com/tkellen/aevitas/pkg/resource/audio/v1"
	domainv1 "github.com/tkellen/aevitas/pkg/resource/domain/v1"
	imagev1 "github.com/tkellen/aevitas/pkg/resource/image/v1"
	metav1 "github.com/tkellen/aevitas/pkg/resource/meta/v1"
	textv1 "github.com/tkellen/aevitas/pkg/resource/text/v1"
	"sort"
	"strings"
)

type Renderable interface {
	Render(context.Context, billy.Filesystem) error
	Current(billy.Filesystem) bool
	Scope(billy.Filesystem) (billy.Filesystem, error)
	Deps(context.Context) ([]string, error)
}

type Resource struct {
	Group    string
	Version  string
	Kind     string
	Meta     metav1.Meta
	Manifest []byte
}

func New(manifest []byte) (*Resource, error) {
	var r Resource
	if err := json.Unmarshal(manifest, &r); err != nil {
		return nil, fmt.Errorf("%w: %s", err, manifest)
	}
	if err := r.Validate(); err != nil {
		return nil, fmt.Errorf("%s\n%w", manifest, err)
	}
	r.Manifest = manifest
	return &r, nil
}

func (r *Resource) Validate() error {
	if r.Kind == "" || r.Group == "" || r.Version == "" {
		return fmt.Errorf("group, version and kind must be set")
	}
	return nil
}

func (r *Resource) String() string {
	return fmt.Sprintf("%s: %s", r.ID(), r.Manifest)
}

func (r *Resource) KGV() string {
	return r.Kind + "/" + r.Group + "/" + r.Version
}

func (r *Resource) ID() string {
	return r.KGV() + "/" + r.Meta.Name
}

func (r *Resource) New() (Renderable, error) {
	kgv := r.KGV()
	switch kgv {
	case "image/jpeg/v1":
		return imagev1.NewJpeg(r.Manifest)
	case "image/png/v1":
		return imagev1.NewPng(r.Manifest)
	case "image/gif/v1":
		return imagev1.NewGif(r.Manifest)
	case "audio/mpeg/v1":
		return audiov1.NewMpeg(r.Manifest)
	case "text/html/v1":
		return textv1.NewHtml(r.Manifest)
	case "domain/blog/v1":
		return domainv1.NewBlog(r.Manifest)
	}
	return nil, fmt.Errorf("%s not registered as renderable", kgv)
}

// List holds an array of resources of unknown type.
type List []*Resource

func (l List) String() string {
	return l.Indexed().String()
}

// Indexed converts a List into an IndexedList.
func (l List) Indexed() IndexedList {
	var shard *IndexShard
	index := IndexedList{}
	for _, resource := range l {
		kgv := resource.KGV()
		if _, ok := index[kgv]; !ok {
			index[kgv] = &IndexShard{
				Resources: List{},
				ById:      map[string]*Resource{},
			}
		}
		shard = index[kgv]
		shard.Resources = append(shard.Resources, resource)
		shard.ById[resource.ID()] = resource
	}
	return index
}

// IndexedList is a map of resources in two forms for fast matching.
type IndexedList map[string]*IndexShard
type IndexShard struct {
	Resources []*Resource
	ById      map[string]*Resource
}

// String returns the count for each unique key/group/version in the list.
func (il IndexedList) String() string {
	var totals []string
	for kgv, shard := range il {
		totals = append(totals, fmt.Sprintf("%s: %d", kgv, len(shard.Resources)))
	}
	return strings.Join(totals, "\n")
}

// Select filters a List only keeping those whose IDs match the provided
// selectors.
func (il IndexedList) Select(selectors ...string) (List, error) {
	matches := List{}
	matchEntireKgv := map[string]struct{}{}
	sort.Strings(selectors)
	for _, selector := range selectors {
		kgv := "unknown"
		parts := strings.Split(selector, "/")
		if len(parts) > 2 {
			kgv = strings.Join(parts[0:3], "/")
		}
		if shard, ok := il[kgv]; ok {
			// If matching entire kgv, append all elements and continue.
			if len(parts) == 3 {
				if _, ok := matchEntireKgv[kgv]; !ok {
					matches = append(matches, shard.Resources...)
					matchEntireKgv[kgv] = struct{}{}
				}
				continue
			}
			// Otherwise, look for a match by ID in the KGV shard.
			if match, ok := shard.ById[selector]; ok {
				matches = append(matches, match)
				continue
			}
		}
		return nil, fmt.Errorf("not present in index: %s", selector)
	}
	return matches, nil
}

func (il IndexedList) Renderables(ctx context.Context, parents List, visited map[string]struct{}) ([]Renderable, error) {
	if visited == nil {
		visited = map[string]struct{}{}
	}
	var result []Renderable
	for _, item := range parents {
		id := item.ID()
		// Increase speed (and prevent infinite recursion on cyclic deps) by
		// remembering each resource that has been visited and skipping if it
		// is seen more than once.
		if _, ok := visited[id]; ok {
			continue
		} else {
			visited[id] = struct{}{}
		}
		// Attempt to instantiate a resource with its concrete type.
		resource, renderableErr := item.New()
		if renderableErr != nil {
			return nil, renderableErr
		}
		result = append(result, resource)
		// Get all dependencies of the one just instantiated.
		deps, depsErr := resource.Deps(ctx)
		if depsErr != nil {
			return nil, depsErr
		}
		// Find resource for each dependency.
		children, selectErr := il.Select(deps...)
		if selectErr != nil {
			return nil, fmt.Errorf("%s: dependency %w", id, selectErr)
		}
		// Recurse through all child dependencies.
		resources, err := il.Renderables(ctx, children, visited)
		if err != nil {
			return nil, err
		}
		result = append(result, resources...)
	}
	return result, nil
}
