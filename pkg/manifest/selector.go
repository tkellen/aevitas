// Package selector provides the basis for identifying and locating resources.
package manifest

import (
	"fmt"
	json "github.com/json-iterator/go"
	"strings"
)

// Selector uniquely identifies a resource.
type Selector struct {
	// Kind describes at the highest level what a resource contains.
	Kind string
	// Group allows subcategorization of resources of a given kind.
	Group string
	// Version describes which iteration of a kind/group the resource describes.
	Version string
	// Name provides a human-friendly identifier for the resource.
	Name string
	// Namespace provides a final level of grouping for resources.
	Namespace string
}

// List defines an array of selectors.
type SelectorList []*Selector

// Len returns the length of the underlying array.
func (l SelectorList) Len() int { return len(l) }

// Less determines which of two selectors is alphanumerically "smaller".
func (l SelectorList) Less(i, j int) bool { return l[i].ID() < l[j].ID() }

// Swap re-orders the underlying array (used by sort.Sort).
func (l SelectorList) Swap(i, j int) { l[i], l[j] = l[j], l[i] }

// NewSelector produces a selector from a string. The expected form is:
// "namespace/kind/group/version/name". A wildcard selector is allowed in this
// form: "namespace/kind/group/version/*".
func NewSelector(selector string) (*Selector, error) {
	parts := strings.Split(selector, "/")
	if len(parts) != 5 {
		return nil, fmt.Errorf("unsupported selector: %s", selector)
	}
	return &Selector{
		Namespace: parts[0],
		Kind:      parts[1],
		Group:     parts[2],
		Version:   parts[3],
		Name:      parts[4],
	}, nil
}

func (s *Selector) Validate() error {
	if s.Namespace == "" || s.Kind == "" || s.Group == "" || s.Version == "" || s.Name == "" {
		return fmt.Errorf("namespace, kind, group, version and name must be set: %", s)
	}
	return nil
}

// KGV returns a "kind group version" string for the selector.
func (s *Selector) KGV() string { return fmt.Sprintf("%s/%s/%s", s.Kind, s.Group, s.Version) }

// NKGV returns a "namespace kind group version" string for the selector.
func (s *Selector) NKGV() string { return fmt.Sprintf("%s/%s", s.Namespace, s.KGV()) }

// ID returns a full string representation of the selector.
func (s *Selector) ID() string { return fmt.Sprintf("%s/%s", s.NKGV(), s.Name) }

// IsWildcard indicates if a selector is meant to reference all resources of a
// given kind/group/version/namespace.
func (s *Selector) IsWildcard() bool { return s.Name == "*" }

// String returns a full string representation of the selector.
func (s *Selector) String() string { return s.ID() }

// Match returns a boolean indicating if the provided selector matches
func (s *Selector) Matches(target string) bool {
	check, err := NewSelector(target)
	if err != nil {
		return false
	}
	return check == s || ((s.IsWildcard() || check.IsWildcard()) &&
		check.Namespace == s.Namespace &&
		check.Group == s.Group &&
		check.Kind == s.Kind &&
		check.Version == s.Version)
}

// UnmarshalJSON instantiates a selector from a string.
func (s *Selector) UnmarshalJSON(data []byte) error {
	var entry string
	if err := json.Unmarshal(data, &entry); err != nil {
		return err
	}
	result, err := NewSelector(entry)
	if err != nil {
		return err
	}
	*s = *result
	return nil
}
