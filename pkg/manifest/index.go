package manifest

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/tkellen/aevitas/internal/selector"
	"sort"
	"strings"
	"time"
)

// Index provides fast lookups for finding resources during rendering.
type Index struct {
	content   *index
	relations map[*Manifest]*index
}

// NewIndex does just what you think it does.
func NewIndex() *Index {
	return &Index{
		content: newIndex(),
	}
}

// String returns the count for each shard in the index as a coarse debugging
// guide about what is contained within.
func (i *Index) String() string {
	format := "%-45s%v"
	totals := []string{fmt.Sprintf(format, "INDEX SHARD", "COUNT")}
	var shards []string
	for shard := range i.content.shard {
		shards = append(shards, shard)
	}
	sort.Strings(shards)
	for _, shard := range shards {
		totals = append(totals, fmt.Sprintf(format, shard, len(i.content.shard[shard].manifests)))
	}
	return strings.Join(totals, "\n")
}

// Insert adds a record to the index. Due to limitations in how relationships
// between manifests are currently handled, any insert invalidates the entire
// computed set of relations. In practice, inserting all manifests currently
// happens once and the index is read-only after that. If that changes, this
// will likely require revision.
func (i *Index) Insert(manifests ...*Manifest) error {
	i.relations = nil
	return i.content.insert(manifests...)
}

// FindMany produces an array of manifests whose selectors match the one provided.
func (i *Index) FindMany(target *selector.Selector) ([]*Manifest, error) {
	if target.IsWildcard() {
		shard, shardErr := i.content.shardOf(target)
		if shardErr != nil {
			return nil, shardErr
		}
		// ensure a copy is returned to prevent external mutation (e.g sorting)
		// ugh. i should use rust.
		return append([]*Manifest{}, shard.manifests...), nil
	}
	match, err := i.content.findOne(target, false)
	if err != nil {
		return nil, err
	}
	return []*Manifest{match}, nil
}

// FindMany locates a single manifest based on the selector provided.
func (i *Index) FindOne(target *selector.Selector) (*Manifest, error) {
	return i.content.findOne(target, false)
}

// Next finds the next latest manifest within the target's KGVN.
func (i *Index) Next(target *Manifest) *Manifest {
	shard, shardErr := i.content.shardOf(target.Selector)
	if shardErr != nil {
		return nil
	}
	return shard.next(target)
}

// Prev finds the next earliest manifest within the target's KGVN.
func (i *Index) Prev(target *Manifest) *Manifest {
	shard, shardErr := i.content.shardOf(target.Selector)
	if shardErr != nil {
		return nil
	}
	return shard.previous(target)
}

// SameMonthDay finds manifest within the target's KGVN that were published on
// the same month and day.
func (i *Index) SameMonthDay(target *Manifest) []*Manifest {
	shard, shardErr := i.content.shardOf(target.Selector)
	if shardErr != nil {
		return nil
	}
	return shard.sameMonthDay(target)
}

// RelatedIndex returns a new index which contains only manifests which are
// related to the supplied target.
func (i *Index) RelatedIndex(target *Manifest) (*Index, error) {
	if index, ok := i.relations[target]; ok {
		return &Index{
			content:   index,
			relations: i.relations,
		}, nil
	}
	return nil, fmt.Errorf("unable to find relationships for %s", target)
}

// RelationsHash returns a unique identifier for all relations of the target
// manifest.
func (i *Index) RelationsHash(target *Manifest) string {
	return i.relations[target].hash()
}

func (i *Index) isRelated(target *Manifest, mustRelateTo *selector.Selector) (bool, error) {
	relations, relatedIndexErr := i.RelatedIndex(target)
	if relatedIndexErr != nil {
		return false, relatedIndexErr
	}
	// If mustRelateTo selector is not a wildcard, the target must relate to a
	// single other manifest.
	if !mustRelateTo.IsWildcard() {
		// The match is valid if it is directly related to mustRelateTo.
		_, findOneErr := relations.content.findOne(mustRelateTo, true)
		if findOneErr != nil {
			return false, findOneErr
		}
		return true, nil
	}
	// If the mustRelateTo selector is a wildcard, a match is valid when it has
	// a relationship with _any_ manifest that matches the mustRelateTo shard
	// (aka kind/group/version/namespace)
	_, shardOfErr := relations.content.shardOf(mustRelateTo)
	if shardOfErr != nil {
		return false, shardOfErr
	}
	return true, nil
}

// FindManyWithRelation searches the relationships of a source selector for any
// manifests that are related to the mustRelateTo selector.
func (i *Index) FindManyWithRelation(target *selector.Selector, mustRelateTo *selector.Selector) ([]*Manifest, error) {
	matches, findErr := i.FindMany(target)
	if findErr != nil {
		return nil, findErr
	}
	var validMatches []*Manifest
	for _, match := range matches {
		if ok, _ := i.isRelated(match, mustRelateTo); ok {
			validMatches = append(validMatches, match)
		}
	}
	return validMatches, nil
}

func (i *Index) Collate() error {
	i.relations = map[*Manifest]*index{}
	totalCount := 0
	lastCount := -1
	// Because relationships can be indirect, this repeatedly passes over the
	// index until all relationships are resolved.
	for lastCount != totalCount {
		lastCount = totalCount
		totalCount = 0
		for _, item := range i.content.all.manifests {
			if item.Meta == nil {
				continue
			}
			relations := append(item.Meta.Imports, item.Meta.Relations...)
			for _, child := range item.Meta.Children {
				relations = append(relations, child.Relation)
			}
			var related []*Manifest
			for _, relation := range relations {
				expanded, err := relation.Resolve(i)
				if !relation.Selector.IsWildcard() && err != nil {
					return fmt.Errorf("%s: resolving relations: %w", item, err)
				}
				related = append(related, expanded...)
			}
			totalCount = totalCount + len(related)
			// skip redundant passes
			if m, ok := i.relations[item]; ok {
				if len(m.byID) == len(related) {
					continue
				}
			}
			if err := i.addRelation(item, related...); err != nil {
				return fmt.Errorf("adding relations to %s: %w", item, err)
			}
		}
	}
	i.content.collate()
	for _, index := range i.relations {
		index.collate()
	}
	return nil
}

// addRelation records a relationship from one manifest to another and the
// inverse relationship back.
func (i *Index) addRelation(parent *Manifest, manifests ...*Manifest) error {
	if _, ok := i.relations[parent]; !ok {
		i.relations[parent] = newIndex()
	}
	relations := make([]*Manifest, len(manifests))
	for idx, m := range manifests {
		manifest, err := i.FindOne(m.Selector)
		if err != nil {
			return fmt.Errorf("%s: %w", m, err)
		}
		relations[idx] = manifest
	}
	// Make parent relations to all supplied manifests. Ignore duplicate insertion
	// errors as this is expected.
	_ = i.relations[parent].insert(relations...)
	for _, target := range relations {
		if _, ok := i.relations[target]; !ok {
			i.relations[target] = newIndex()
		}
		// Make all supplied manifests relate back to parent. Ignore duplicate
		// insertion errors as this is expected.
		_ = i.relations[target].insert(parent)
	}
	return nil
}

type index struct {
	all     *shard // all manifests.
	byID    map[string]*Manifest
	notLive map[string]*Manifest
	shard   map[string]*shard // manifests sharded by KGVN
}

func newIndex() *index {
	return &index{
		all:     newShard(),
		byID:    map[string]*Manifest{},
		notLive: map[string]*Manifest{},
		shard:   map[string]*shard{},
	}
}

func (i *index) shardOf(target *selector.Selector) (*shard, error) {
	shardKey := target.KGVN
	shard, exists := i.shard[shardKey]
	if exists {
		return shard, nil
	}
	return nil, fmt.Errorf("%s is empty", shardKey)
}

func (i *index) hash() string {
	var hash strings.Builder
	for _, entry := range i.all.manifests {
		hash.WriteString(entry.Hash)
	}
	return hash.String()
}

func (i *index) collate() {
	i.all.collate()
	for _, shard := range i.shard {
		shard.collate()
	}
}

var notFound = errors.New("resource not found")

func (i *index) findOne(target *selector.Selector, fastError bool) (*Manifest, error) {
	id := target.ID()
	manifest, found := i.byID[id]
	if !found {
		if fastError {
			// this is in a hot path to validate relationships. when used in
			// this manner errors are ignored and the cost of producing them can
			// be eliminated.
			return nil, notFound
		}
		if m, notLive := i.notLive[id]; notLive {
			return nil, fmt.Errorf("%s: must be \"live\" to be used", m)
		}
		return nil, fmt.Errorf("%w: %s", notFound, target.ID())
	}
	return manifest, nil
}

// insert adds N manifests that are currently marked as live and, if there is a
// publication date, that the date is older than the time this is run. If a
// manifest of the same ID has been previously inserted, trigger an error. This
// error is for detecting duplicates during initial index creation.
func (i *index) insert(manifests ...*Manifest) error {
	var collisions bytes.Buffer
	for _, m := range manifests {
		id := m.Selector.ID()
		// skip unpublished resources (save for helpful error messages though).
		if !m.IsLive() {
			i.notLive[id] = m
			continue
		}
		// skip repeated inserts (but collect errors).
		if _, ok := i.byID[id]; ok {
			collisions.WriteString(fmt.Sprintf("%s\n", m.Selector))
			continue
		}
		i.byID[id] = m
		i.all.insert(m)
		// shard index by kind group version namespace.
		shardKey := m.Selector.KGVN
		shard, ok := i.shard[shardKey]
		if !ok {
			i.shard[shardKey] = newShard()
			shard = i.shard[shardKey]
		}
		shard.insert(m)
	}
	// If there were any collisions, enumerate them all in the returned error.
	if collisions.Len() > 0 {
		return fmt.Errorf("collisions:\n%s", collisions.String())
	}
	return nil
}

type manifestList []*Manifest

func (l manifestList) Len() int           { return len(l) }
func (l manifestList) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }
func (l manifestList) Less(i, j int) bool { return l[i].Less(l[j]) }

// shard holds a collection of manifests indexed to support common operations.
type shard struct {
	manifests []*Manifest
	collated  bool
	before    map[*Manifest]*Manifest
	after     map[*Manifest]*Manifest
	sameTime  map[time.Time][]*Manifest
}

// newShard does just what you think it does.
func newShard() *shard {
	return &shard{
		manifests: []*Manifest{},
	}
}

func (l *shard) collate() {
	sort.Sort(manifestList(l.manifests))
	l.before = map[*Manifest]*Manifest{}
	l.after = map[*Manifest]*Manifest{}
	l.sameTime = map[time.Time][]*Manifest{}
	count := len(l.manifests)
	if count > 1 {
		for idx, manifest := range l.manifests {
			if idx != 0 {
				l.before[manifest] = l.manifests[idx-1]
			}
			if idx+1 < count {
				l.after[manifest] = l.manifests[idx+1]
			}
			timeKey := manifest.PublishMonthDay()
			l.sameTime[timeKey] = append(l.sameTime[timeKey], manifest)
		}
	}
	l.collated = true
}

func (l *shard) next(compare *Manifest) *Manifest {
	return l.after[compare]
}

func (l *shard) previous(compare *Manifest) *Manifest {
	return l.before[compare]
}

func (l *shard) sameMonthDay(compare *Manifest) []*Manifest {
	return l.sameTime[compare.PublishMonthDay()]
}

func (l *shard) insert(manifests ...*Manifest) {
	l.collated = false
	for _, manifest := range manifests {
		l.manifests = append(l.manifests, manifest)
	}
}
