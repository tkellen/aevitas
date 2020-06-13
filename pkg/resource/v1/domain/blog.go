package domain

import (
	"bytes"
	"context"
	"fmt"
	"github.com/go-git/go-billy/v5"
	json "github.com/json-iterator/go"
	"github.com/tkellen/aevitas/pkg/resource/v1"
	"html/template"
	"os"
	"path/filepath"
)

type Blog struct {
	Resource *resource.Resource
	Spec     BlogSpec
}

func NewBlog(r *resource.Resource) (*Blog, error) {
	instance := &Blog{Resource: r}
	if err := json.Unmarshal(r.Spec, &instance.Spec); err != nil {
		return nil, err
	}
	if err := instance.Validate(); err != nil {
		return nil, fmt.Errorf("%s\n%w", r.Spec, err)
	}
	return instance, nil
}

func (b *Blog) Validate() error {
	return b.Spec.Website.Validate()
}

func (b *Blog) Current(_ billy.Filesystem) bool {
	return false
}

func (b *Blog) Scope(fs billy.Filesystem) (billy.Filesystem, error) {
	return fs.Chroot(filepath.Join("goinglowly.com"))
}

func (b *Blog) Content(ctx context.Context) ([]byte, error) { return b.Resource.Bytes(ctx) }

func (b *Blog) Render(ctx context.Context, fs billy.Filesystem, root *template.Template) error {
	for _, pages := range b.Spec.Website.Pages {
		selector, _ := resource.NewSelector(pages.Selector)
		for _, template := range root.Templates() {
			if selector.Matches(template.Name()) {
				var buf bytes.Buffer
				if err := template.Execute(&buf, nil); err != nil {
					return err
				}
				fmt.Fprintf(os.Stdout, "%s\n", buf.String())
			}
		}
	}
	return nil
}

type BlogSpec struct {
	Website  websiteSpec
	Topics   []TopicSpec
	Sections []SectionSpec
}

type ResourceMap struct {
	Name string
	GVK  string
}

type TopicSpec struct {
	Name          string
	Slug          string
	TitleFragment string
	Description   string
	Attributes    []string
}

type SectionSpec struct {
	Name string
	Slug string
}
