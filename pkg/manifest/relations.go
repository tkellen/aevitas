package manifest

// Relations holds information about how manifests relate to one another.
type Relations map[*Manifest]*Index

// NewRelations creates a bi-directional index of relationships between
// manifests in a supplied index.
func NewRelations(index *Index) (Relations, error) {
	relations := Relations{}
	for _, item := range index.All.Manifests {
		// Record the manifests this manifest has specified relating to.
		for _, relatedSelector := range item.Meta.Related {
			related, err := index.Find(relatedSelector)
			if err != nil {
				return nil, err
			}
			relations.Add(item, related...)
		}
		// Record the manifests this manifest has included
		if err := item.EachInclude(index, func(include *Include) error {
			target, err := index.GetSelector(include.Resource)
			if err != nil {
				return err
			}
			relations.Add(item, target)
			return nil
		}); err != nil {
			return nil, err
		}
	}
	return relations, nil
}

// Add records a relationship from one manifest to others and the inverse
// relationship back.
func (r Relations) Add(parent *Manifest, related ...*Manifest) {
	if _, ok := r[parent]; !ok {
		r[parent], _ = NewIndex([]*Manifest{})
	}
	// Make parent related to all supplied manifests. Ignore duplicate insertion
	// errors as this is expected.
	_ = r[parent].Insert(related...)
	for _, target := range related {
		if _, ok := r[target]; !ok {
			r[target], _ = NewIndex([]*Manifest{})
		}
		// Make all supplied manifests related back to parent. Ignore duplicate
		// insertion errors as this is expected.
		_ = r[target].Insert(parent)
	}
}
