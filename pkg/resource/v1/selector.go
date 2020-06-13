package resource

import (
	"fmt"
	"strings"
)

// Selector uniquely identifies a resource.
type Selector struct {
	// Kind describes at the highest level what a resource contains. Usage of
	// IANA media types is encouraged.
	Kind string
	// Group allows subcategorization of resources of a given kind. Usage of
	// IANA media subtypes is encouraged.
	Group string
	// Version describes which iteration of a given kind/group a resource
	// describes.
	Version string
	// Name provides a human-friendly identifier for a given resource.
	Name string
}

type SelectorList []Selector
// Len returns the length of the underlying array.
func (sl SelectorList) Len() int { return len(sl) }

// Less returns which of two indexes in the array is "smaller" alphanumerically.
func (sl SelectorList) Less(i, j int) bool { return sl[i].ID() < sl[j].ID() }

// Swap re-orders the underlying array (used by sort.Sort).
func (sl SelectorList) Swap(i, j int) { sl[i], sl[j] = sl[j], sl[i] }

// NewSelector produces a selector from a string. The expected form is:
// "kind/group/version/name". A wildcard selector is allowed in this form:
// "kind/group/version/*".
func NewSelector(selector string) (Selector, error) {
	parts := strings.Split(selector, "/")
	if len(parts) != 4 {
		return Selector{}, fmt.Errorf("unsupported selector: %s", selector)
	}
	return Selector{
		Kind:    parts[0],
		Group:   parts[1],
		Version: parts[2],
		Name:    parts[3],
	}, nil
}

// KGV returns a "kind group version" string sfor a given selector.
func (s *Selector) KGV() string { return s.Kind + "/" + s.Group + "/" + s.Version }

// ID returns a full string representation of the selector.
func (s *Selector) ID() string { return s.KGV() + "/" + s.Name }

// IsWildcard indicates if a selector is meant to reference all resources of a
// given kind/group/version.
func (s Selector) IsWildcard() bool { return s.Name == "*" }

// String returns a full string representation of the selector.
func (s Selector) String() string { return s.ID() }

// Match returns a boolean indicating if the provided selector matches
func (s Selector) Matches(target string) bool {
	check, err := NewSelector(target)
	if err != nil {
		return false
	}
	return check == s || (s.IsWildcard() &&
		check.Group == s.Group &&
		check.Kind == s.Kind &&
		check.Version == s.Version)
}

func (s Selector) From(index IndexedList) (*Resource, error) {
	return index.LookupSelector(s)
}