// Package selector provides the basis for identifying and locating resources.
package selector

import (
	"fmt"
	json "github.com/json-iterator/go"
	"strings"
)

// Selector uniquely identifies a resource. Care has been taken to ensure this
// is as fast as possible.
type Selector struct {
	// Raw is the Raw input used to produce the selector
	Raw string
	// KGV is a memoized entry for `kind/group/version`
	KGV string
	// KGVN is a memoized entry for `kind/group/version/Name`
	KGVN string
	// Name provides a human-friendly identifier for the resource.
	Name string
}

// New produces a selector from a string.
// The expected form is: "namespace/kind/group/version/Name".
// A wildcard selector is this form: "namespace/kind/group/version/*".
func New(selector string) (*Selector, error) {
	parts := strings.Split(selector, "/")
	if len(parts) != 5 {
		return nil, fmt.Errorf("unsupported selector: %s", selector)
	}
	instance := &Selector{
		Raw:  selector,
		KGV:  strings.Join(parts[0:3], "/"),
		KGVN: strings.Join(parts[0:4], "/"),
		Name: parts[4],
	}
	if err := instance.Validate(); err != nil {
		return nil, err
	}
	return instance, nil
}

func Must(selector string) *Selector {
	instance, err := New(selector)
	if err != nil {
		panic(err)
	}
	return instance
}

func (s Selector) Validate() error {
	parts := strings.Split(s.Raw, "/")
	if parts[0] == "" || parts[1] == "" || parts[2] == "" || parts[3] == "" || parts[4] == "" {
		return fmt.Errorf("kind, group, version, Name and namespace must be set: %s", s)
	}
	return nil
}

// String returns a full string representation of the selector.
func (s Selector) String() string { return s.ID() }

// ID returns a full string representation of the selector.
func (s Selector) ID() string { return s.Raw }

// IsWildcard indicates if a selector is meant to reference all manifests of
// a namespace/kind/group/version.
func (s Selector) IsWildcard() bool { return s.Name == "*" }

// Match returns a boolean indicating if the provided selector matches
func (s Selector) Matches(check *Selector) bool {
	return check.KGVN == s.KGVN && (check.Name == s.Name || check.IsWildcard() || s.IsWildcard())
}

// UnmarshalJSON instantiates a selector from a string.
func (s *Selector) UnmarshalJSON(data []byte) error {
	var entry string
	if err := json.Unmarshal(data, &entry); err != nil {
		return err
	}
	result, err := New(entry)
	if err != nil {
		return err
	}
	*s = *result
	return nil
}
