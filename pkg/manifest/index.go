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
)

// AllFromReader creates an array of manifests from a provided reader taking the
// assumption that the reader contains one manifest per line.
func AllFromReader(input io.Reader) ([]*Manifest, error) {
	reader := bufio.NewReader(input)
	queue := make(chan *Manifest)
	process := errgroup.Group{}
	for {
		raw, err := reader.ReadBytes('\n')
		if errors.Is(err, io.EOF) {
			break
		}
		if len(bytes.TrimSpace(raw)) == 0 {
			continue
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
	collector := errgroup.Group{}
	var manifests []*Manifest
	collector.Go(func() error {
		for manifest := range queue {
			manifests = append(manifests, manifest)
		}
		return nil
	})
	if err := process.Wait(); err != nil {
		return nil, err
	}
	close(queue)
	collector.Wait()
	return manifests, nil
}

// AllFromReader creates an array of manifests from a provided directory by
// traversing every file in every directory from a specified parent.
func AllFromDirectory(dir string) ([]*Manifest, error) {
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
	collector := errgroup.Group{}
	var manifests []*Manifest
	collector.Go(func() error {
		for manifest := range queue {
			manifests = append(manifests, manifest)
		}
		return nil
	})
	if err := process.Wait(); err != nil {
		return nil, err
	}
	close(queue)
	collector.Wait()
	return manifests, nil
}

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

// Indexed produces a sharded representation of all items in the list aimed at
// making searches fast.
func (l *List) Indexed(withRelations bool) (*Index, error) {
	return NewIndex(l.Manifests, withRelations)
}

// Index provides fast lookups for finding resources during rendering.
type Index struct {
	// Manifests sharded by KGVN
	Shards    map[string]*List
	Relations Relations
}

// String returns the count for each shard.
func (il Index) String() string {
	format := "%-45s%v"
	totals := []string{fmt.Sprintf(format, "INDEX SHARD", "COUNT")}
	var shards []string
	for shard := range il.Shards {
		shards = append(shards, shard)
	}
	sort.Strings(shards)
	for _, shard := range shards {
		totals = append(totals, fmt.Sprintf(format, shard, len(il.Shards[shard].Manifests)))
	}
	return strings.Join(totals, "\n")
}

// Index creates an indexed listing aimed at supporting fast lookups during
// rendering.
func NewIndex(list []*Manifest, withRelations bool) (*Index, error) {
	index := &Index{
		Shards: map[string]*List{},
	}
	for _, manifest := range list {
		// Shard index by kind group version namespace
		shardKey := manifest.Selector.KGVN()
		shard, ok := index.Shards[shardKey]
		if !ok {
			index.Shards[shardKey] = NewList()
			shard = index.Shards[shardKey]
		}
		if err := shard.Insert(manifest); err != nil {
			return nil, err
		}
	}
	// If requested, record each manifest relationship on both sides regardless
	// of which side it was recorded on.
	if withRelations {
		relations := Relations{}
		for _, manifest := range list {
			relations[manifest] = NewList()
		}
		for _, manifest := range list {
			for _, relatedSelector := range manifest.Meta.Related {
				related, err := index.Find(relatedSelector)
				if err != nil {
					return nil, err
				}
				// Ignore duplicate insertion errors, this is allowed.
				_ = relations[manifest].Insert(related...)
			}
			if err := manifest.EachInclude(index, func(include *Include) error {
				target, err := index.GetSelector(include.Resource)
				if err != nil {
					return err
				}
				// Ensure relationships are visible from both sides.
				// Ignore duplicate insertion errors.
				_ = relations[manifest].Insert(target)
				_ = relations[target].Insert(manifest)
				return nil
			}); err != nil {
				return nil, err
			}
		}
		index.Relations = relations
	}
	return index, nil
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
	shard, ok := il.Shards[shardKey]
	if !ok {
		return nil, fmt.Errorf("no manifests in shard %s", shardKey)
	}
	resource, found := shard.ByID[id]
	if !found {
		return nil, fmt.Errorf("%s not found\n%s", id, il)
	}
	return resource, nil
}

// Find produces a List that contains manifests whose IDs match the provided
// selectors.
func (il *Index) Find(target *Selector) ([]*Manifest, error) {
	shard, exists := il.Shards[target.KGVN()]
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

// Relations describes all manifests that are related to a given manifest.
type Relations map[*Manifest]*List

func (r Relations) String() string {
	format := "%-45s%v"
	output := []string{fmt.Sprintf(format, "MANIFEST", "RELATED")}
	for item, relations := range r {
		var related []string
		for _, item := range relations.Manifests {
			related = append(related, item.Selector.ID())
		}
		sort.Strings(related)
		output = append(output, fmt.Sprintf(format, item.Selector.ID(), strings.Join(related, ", ")))
	}
	sort.Strings(output)
	return strings.Join(output, "\n")
}
