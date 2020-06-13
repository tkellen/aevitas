package text

import (
	"bytes"
	"fmt"
	"github.com/go-git/go-billy/v5"
	json "github.com/json-iterator/go"
	"github.com/tkellen/aevitas/pkg/resource/v1"
	"html/template"
)

type HTML struct {
	Resource *resource.Resource
	Spec     *HTMLSpec
}

type HTMLSpec struct {
	Body string
}

func NewHTML(r *resource.Resource) (*HTML, error) {
	instance := &HTML{Resource: r}
	if err := json.Unmarshal(r.Spec, &instance.Spec); err != nil {
		return nil, err
	}
	if err := instance.Validate(); err != nil {
		return nil, fmt.Errorf("%s\n%w", r.Spec, err)
	}
	return instance, nil
}

func (h *HTML) Validate() error { return nil }
func (h *HTML) Scope(fs billy.Filesystem) (billy.Filesystem, error) { return fs, nil }
func (h *HTML) Current(fs billy.Filesystem) bool { return false }
func (h *HTML) Render(root *template.Template, name string, context interface{}) error {
	var err error
	temp, cloneErr := root.Clone()
	if cloneErr != nil {
		return cloneErr
	}
	if asText, ok := context.(*HTML); ok {
		if temp, err = temp.New("test").Parse(asText.Spec.Body); err != nil {
			return err
		}
	}
	if temp, err = temp.Parse(h.Spec.Body); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := temp.Execute(&buf, context); err != nil {
		return err
	}
	_, err = root.New(name).Parse(buf.String())
	return err
}
