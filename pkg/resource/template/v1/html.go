package v1

import (
	"github.com/go-git/go-billy/v5"
	json "github.com/json-iterator/go"
	metav1 "github.com/tkellen/aevitas/pkg/resource/meta/v1"
	"html/template"
)

type Html struct {
	Meta metav1.Meta
	Spec HtmlSpec
}

type HtmlSpec struct {
	Templates []TemplateSpec
}

type TemplateSpec struct {
	tmpl template.HTML
}

func NewHtml(manifest []byte) (*Html, error) {
	var instance Html
	if err := json.Unmarshal(manifest, &instance); err != nil {
		return nil, err
	}
	if err := instance.Validate(); err != nil {
		return nil, err
	}
	return &instance, nil
}

func (h *Html) Current(_ billy.Filesystem) bool {
	return false
}

func (h *Html) Validate() error {
	return nil
}
