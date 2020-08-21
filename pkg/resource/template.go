package resource

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/tkellen/aevitas/pkg/manifest"
	"html/template"
	"strconv"
)

// Template extends a Resource with additional context needed to fully body it
// to text.
type Template struct {
	*Resource
	renderWith []*Template
	id         string
}

func NewTemplate(self *Resource) (*Template, error) {
	var id bytes.Buffer
	id.WriteString(self.Hash)
	id.WriteString(self.index.RelationsHash(self.Manifest))
	template := &Template{Resource: self}
	imports, importsErr := self.ResolveStaticImports(self.index)
	if importsErr != nil {
		return nil, importsErr
	}
	for _, imported := range imports {
		for _, manifest := range imported.Manifests {
			id.WriteString(manifest.Hash)
		}
	}
	if err := template.mergeImports(template.associated, imports); err != nil {
		return nil, err
	}
	renderWith, resolveErr := self.Meta.RenderWith.Resolve(self.index)
	if resolveErr != nil {
		return nil, resolveErr
	}
	for _, item := range renderWith {
		resource, err := self.newStub(item, nil)
		if err != nil {
			return nil, err
		}
		layout, layoutErr := NewTemplate(resource)
		if layoutErr != nil {
			return nil, layoutErr
		}
		id.WriteString(layout.id)
		template.renderWith = append(template.renderWith, layout)
	}
	template.id = id.String()
	return template, nil
}

func (t *Template) render(context *Template, yield template.HTML) (template.HTML, error) {
	var err error
	if context == nil {
		context = t
	}
	if yield, err = t.body(context, yield); err != nil {
		return "", fmt.Errorf("%s: %w", context, err)
	}
	for _, tmpl := range t.renderWith {
		if yield, err = tmpl.body(context, yield); err != nil {
			return "", fmt.Errorf("%s: %w", tmpl, err)
		}
		for _, innerTmpl := range tmpl.renderWith {
			if yield, err = innerTmpl.render(t, yield); err != nil {
				return "", fmt.Errorf("%s: %w", innerTmpl, err)
			}
		}
	}
	return yield, nil
}

func (t *Template) body(context interface{}, yield template.HTML) (template.HTML, error) {
	var buf bytes.Buffer
	if context == nil {
		context = t
	}
	funcMap := map[string]interface{}{}
	funcMap["yield"] = func() template.HTML { return yield }
	funcMap["ordinal"] = ordinal
	merge(funcMap, t.associated)
	if tmpl, ok := context.(*Template); ok {
		imports, err := t.ResolveDynamicImports(t.index, tmpl.Manifest)
		if err != nil {
			return "", err
		}
		if err = tmpl.mergeImports(funcMap, imports); err != nil {
			return "", err
		}
	}
	// This crazy hack makes template error messages a lot more readable.
	tmpl, tmplErr := template.
		Must(template.New("root").Parse("{{template \"self\" . }}")).
		New(t.String()).Funcs(funcMap).
		Parse(fmt.Sprintf("{{block \"self\" .}}%s{{end}}", t.Body))
	// tmpl, tmplErr := template.New(t.String()).Funcs(funcMap).Parse(t.Body)
	// ^ this is the non-hacked-up call that was replaced to make error messages
	// readable.
	if tmplErr != nil {
		return "", tmplErr
	}
	if err := tmpl.Execute(&buf, context); err != nil {
		return "", fmt.Errorf("%s: %w", t, err)
	}
	return template.HTML(buf.String()), nil
}

func (t *Template) mergeImports(dest map[string]interface{}, imports []*manifest.Import) error {
	if dest == nil {
		return errors.New("destination map must be supplied")
	}
	for _, item := range imports {
		if item.Name == "" {
			continue
		}
		var resources []*Resource
		for _, item := range item.Manifests {
			resource, err := t.newStub(item, nil)
			if err != nil {
				return err
			}
			resources = append(resources, resource)
		}
		dest[item.Name] = t.templateFn(item, resources)
	}
	return nil
}

func (t *Template) templateFn(config *manifest.Import, imports []*Resource) interface{} {
	if !config.Single {
		return func() []*Resource {
			return imports
		}
	}
	if config.IsTemplate {
		return func(context interface{}) (template.HTML, error) {
			if len(imports) == 0 {
				return "", fmt.Errorf("%s not found", config.Name)
			}
			tmpl, err := NewTemplate(imports[0])
			if err != nil {
				return "", err
			}
			return tmpl.body(context, "")
		}
	}
	return func() (*Resource, error) {
		if len(imports) == 0 {
			return nil, fmt.Errorf("%s not found", config.Name)
		}
		return imports[0], nil
	}
}

// gross
func merge(dest map[string]interface{}, source map[string]interface{}) {
	for key, value := range source {
		dest[key] = value
	}
}

func ordinal(x int) string {
	suffix := "th"
	switch x % 10 {
	case 1:
		if x%100 != 11 {
			suffix = "st"
		}
	case 2:
		if x%100 != 12 {
			suffix = "nd"
		}
	case 3:
		if x%100 != 13 {
			suffix = "rd"
		}
	}
	return strconv.Itoa(x) + suffix
}
