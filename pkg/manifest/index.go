package manifest

import (
	"fmt"
	"sort"
	"strings"
)

// List holds an array of manifests.
type List struct {
	Manifests []*Manifest
	ByID      map[string]*Manifest
}

// NewList does just what you think it does.
func NewList() *List {
	return &List{
		Manifests: []*Manifest{},
		ByID:      map[string]*Manifest{},
	}
}

// Insert adds N manifests to a provided list failing if a manifest of the same
// ID has been previously inserted.
func (l *List) Insert(manifests ...*Manifest) error {
	for _, manifest := range manifests {
		id := manifest.Selector.ID()
		if _, ok := l.ByID[id]; ok {
			return fmt.Errorf("%s: duplicate entry", manifest.Selector)
		}
		l.ByID[id] = manifest
		l.Manifests = append(l.Manifests, manifest)
	}
	return nil
}

// Indexed creates an index of manifests aimed at supporting fast lookups during
// rendering.
func (l *List) Indexed() (*Index, error) {
	return NewIndex(l.Manifests)
}

// Index provides fast lookups for finding resources during rendering.
type Index struct {
	All   *List            // All manifests.
	Shard map[string]*List // Manifests sharded by KGVN
}

// Index creates an sharded set of manifests aimed at supporting fast lookups
// during rendering.
func NewIndex(manifests []*Manifest) (*Index, error) {
	index := &Index{
		All:   NewList(),
		Shard: map[string]*List{},
	}
	if err := index.Insert(manifests...); err != nil {
		return nil, err
	}
	return index, nil
}

// String returns the count for each shard.
func (il Index) String() string {
	format := "%-45s%v"
	totals := []string{fmt.Sprintf(format, "INDEX SHARD", "COUNT")}
	var shards []string
	for shard := range il.Shard {
		shards = append(shards, shard)
	}
	sort.Strings(shards)
	for _, shard := range shards {
		totals = append(totals, fmt.Sprintf(format, shard, len(il.Shard[shard].Manifests)))
	}
	return strings.Join(totals, "\n")
}

// Insert adds N manifests to the index.
func (il *Index) Insert(manifests ...*Manifest) error {
	for _, manifest := range manifests {
		if err := il.All.Insert(manifest); err != nil {
			return err
		}
		// Shard index by kind group version namespace
		shardKey := manifest.Selector.KGVN()
		shard, ok := il.Shard[shardKey]
		if !ok {
			il.Shard[shardKey] = NewList()
			shard = il.Shard[shardKey]
		}
		if err := shard.Insert(manifest); err != nil {
			return err
		}
	}
	return nil
}

// Get finds a single resource using a string target.
func (il *Index) Get(target string) (*Manifest, error) {
	s, selectorErr := NewSelector(target)
	if selectorErr != nil {
		return nil, selectorErr
	}
	return il.GetSelector(&s)
}

// GetSelector finds a single resource using a selector target.
func (il *Index) GetSelector(target *Selector) (*Manifest, error) {
	shardKey := target.KGVN()
	id := target.ID()
	shard, ok := il.Shard[shardKey]
	if !ok {
		return nil, fmt.Errorf("no manifests in shard %s", shardKey)
	}
	resource, found := shard.ByID[id]
	if !found {
		return nil, fmt.Errorf("%s not found\n%s", id, il)
	}
	return resource, nil
}

// Find produces an array of manifests whose selectors match the one provided.
func (il *Index) Find(target *Selector) ([]*Manifest, error) {
	shard, exists := il.Shard[target.KGVN()]
	if exists {
		// If a selector targets an entire shard, append all of its manifests
		// without iterating them individually.
		if target.NameIsWildcard() {
			return shard.Manifests, nil
		}
		// Otherwise, look for a match by ID.
		if match, ok := shard.ByID[target.ID()]; ok {
			return []*Manifest{match}, nil
		}
	}
	return nil, fmt.Errorf("%s: not present\n%s", target, il)
}

// Traverse recursively produces an array of manifests required to render a
// supplied manifest.
func (il *Index) Traverse(manifest *Manifest) ([]*Manifest, error) {
	return il.traverse([]*Manifest{manifest}, nil)
}

func (il *Index) traverse(parents []*Manifest, visited map[string]struct{}) ([]*Manifest, error) {
	var result []*Manifest
	if visited == nil {
		visited = map[string]struct{}{}
	}
	for _, manifest := range parents {
		id := manifest.Selector.ID()
		// Increase speed (and prevent infinite recursion on cyclic deps) by
		// remembering each manifest that has been visited and skipping if it
		// is seen more than once.
		if _, ok := visited[id]; ok {
			continue
		} else {
			visited[id] = struct{}{}
		}
		// Save parent manifest in the results.
		result = append(result, manifest)
		// Get all selectors to other manifests required to render this.
		var deps []*Manifest
		for _, selector := range manifest.Required() {
			matches, findErr := il.Find(selector)
			if findErr != nil {
				return nil, findErr
			}
			deps = append(deps, matches...)
		}
		// Recurse through all child dependencies.
		children, err := il.traverse(deps, visited)
		if err != nil {
			return nil, err
		}
		result = append(result, children...)
	}
	return result, nil
}
