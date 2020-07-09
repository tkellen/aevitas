package manifest

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// List holds an array of manifests.
type List []*Manifest

// NewListFromReader creates a List from a provided reader taking the assumption
// that the reader contains one manifest per line.
func NewListFromReader(input io.Reader) (List, error) {
	reader := bufio.NewReader(input)
	queue := make(chan *Manifest)
	process := errgroup.Group{}
	for {
		raw, err := reader.ReadBytes('\n')
		if errors.Is(err, io.EOF) {
			break
		}
		process.Go(func() error {
			manifest, err := New(bytes.TrimRight(raw, "\n"))
			if err != nil {
				return fmt.Errorf("%s: %w", raw, err)
			}
			queue <- manifest
			return nil
		})
	}
	collector := sync.WaitGroup{}
	collector.Add(1)
	manifests := List{}
	go func() {
		defer collector.Done()
		for manifest := range queue {
			manifests = append(manifests, manifest)
		}
	}()
	if err := process.Wait(); err != nil {
		return List{}, err
	}
	close(queue)
	collector.Wait()
	return manifests, nil
}

func NewListFromDirectory(dir string) (List, error) {
	queue := make(chan *Manifest)
	process, processCtx := errgroup.WithContext(context.Background())
	sem := semaphore.NewWeighted(10)
	process.Go(func() error {
		return filepath.Walk(dir, func(path string, f os.FileInfo, err error) error {
			if f.IsDir() {
				return nil
			}
			if err := sem.Acquire(processCtx, 1); err != nil {
				return err
			}
			process.Go(func() error {
				defer sem.Release(1)
				manifest, err := NewFromFile(path)
				if err != nil {
					return err
				}
				queue <- manifest
				return nil

			})
			return nil
		})
	})
	collector := sync.WaitGroup{}
	collector.Add(1)
	manifests := List{}
	go func() {
		defer collector.Done()
		for manifest := range queue {
			manifests = append(manifests, manifest)
		}
	}()
	if err := process.Wait(); err != nil {
		return List{}, err
	}
	close(queue)
	collector.Wait()
	return manifests, nil
}

// String produces a human readable representation of what a List contains.
func (l List) String() string {
	return l.Indexed().String()
}

// Index converts a List into an Index.
func (l List) Indexed() *Index {
	var shard *IndexShard
	index := Index{}
	for _, manifest := range l {
		nkgv := manifest.NKGV()
		if _, ok := index[nkgv]; !ok {
			index[nkgv] = &IndexShard{
				Manifests: &List{},
				ById:      map[string]*Manifest{},
			}
		}
		shard = index[nkgv]
		*shard.Manifests = append(*shard.Manifests, manifest)
		shard.ById[manifest.ID()] = manifest
	}
	return &index
}

func (l List) Relations(index *Index) (*Relations, error) {
	relations := make(Relations, len(l))
	// Instantiate a list to hold relations for every manifest.
	for _, manifest := range l {
		relations[manifest] = &List{}
	}
	// Record each manifest relationship on both sides regardless of which side
	// it was actually recorded on.
	for _, source := range l {
		targets, err := source.Related(index)
		if err != nil {
			return nil, err
		}
		if len(targets) > 0 {
			*relations[source] = append(*relations[source], targets...)
			for _, target := range targets {
				if _, ok := relations[target]; ok {
					*relations[target] = append(*relations[target], source)
				} else {
					fmt.Fprintf(os.Stdout, "missing target %s\n", target)
				}

			}
		}
	}
	return &relations, nil
}

// Index is a map of manifests keyed by NKGV and ID for fast lookups.
type Index map[string]*IndexShard

// Relations is a map of Lists keyed by manifest that record relationships
// between manifests.
type Relations map[*Manifest]*List

type IndexShard struct {
	Manifests *List
	ById      map[string]*Manifest
}

// String returns the count for each unique key/group/version in an Index.
func (il Index) String() string {
	format := "%-45s%v"
	totals := []string{fmt.Sprintf(format, "INDEX SHARD", "COUNT")}
	var shards []string
	for shard := range il {
		shards = append(shards, shard)
	}
	sort.Strings(shards)
	for _, shard := range shards {
		totals = append(totals, fmt.Sprintf(format, shard, len(*il[shard].Manifests)))
	}
	return strings.Join(totals, "\n")
}

// Get finds a single resource using a string target.
func (il *Index) Get(target string) (*Manifest, error) {
	s, selectorErr := NewSelector(target)
	if selectorErr != nil {
		return nil, selectorErr
	}
	return il.GetSelector(s)
}

// GetSelector finds a single resource using a selector.
func (il *Index) GetSelector(target *Selector) (*Manifest, error) {
	nkgv := target.NKGV()
	id := target.ID()
	shard, ok := (*il)[nkgv]
	if !ok {
		return nil, fmt.Errorf("no manifests in shard %s", target)
	}
	resource, found := shard.ById[id]
	if !found {
		return nil, fmt.Errorf("%s not found\n%s", id, il)
	}
	return resource, nil
}

// Find produces a List that contains manifests whose IDs match the provided
// selectors.
func (il *Index) Find(selectors SelectorList) (List, error) {
	matches := List{}
	// Save references to KGVs that have been entirely collected so they aren't
	// collected more than once.
	collectEntireKGV := map[string]struct{}{}
	// Sorting ensures that wildcard NKGV selectors appear first.
	sort.Sort(selectors)
	for _, s := range selectors {
		entireKgv := s.IsWildcard()
		nkgv := s.NKGV()
		if shard, ok := (*il)[nkgv]; ok {
			// If a selector targets an entire NKGV, append all of its manifests
			// without iterating them individually.
			if entireKgv {
				if _, ok := collectEntireKGV[nkgv]; !ok {
					matches = append(matches, *shard.Manifests...)
					collectEntireKGV[nkgv] = struct{}{}
				}
				continue
			}
			// If selector does not target entire NKGV, but this NKGV has been
			// entirely selected already, skip this.
			if _, ok := collectEntireKGV[nkgv]; ok {
				continue
			}
			// Otherwise, look for a match by ID in the NKGV shard.
			if match, ok := shard.ById[s.ID()]; ok {
				matches = append(matches, match)
				continue
			}
		}
		return nil, fmt.Errorf("not present in index: %s\n%s", s, il)
	}
	return matches, nil
}

// traverse recursively produces a full list of manifests imports for a supplied
// array of parents.
func (il *Index) traverse(parents List, visited map[string]struct{}) (List, error) {
	var result List
	if visited == nil {
		visited = map[string]struct{}{}
	}
	for _, item := range parents {
		id := item.ID()
		// Increase speed (and prevent infinite recursion on cyclic deps) by
		// remembering each manifest that has been visited and skipping if it
		// is seen more than once.
		if _, ok := visited[id]; ok {
			continue
		} else {
			visited[id] = struct{}{}
		}
		// Save parent manifest in the results.
		result = append(result, item)
		// Find manifest for each dependency and related resource.
		children, relateErr := item.Related(il)
		if relateErr != nil {
			return nil, relateErr
		}
		// Recurse through all child dependencies.
		deps, err := il.traverse(children, visited)
		if err != nil {
			return nil, err
		}
		result = append(result, deps...)
	}
	return result, nil
}
