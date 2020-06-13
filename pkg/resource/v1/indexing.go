package resource

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

// List holds an array of resources.
type List []*Resource

// NewListFromReader creates a list from a provided reader taking the assumption
// that the  reader contains one resource per line.
func NewListFromReader(input io.Reader) (List, error) {
	resources := List{}
	reader := bufio.NewReader(input)
	for {
		manifest, err := reader.ReadBytes('\n')
		if errors.Is(err, io.EOF) {
			break
		}
		resource, err := New(bytes.TrimRight(manifest, "\n"))
		if err != nil {
			return nil, err
		}
		resources = append(resources, resource)
	}
	return resources, nil
}

// String produces a human readable representation of what the list contains.
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

// Lookup finds a single resource using a string target.
func (il IndexedList) Lookup(target string) (*Resource, error) {
	selector, selectorErr := NewSelector(target)
	if selectorErr != nil {
		return nil, selectorErr
	}
	return il.LookupSelector(selector)
}

// LookupSelector finds a single resource using a selector.
func (il IndexedList) LookupSelector(target Selector) (*Resource, error) {
	kgv := target.KGV()
	id := target.ID()
	shard, ok := il[kgv]
	if !ok {
		return nil, fmt.Errorf("no resources in shard %s", target)
	}
	resource, found := shard.ById[id]
	if !found {
		return nil, fmt.Errorf("%s not found", id)
	}
	return resource, nil
}

// Resolve recursively produces a full list of resource imports for a supplied
// array of parents.
func (il IndexedList) Resolve(parents List, visited map[string]struct{}) (List, error) {
	if visited == nil {
		visited = map[string]struct{}{}
	}
	var result List
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
		// Save parent resource in the results.
		result = append(result, item)
		// Find resource for each dependency.
		children, selectErr := il.Select(item.Deps())
		if selectErr != nil {
			return nil, fmt.Errorf("%s: dependency %w", id, selectErr)
		}
		// Recurse through all child dependencies.
		renderables, err := il.Resolve(children, visited)
		if err != nil {
			return nil, err
		}
		result = append(result, renderables...)
	}
	return result, nil
}

// Select produces a List that contains resources whose IDs match the provided
// selectors.
func (il IndexedList) Select(selectors SelectorList) (List, error) {
	matches := List{}
	// Save a list of KGVs that have been exactly matched so the matching
	// process can be short circuited if a given set of selectors contains a
	// fully specified selector and a wildcard selector for a given KGV.
	matchEntireKgv := map[string]struct{}{}
	// Sorting ensures that wildcard KGV selectors appear first.
	sort.Sort(selectors)
	for _, selector := range selectors {
		entireKgv := selector.IsWildcard()
		kgv := selector.KGV()
		if shard, ok := il[kgv]; ok {
			if entireKgv {
				if _, ok := matchEntireKgv[kgv]; !ok {
					matches = append(matches, shard.Resources...)
					matchEntireKgv[kgv] = struct{}{}
				}
				continue
			}
			// Otherwise, look for a match by ID in the KGV shard.
			if match, ok := shard.ById[selector.ID()]; ok {
				matches = append(matches, match)
				continue
			}
		}
		return nil, fmt.Errorf("not present in index: %s", selector)
	}
	return matches, nil
}