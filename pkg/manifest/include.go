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
	// template optionally points to a resource that should be used to
	// render the resource being imported.
	Template *Selector
	// Layout optionally points to a resource that should be used to
	// wrap the resource being imported after the template has been applied.
	Layouts []*Selector
	// As provides an alternative name for referring to the imported resource.
	As string
}

// NewInclude produces an import entry and validates it.
func NewInclude(resource string, template string, layouts []string, as string) (*Include, error) {
	instance := &Include{As: as}
	if resource != "" {
		s, err := NewSelector(resource)
		if err != nil {
			return nil, err
		}
		instance.Resource = s
	}
	if template != "" {
		s, err := NewSelector(template)
		if err != nil {
			return nil, err
		}
		instance.Template = s
	}
	for _, layout := range layouts {
		s, err := NewSelector(layout)
		if err != nil {
			return nil, err
		}
		instance.Layouts = append(instance.Layouts, s)
	}
	if err := instance.Validate(); err != nil {
		return nil, err
	}
	return instance, nil
}

// Validate does just what you think it does.
func (i *Include) Validate() error {
	if i.Template != nil && i.Template.IsWildcard() {
		return fmt.Errorf("template selector cannot be wildcard")
	}
	for _, layout := range i.Layouts {
		if layout.IsWildcard() {
			return fmt.Errorf("layout selector cannot be wildcard")
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
				Template: i.Template,
				Layouts:  i.Layouts,
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
		Template string
		Layouts  []string
		As       string
	}
	if err := json.Unmarshal(data, &entry); err != nil {
		return err
	}
	imported, err := NewInclude(entry.Resource, entry.Template, entry.Layouts, entry.As)
	if err != nil {
		return err
	}
	*i = *imported
	return nil
}
