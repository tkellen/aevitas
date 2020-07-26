// Package selector provides the basis for identifying and locating resources.
package manifest

import (
	"fmt"
	json "github.com/json-iterator/go"
	"strings"
)

// selector uniquely identifies a resource.
type Selector struct {
	// Kind describes at the highest level what a resource contains.
	Kind string
	// Group allows subcategorization of resources of a given kind.
	Group string
	// Version describes which iteration of a kind/group the resource describes.
	Version string
	// Namespace allows grouping of KGVs
	Namespace string
	// id provides a human-friendly identifier for the resource.
	Name string
}

// NewSelector produces a selector from a string. The expected form is:
// "namespace/kind/group/version/name". A wildcard selector is allowed in this
// form: "namespace/kind/group/version/*".
func NewSelector(selector string) (*Selector, error) {
	parts := strings.Split(selector, "/")
	if len(parts) != 5 {
		return nil, fmt.Errorf("unsupported selector: %s", selector)
	}
	instance := &Selector{
		Kind:      parts[0],
		Group:     parts[1],
		Version:   parts[2],
		Namespace: parts[3],
		Name:      parts[4],
	}
	if err := instance.Validate(); err != nil {
		return nil, err
	}
	return instance, nil
}

func NewSelectorMust(selector string) *Selector {
	instance, err := NewSelector(selector)
	if err != nil {
		panic(err)
	}
	return instance
}

func (s Selector) Validate() error {
	if s.Kind == "" || s.Group == "" || s.Version == "" || s.Name == "" || s.Namespace == "" {
		return fmt.Errorf("kind, group, version, name and namespace must be set: %s", s)
	}
	return nil
}

// KGV returns a "kind group version" string for the selector.
func (s Selector) KGV() string { return fmt.Sprintf("%s/%s/%s", s.Kind, s.Group, s.Version) }

// KGV returns a "kind group version namespace" string for the selector.
func (s Selector) KGVN() string { return fmt.Sprintf("%s/%s", s.KGV(), s.Namespace) }

// ID returns a full string representation of the selector.
func (s Selector) ID() string { return fmt.Sprintf("%s/%s", s.KGVN(), s.Name) }

// IsWildcard indicates if a selector is meant to reference all manifests of
// a namespace/kind/group/version.
func (s Selector) IsWildcard() bool { return s.Name == "*" }

// String returns a full string representation of the selector.
func (s Selector) String() string { return s.ID() }

// Match returns a boolean indicating if the provided selector matches
func (s Selector) Matches(check Selector) bool {
	return check.Kind == s.Kind &&
		check.Group == s.Group &&
		check.Version == s.Version &&
		check.Namespace == s.Namespace &&
		(check.Name == s.Name || check.IsWildcard() || s.IsWildcard())
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
