package domain

import (
	"context"
	"fmt"
	"github.com/go-git/go-billy/v5"
	json "github.com/json-iterator/go"
	metav1 "github.com/tkellen/aevitas/pkg/resource/meta/v1"
	"path/filepath"
)

type Blog struct {
	Meta metav1.Meta
	Spec BlogSpec
}

func NewBlog(manifest []byte) (*Blog, error) {
	var instance Blog
	if err := json.Unmarshal(manifest, &instance); err != nil {
		return nil, err
	}
	if err := instance.Validate(); err != nil {
		return nil, fmt.Errorf("%s\n%w", manifest, err)
	}
	return &instance, nil
}

func (b *Blog) Validate() error {
	return b.Spec.Website.Validate()
}

func (b *Blog) Deps(_ context.Context) ([]string, error) {
	return b.Spec.Entries, nil
}

func (b *Blog) Current(fs billy.Filesystem) bool {
	return false
}

func (b *Blog) Scope(fs billy.Filesystem) (billy.Filesystem, error) {
	return fs.Chroot(filepath.Join("goinglowly.com"))
}

func (b *Blog) Render(ctx context.Context, fs billy.Filesystem) error {
	return nil
}

type BlogSpec struct {
	Website  websiteSpec
	Entries  []string
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
