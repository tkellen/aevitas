package website

import (
	"fmt"
	json "github.com/json-iterator/go"
	"github.com/tkellen/aevitas/pkg/manifest"
)

const KGVContent = "website/content/v1"

type Content struct {
	*manifest.Manifest
	Spec *ContentSpec
}

type ContentSpec struct {
	Title       string
	Description string
	Href        string
	Body        string
}

func NewContent(m *manifest.Manifest) (*Content, error) {
	instance := &Content{
		Manifest: m,
		Spec:     &ContentSpec{},
	}
	if err := json.Unmarshal(m.Spec, instance.Spec); err != nil {
		return nil, err
	}
	if instance.Spec.Description == "" {
		instance.Spec.Description = instance.Spec.Body
		if len(instance.Spec.Body) > 50 {
			instance.Spec.Description = instance.Spec.Body[0:50]
		}
	}
	if instance.Spec.Description != "" && instance.Spec.Body == "" {
		instance.Spec.Body =instance.Spec.Description
	}
	if err := instance.Validate(); err != nil {
		return nil, err
	}
	return instance, nil
}

func (c *Content) Validate() error {
	if c.Spec.Href == "" {
		return fmt.Errorf("href must be defined")
	}
	if c.Spec.Title == "" {
		return fmt.Errorf("title must be defined")
	}
	if c.Spec.Description == "" {
		return fmt.Errorf("description must be defined")
	}
	if c.Spec.Body == "" {
		return fmt.Errorf("body must be defined")
	}
	return nil
}
func (c *Content) Body() string { return c.Spec.Body }
func (c *Content) Href() string    { return c.Spec.Href }
func (c *Content) Title() string   { return c.Spec.Title }
