package manifest

import (
	"encoding/json"
	"fmt"
	"github.com/tidwall/gjson"
)

// Relation describes a relationship to one or many manifests.
type Relation struct {
	// Selector points to the manifest(s) that are "related".
	Selector *Selector
	// MatchIfRelatedTo is the first step in finding matched manifests. Each
	// selector accumulates more potential matches (multiple entries are OR'd).
	MatchIfRelatedTo []*Selector
	// MatchExpression is iterated in order, successively narrowing eligible
	// matched manifests with each step (multiple entries are AND'd).
	MatchExpression []*MatchExpression
}

// Validate does just what you think it does.
func (r *Relation) Validate() error {
	for _, matcher := range r.MatchExpression {
		if err := matcher.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Resolve turns a relation into (potentially) many manifests by searching the
// index for matches and filtering the results on match expressions.
func (r *Relation) Resolve(index *Index) ([]*Selector, error) {
	var validMatches []*Manifest
	// If validMatches manifests are not constrained by relationships, find all that
	// satisfy the selector.
	if len(r.MatchIfRelatedTo) == 0 {
		var findErr error
		if validMatches, findErr = index.FindMany(r.Selector); findErr != nil {
			return nil, findErr
		}
	}
	// If validMatches manifests are constrained by their relationships, accumulate
	// valid ones using the index.
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
		// Each matcher narrows validMatches further. If the count reaches zero,
		// exit early.
		if len(validMatches) == 0 {
			break
		}
		// Prepare a new list of valid manifests.
		var filtered []*Manifest
		// Iterate each of the currently valid matches, populating the filtered
		// array with each that is still valid.
		for _, potential := range validMatches {
			value := gjson.GetBytes(potential.Json, matcher.Key)
			if matcher.Operator == "In" {
				for _, compare := range matcher.Values {
					if fmt.Sprintf("%s", value.Raw) == fmt.Sprintf("%s", compare) {
						filtered = append(filtered, potential)
						break
					}
				}
			}
		}
		// Narrow validMatches to the filtered set and repeat.
		validMatches = filtered
	}
	results := make([]*Selector, len(validMatches))
	for idx, manifest := range validMatches {
		results[idx] = manifest.Selector
	}
	return results, nil
}

// MatchExpression describes how manifest relationships can be filtered.
type MatchExpression struct {
	Key      string
	Operator string
	Values   []json.RawMessage
}

// Validate does just what you think it does.
func (m *MatchExpression) Validate() error {
	if m.Operator != "In" {
		return fmt.Errorf("%s is not (yet) a supported operator", m.Operator)
	}
	if m.Key == "" {
		return fmt.Errorf("key must be defined")
	}
	if len(m.Values) == 0 {
		return fmt.Errorf("values must contain at least one entry")
	}
	return nil
}
