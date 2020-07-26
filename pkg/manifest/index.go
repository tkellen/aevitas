package manifest

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

// Index provides fast lookups for finding resources during rendering.
type Index struct {
	content   *index
	relations map[*Manifest]*index
}

// NewIndex does just what you think it does.
func NewIndex() *Index {
	index := &Index{
		content: newIndex(),
	}
	return index
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
		sort.Sort(i.content.shard[shard])
		totals = append(totals, fmt.Sprintf(format, shard, len(i.content.shard[shard].manifests)))
	}
	return strings.Join(totals, "\n")
}

// Insert adds a record to the index. Due to limitations in how relationships
// between manifests are currently handled, any insert invalidates the entire
// computed set of relations. In practice, inserting all manifests currently
// happens once and the index is read-only after that. If that changes, this
// will likely require revision.
func (i *Index) Insert(manifests ...*Manifest) (*Index, error) {
	i.relations = nil
	if err := i.content.insert(manifests...); err != nil {
		return nil, err
	}
	return i, nil
}

// FindMany produces an array of manifests whose selectors match the one provided.
func (i *Index) FindMany(target *Selector) ([]*Manifest, error) {
	shard, shardErr := i.shardOf(target)
	if shardErr != nil {
		return nil, shardErr
	}
	if target.IsWildcard() {
		return shard.manifests, nil
	}
	match, err := shard.findOne(target)
	if err != nil {
		return nil, err
	}
	return []*Manifest{match}, nil
}

// FindMany locates a single manifest based on the selector provided.
func (i *Index) FindOne(target *Selector) (*Manifest, error) {
	shard, shardErr := i.shardOf(target)
	if shardErr != nil {
		shard = i.content.all
	}
	return shard.findOne(target)
}

// Next finds the next latest manifest within the target's KGVN.
func (i *Index) Next(target *Manifest) *Manifest {
	shard, shardErr := i.shardOf(target.Selector)
	if shardErr != nil {
		return nil
	}
	return shard.next(target)
}

// Prev finds the next earliest manifest within the target's KGVN.
func (i *Index) Prev(target *Manifest) *Manifest {
	shard, shardErr := i.shardOf(target.Selector)
	if shardErr != nil {
		return nil
	}
	return shard.previous(target)
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

func (i *Index) isRelated(target *Manifest, mustRelateTo *Selector) (bool, error) {
	relations, relatedIndexErr := i.RelatedIndex(target)
	if relatedIndexErr != nil {
		return false, relatedIndexErr
	}
	// If mustRelateTo selector is not a wildcard, the target must relate to a
	// single other manifest.
	if !mustRelateTo.IsWildcard() {
		// The match is valid if it is directly related to mustRelateTo.
		_, findOneErr := relations.FindOne(mustRelateTo)
		if findOneErr != nil {
			return false, findOneErr
		}
		return true, nil
	}
	// If the mustRelateTo selector is a wildcard, a match is valid when it has
	// a relationship with _any_ manifest that matches the mustRelateTo shard
	// (aka kind/group/version/namespace)
	_, shardOfErr := relations.shardOf(mustRelateTo)
	if shardOfErr != nil {
		return false, shardOfErr
	}
	return true, nil
}

func (i *Index) FindOneWithRelation(target *Selector, mustRelateTo *Selector) (*Manifest, error) {
	match, findErr := i.FindOne(target)
	if findErr != nil {
		return nil, findErr
	}
	ok, err := i.isRelated(match, mustRelateTo)
	if err != nil {
		return nil, err
	}
	if ok {
		return match, nil
	}
	return nil, nil
}

// FindManyWithRelation searches the relationships of a source selector for any
// manifests that are related to the mustRelateTo selector.
func (i *Index) FindManyWithRelation(target *Selector, mustRelateTo *Selector) ([]*Manifest, error) {
	matches, findErr := i.FindMany(target)
	if findErr != nil {
		return nil, findErr
	}
	var validMatches []*Manifest
	for _, match := range matches {
		ok, _ := i.isRelated(match, mustRelateTo)
		if ok {
			validMatches = append(validMatches, match)
		}
	}
	return validMatches, nil
}

func (i *Index) shardOf(target *Selector) (*shard, error) {
	shardKey := target.KGVN()
	shard, exists := i.content.shard[shardKey]
	if exists {
		return shard, nil
	}
	return nil, fmt.Errorf("%s is empty", shardKey)
}

func (i *Index) ComputeRelations() error {
	i.relations = map[*Manifest]*index{}
	for _, item := range i.content.all.manifests {
		relatedSelectors, relatedErr := item.relations(i)
		if relatedErr != nil {
			return relatedErr
		}
		for _, selector := range relatedSelectors {
			if !selector.IsWildcard() {
				related, err := i.FindOne(selector)
				if err != nil {
					return fmt.Errorf("%s: %w", item, err)
				}
				i.addRelation(item, related)
			} else {
				return fmt.Errorf("%s: wildcard not allowed", selector)
			}
		}
	}
	return nil
}

// addRelation records a relationship from one manifest to another and the
// inverse relationship back.
func (i *Index) addRelation(parent *Manifest, related ...*Manifest) {
	if _, ok := i.relations[parent]; !ok {
		i.relations[parent] = newIndex()
	}
	// Make parent related to all supplied manifests. Ignore duplicate insertion
	// errors as this is expected.
	_ = i.relations[parent].insert(related...)
	for _, target := range related {
		if _, ok := i.relations[target]; !ok {
			i.relations[target] = newIndex()
		}
		// Make all supplied manifests relate back to parent. Ignore duplicate
		// insertion errors as this is expected.
		_ = i.relations[target].insert(parent)
	}
}

type index struct {
	all   *shard            // all manifests.
	shard map[string]*shard // manifests sharded by KGVN
}

func newIndex() *index {
	return &index{
		all:   newShard(),
		shard: map[string]*shard{},
	}
}

// insert adds N manifests to the index.
func (i *index) insert(manifests ...*Manifest) error {
	for _, manifest := range manifests {
		if err := i.all.insert(manifest); err != nil {
			return err
		}
		// shard index by kind group version namespace
		shardKey := manifest.Selector.KGVN()
		shard, ok := i.shard[shardKey]
		if !ok {
			i.shard[shardKey] = newShard()
			shard = i.shard[shardKey]
		}
		if err := shard.insert(manifest); err != nil {
			return err
		}
	}
	return nil
}

// shard holds a collection of manifests.
type shard struct {
	manifests []*Manifest
	byID      map[string]*Manifest
	notLive   map[string]*Manifest
}

// newShard does just what you think it does.
func newShard() *shard {
	return &shard{
		manifests: []*Manifest{},
		byID:      map[string]*Manifest{},
		notLive:   map[string]*Manifest{},
	}
}

func (l shard) Len() int {
	return len(l.manifests)
}

func (l shard) Swap(i, j int) {
	l.manifests[i], l.manifests[j] = l.manifests[j], l.manifests[i]
}

func (l shard) Less(i, j int) bool {
	return l.manifests[i].Less(l.manifests[j])
}

func (l *shard) findOne(target *Selector) (*Manifest, error) {
	id := target.ID()
	manifest, found := l.byID[id]
	if !found {
		if m, notLive := l.notLive[id]; notLive {
			return nil, fmt.Errorf("%s: must be \"live\" to be used", m)
		}
		return nil, fmt.Errorf("%s: not found", id)
	}
	return manifest, nil
}

func (l *shard) next(target *Manifest) *Manifest {
	var result *Manifest
	for _, item := range l.manifests {
		if item.Less(target) && (result == nil || item.Greater(result)) {
			result = item
		}
	}
	return result
}

func (l *shard) previous(target *Manifest) *Manifest {
	var result *Manifest
	for _, item := range l.manifests {
		if item.Greater(target) && (result == nil || item.Less(result)) {
			result = item
		}
	}
	return result
}

// insert adds N manifests that are currently marked as live and, if there is a
// publication date, that the date is older than the time this is run. If a
// manifest of the same ID has been previously inserted, trigger an error. This
// error is for detecting duplicates during initial index creation.
func (l *shard) insert(manifests ...*Manifest) error {
	var collisions bytes.Buffer
	for _, manifest := range manifests {
		id := manifest.Selector.ID()
		// Skip unpublished resources (save for helpful error messages though)
		if !manifest.IsLive() {
			l.notLive[id] = manifest
			continue
		}
		// There is an entry for this manifest already, collect the collision.
		if _, ok := l.byID[id]; ok {
			collisions.WriteString(fmt.Sprintf("%s\n", manifest.Selector))
		}
		l.byID[id] = manifest
		l.manifests = append(l.manifests, manifest)
	}
	// If there were any collisions, enumerate them all in the returned error.
	if collisions.Len() > 0 {
		return fmt.Errorf("collisions:\n%s", collisions.String())
	}
	return nil
}
