package website

import (
	"context"
	"fmt"
	json "github.com/json-iterator/go"
	"github.com/tkellen/aevitas/pkg/resource"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

const KGVDomain = "website/domain/v1"

type Domain struct {
	resource *resource.Resource
	Spec     DomainSpec
}

type DomainSpec struct {
	Title       string
	Description string
	Author      string
	Pages       []string
	Host        string
}

type SectionSpec struct {
	Name string
	Slug string
}

func NewDomain(r *resource.Resource) (*Domain, error) {
	instance := &Domain{resource: r}
	if err := json.Unmarshal(r.Spec, &instance.Spec); err != nil {
		return nil, err
	}
	if err := instance.Validate(); err != nil {
		return nil, fmt.Errorf("%s\n%w", r.Spec, err)
	}
	return instance, nil
}

func (d *Domain) Validate() error {
	if d.Spec.Title == "" {
		return fmt.Errorf("title must be defined")
	}
	if d.Spec.Description == "" {
		return fmt.Errorf("description must be defined")
	}
	if d.Spec.Author == "" {
		return fmt.Errorf("author must be defined")
	}
	return nil
}

func (d *Domain) Render(ctx context.Context, r resource.Element) error {
	eg, egCtx := errgroup.WithContext(ctx)
	sem := semaphore.NewWeighted(10)
	for _, dep := range r.Imports() {
		dep := dep
		eg.Go(func() error {
			if err := sem.Acquire(egCtx, 1); err != nil {
				return err
			}
			eg.Go(func() error {
				defer sem.Release(1)
				for _, item := range d.Spec.Pages {
					if dep.Resource().Matches(item) {
						return dep.Instance().Render(ctx, dep)
					}
				}
				return nil
			})
			return nil
		})
	}
	return nil
}
