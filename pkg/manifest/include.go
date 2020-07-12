package manifest

import (
	"fmt"
	json "github.com/json-iterator/go"
)

// included describes how a manifest can depend on other manifests.
type Include struct {
	// Resource points to a resource that is required for rendering the
	// manifest this import belongs to.
	Resource *Selector
	// Templates optionally points to resources that should be used as a "view"
	// of the resource.
	Templates []*Selector
	// As provides an alternative name for referring to the imported resource.
	As string
}

// NewInclude produces an import entry and validates it.
func NewInclude(resource string, templates []string, as string) (*Include, error) {
	instance := &Include{As: as}
	if resource != "" {
		s, err := NewSelector(resource)
		if err != nil {
			return nil, err
		}
		instance.Resource = s
	}
	for _, tmpl := range templates {
		s, err := NewSelector(tmpl)
		if err != nil {
			return nil, err
		}
		instance.Templates = append(instance.Templates, s)
	}
	if err := instance.Validate(); err != nil {
		return nil, err
	}
	return instance, nil
}

// Validate does just what you think it does.
func (i *Include) Validate() error {
	for _, tmpl := range i.Templates {
		if tmpl.NameIsWildcard() {
			return fmt.Errorf("template selector cannot be wildcard")
		}
	}
	if i.As != "" && i.Resource != nil && i.Resource.IsWildcard() {
		return fmt.Errorf("wildcard selectors cannot be aliased")
	}
	return nil
}

// Expand turns an include with a wildcard selector into a static array of fully
// specified includes by matching against a supplied index.
func (i *Include) Expand(index *Index) []*Include {
	if i.Resource.IsWildcard() {
		// Error ignored because wildcard selector is valid with no matches.
		manifests, _ := index.Find(SelectorList{i.Resource})
		includes := make([]*Include, len(manifests))
		for idx, manifest := range manifests {
			includes[idx] = &Include{
				Resource: manifest.Selector,
				Templates:  i.Templates,
				As:       manifest.ID(),
			}
		}
		return includes
	}
	return []*Include{i}
}

// UnmarshalJSON converts string selectors in an import payload to instantiated
// Selectors.
func (i *Include) UnmarshalJSON(data []byte) error {
	var entry struct {
		Resource string
		Templates  []string
		As       string
	}
	if err := json.Unmarshal(data, &entry); err != nil {
		return err
	}
	imported, err := NewInclude(entry.Resource, entry.Templates, entry.As)
	if err != nil {
		return err
	}
	*i = *imported
	return nil
}
