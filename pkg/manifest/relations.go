package manifest

import (
	"fmt"
	"github.com/tkellen/aevitas/internal/selector"
	"sort"
)

// Relation describes a relationship to one or many manifests.
type Relation struct {
	// Name gives a unique name to the relation.
	Name string
	// Selector points to the manifest(s) that are "related".
	Selector *selector.Selector
	// MatchIfRelatedTo is the first step in finding matched manifests. Each
	// selector accumulates more potential matches (multiple entries are OR'd).
	MatchIfRelatedTo []*selector.Selector
	// MatchExpression is iterated in order, successively narrowing eligible
	// matched manifests with each step (multiple entries are AND'd).
	MatchExpression []*MatchExpression
	Limit           int
	Offset          int
	Order           string
}

// validate does just what you think it does.
func (r *Relation) validate() error {
	if r.Selector == nil {
		return fmt.Errorf("selector must not be nil")
	}
	if r.Order != "" && r.Order != "asc" && r.Order != "desc" {
		return fmt.Errorf("order must be asc or desc")
	}
	for _, matcher := range r.MatchExpression {
		if err := matcher.validate(); err != nil {
			return err
		}
	}
	return nil
}

// Resolve turns a relation into (potentially) many manifests by searching the
// index for matches and filtering the results on match expressions.
func (r *Relation) Resolve(index *Index) ([]*Manifest, error) {
	return r.resolve(index, nil, false)
}

func (r *Relation) resolve(index *Index, context *Manifest, mustBeRelatedToContext bool) ([]*Manifest, error) {
	var validMatches manifestList
	var findErr error
	if mustBeRelatedToContext {
		if validMatches, findErr = index.FindManyWithRelation(r.Selector, context.Selector); findErr != nil {
			return nil, findErr
		}
	} else if len(r.MatchIfRelatedTo) == 0 {
		// If validMatches manifests are not constrained by context and there are
		// no "related to" selectors, find all that satisfy the selector.
		if validMatches, findErr = index.FindMany(r.Selector); findErr != nil {
			return nil, findErr
		}
	}
	// If validMatches manifests are constrained by their relationships,
	// accumulate valid ones using the index.
	for _, related := range r.MatchIfRelatedTo {
		matched, findErr := index.FindManyWithRelation(r.Selector, related)
		if findErr != nil {
			return nil, findErr
		}
		validMatches = append(validMatches, matched...)
	}
	// If there are matchExpressions, narrow validMatches to those that satisfy
	// the matching criteria.
	for _, matcher := range r.MatchExpression {
		// Each matcher narrows validMatches further.
		// If the count reaches zero, exit early.
		if len(validMatches) == 0 {
			break
		}
		var filterErr error
		if validMatches, filterErr = matcher.filter(validMatches, context); filterErr != nil {
			return nil, filterErr
		}
	}
	if r.Order == "" || r.Order == "asc" {
		sort.Sort(validMatches)
	} else {
		sort.Sort(sort.Reverse(validMatches))
	}
	if r.Offset == 0 && r.Limit == 0 {
		return validMatches, nil
	}
	// Apply limiting and offsets.
	offset := r.Offset
	limit := r.Limit
	total := len(validMatches)
	if offset > total {
		offset = total
	}
	if limit == 0 {
		limit = total
	} else {
		limit = offset + limit
		if limit > total {
			limit = total
		}
	}
	return validMatches[offset:limit], nil
}

// MatchExpression describes how manifest relationships can be filtered.
type MatchExpression struct {
	Key      string
	Operator string
	Values   []interface{}
}

// validate does just what you think it does.
func (m *MatchExpression) validate() error {
	if m.Operator == "InSameMonthAsContext" || m.Operator == "OnSameDayAsContext" {
		if len(m.Values) != 0 {
			return fmt.Errorf("%s cannot contain explicit values", m.Operator)
		}
	} else if len(m.Values) == 0 {
		return fmt.Errorf("values must contain at least one entry")
	}
	return nil
}

func (m *MatchExpression) filter(search []*Manifest, context *Manifest) ([]*Manifest, error) {
	var filtered []*Manifest
	var useValues bool
	var compare func(*Manifest, interface{}) bool
	switch op := m.Operator; op {
	case "InYear":
		compare = func(potential *Manifest, compare interface{}) bool {
			return potential.Meta.PublishAt.Year == int(compare.(float64))
		}
		useValues = true
	case "InMonth":
		compare = func(potential *Manifest, compare interface{}) bool {
			return potential.Meta.PublishAt.Month == int(compare.(float64))
		}
		useValues = true
	case "OnDate":
		compare = func(potential *Manifest, compare interface{}) bool {
			matchWith := compare.([]interface{})
			return potential.Meta.PublishAt.Year == int(matchWith[0].(float64)) &&
				potential.Meta.PublishAt.Month == int(matchWith[1].(float64)) &&
				potential.Meta.PublishAt.Day == int(matchWith[2].(float64))
		}
		useValues = true
	case "InSameMonthAsContext":
		compare = func(potential *Manifest, _ interface{}) bool {
			if potential == context {
				return false
			}
			return potential.Meta.PublishAt.Month == context.Meta.PublishAt.Month
		}
	case "OnSameDayAsContext":
		compare = func(potential *Manifest, _ interface{}) bool {
			if potential == context {
				return false
			}
			return potential.Meta.PublishAt.Day == context.Meta.PublishAt.Day
		}
	default:
		return nil, fmt.Errorf("%s is not (yet) a supported operator", op)
	}
	// Iterate each of the currently valid matches, populating the filtered
	// array with each that is still valid.
	if len(m.Values) == 0 {
		for _, potential := range search {
			if useValues {
				for _, check := range m.Values {
					if compare(potential, check) {
						filtered = append(filtered, potential)
						break
					}
				}
			} else if compare(potential, nil) {
				filtered = append(filtered, potential)
			}

		}
	}
	return filtered, nil
}

// DynamicRelation defines a relationship with other manifests through a
// relationship with another.
type DynamicRelation struct {
	Relation
	SelectorTemplate        string
	MatchIfRelatedToContext bool
}

func (dr *DynamicRelation) validate() error {
	return dr.Relation.validate()
}

func (dr *DynamicRelation) Resolve(index *Index, context *Manifest) ([]*Manifest, error) {
	if dr.MatchIfRelatedToContext {
		return dr.resolve(index, context, true)
	}
	return dr.resolve(index, context, false)
}
