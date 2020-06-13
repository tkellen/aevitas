package resource

import (
	"fmt"
	json "github.com/json-iterator/go"
)

// Import describes how a resource can depend on another.
type Import struct {
	// Resource is a selector string for a resource that is required for the
	// rendering of the parent resource.
	Resource Selector
	// Template is an optional selector string for a resource that should be
	// used to render the resource being imported.
	Template Selector
	// As provides an alternative name for referring to the imported resource.
	As string
}

// NewImport produces an import entry and validates it.
func NewImport(resource string, template string, as string) (*Import, error) {
	instance := &Import{As: as}
	if resource != "" {
		selector, err := NewSelector(resource)
		if err != nil {
			return nil, err
		}
		instance.Resource = selector
	}
	if template != "" {
		selector, err := NewSelector(template)
		if err != nil {
			return nil, err
		}
		instance.Template = selector
	}
	if err := instance.Validate(); err != nil {
		return nil, err
	}
	return instance, nil
}

// Validate does just what you think it does.
func (i *Import) Validate() error {
	if i.Template != (Selector{}) && i.Template.IsWildcard() {
		return fmt.Errorf("template cannot be wildcard selector")
	}
	if i.As != "" && i.Resource != (Selector{}) && i.Resource.IsWildcard() {
		return fmt.Errorf("wildcard selectors cannot be aliased")
	}
	return nil
}

// Expand a single import to a static array of imports by finding matching
// entries in the supplied index.
func (i *Import) Expand(index IndexedList) ([]Import, error) {
	if !i.Resource.IsWildcard() {
		return []Import{*i}, nil
	}
	shard, ok := index[i.Resource.KGV()]
	if !ok {
		return nil, fmt.Errorf("index has no entries for %s", i.Resource)
	}
	imports := make([]Import, len(shard.Resources))
	for idx, resource := range shard.Resources {
		imports[idx] = Import{
			Template: i.Template,
			Resource: resource.Selector,
		}
	}
	return imports, nil
}

// UnmarshalJSON converts string selectors in an import to valid selectors.
func (i *Import) UnmarshalJSON(data []byte) error {
	var entry struct {
		Resource string
		Template string
		As string
	}
	if err := json.Unmarshal(data, &entry); err != nil {
		return err
	}
	imported, err := NewImport(entry.Resource, entry.Template, entry.As)
	if err != nil {
		return err
	}
	*i = *imported
	return nil
}