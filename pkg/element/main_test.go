package element_test

import (
	"context"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/tkellen/aevitas/pkg/element"
	"github.com/tkellen/aevitas/pkg/resource"
	"github.com/tkellen/aevitas/pkg/selector"
	"testing"
)

func getSelector(t *testing.T, target string) selector.Selector {
	selector, err := selector.New(target)
	if err != nil {
		t.Fatal(err)
	}
	return selector
}

func TestElement_Render(t *testing.T) {
	headTmpl := &resource.Resource{
		Selector: getSelector(t, "ns/html/fragment/v1/head"),
		Spec:     []byte(`{"body":"<head><title>{{ .Spec.Title }}</title></head>"}`),
	}
	pageTmpl := &resource.Resource{
		Selector: getSelector(t, "ns/html/fragment/v1/page"),
		Imports: []resource.Import{
			{Resource: getSelector(t, "ns/html/fragment/v1/head"), As: "head"},
		},
		Spec: []byte(`{"body":"<!DOCTYPE html><html>{{ template \"head\" .Instance }}<body>{{ .Body }}</body></html>"}`),
	}
	responsiveImageTmpl := &resource.Resource{
		Selector: getSelector(t, "ns/html/fragment/v1/responsive-image"),
		Spec:     []byte(`{"body":"<img src=\"{{.Instance.Spec.Path}}/{{index .Instance.Spec.Widths 0}}\" srcset=\"{{range .Instance.Spec.Widths}}{{$.Instance.Spec.Path}}/{{.}} {{.}}w {{end}}\" title=\"{{.Instance.Spec.Title}}\"/>"}`),
	}
	firstImage := &resource.Resource{
		Selector: getSelector(t, "ns/image/jpeg/v1/first-image"),
		Spec:     []byte(`{"title":"nun","widths":[100,200,300],"path":"nun"}`),
		Meta: resource.Meta{
			File: "image.jpg",
		},
	}
	secondImage := &resource.Resource{
		Selector: getSelector(t, "ns/image/jpeg/v1/second-image"),
		Spec:     []byte(`{"title":"second-nun","widths":[100,200,300],"path":"second-nun"}`),
		Meta: resource.Meta{
			File: "image.jpg",
		},
	}
	firstWebsitePage := &resource.Resource{
		Selector: getSelector(t, "ns/website/page/v1/test-page-one"),
		Imports: []resource.Import{
			{
				Resource: getSelector(t, "ns/image/jpeg/v1/first-image"),
				Template: getSelector(t, "ns/html/fragment/v1/responsive-image"),
				As:       "image",
			},
		},
		Spec: []byte(`{"path":"/first","title":"first","body":"{{ template \"image\" }}"}`),
	}
	secondWebsitePage := &resource.Resource{
		Selector: getSelector(t, "ns/website/page/v1/test-page-two"),
		Imports: []resource.Import{
			{
				Resource: getSelector(t, "ns/image/jpeg/v1/second-image"),
				Template: getSelector(t, "ns/html/fragment/v1/responsive-image"),
				As:       "image",
			},
			{
				Resource: getSelector(t, "ns/image/jpeg/v1/second-image"),
				Template: getSelector(t, "ns/html/fragment/v1/responsive-image"),
				As:       "image-alt",
			},
		},
		Spec: []byte(`{"path":"/second","title":"second","body":"{{ template \"image\" }}{{ template \"image-alt\" }}"}`),
	}
	thirdWebsitePage := &resource.Resource{
		Selector: getSelector(t, "ns/website/page/v1/test-page-three"),
		Imports: []resource.Import{
			{
				Resource: getSelector(t, "ns/image/jpeg/v1/second-image"),
				Template: getSelector(t, "ns/html/fragment/v1/responsive-image"),
				As:       "imagey",
			},
		},
		Spec: []byte(`{"path":"/third","title":"third","body":"hello {{ template \"imagey\" }}"}`),
	}
	websiteDomain := &resource.Resource{
		Selector: getSelector(t, "ns/website/domain/v1/test"),
		Imports: []resource.Import{
			{
				Resource: getSelector(t, "ns/website/page/v1/*"),
				Template: getSelector(t, "ns/html/fragment/v1/page"),
			},
		},
		Spec: []byte(`{"host":"test.com","title":"test","author":"test","description":"test","pages":["ns/website/page/v1/*"]}`),
	}
	fixtures := resource.List{
		headTmpl,
		pageTmpl,
		responsiveImageTmpl,
		firstImage,
		secondImage,
		firstWebsitePage,
		secondWebsitePage,
		thirdWebsitePage,
		websiteDomain,
	}
	source := osfs.New("../../testdata")
	dest := osfs.New("../../testdata/build")
	index := fixtures.Indexed()
	el, newErr := element.New("", websiteDomain.Selector, selector.Selector{}, index, source, dest)
	if newErr != nil {
		t.Fatal(newErr)
	}
	if err := el.Render(context.Background()); err != nil {
		t.Fatal(err)
	}
}
