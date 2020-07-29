package resource_test

import (
	"github.com/tkellen/aevitas/pkg/manifest"
	"github.com/tkellen/aevitas/pkg/resource"
	"reflect"
	"testing"
)

/*
func testIndex(t *testing.T) *resource.Tree {
	list, err := manifest.NewFromDirs([]string{"../../example/website","../../example/templates"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	index := manifest.NewIndex()
	if err := index.Insert(list...); err != nil {
		t.Fatal(err)
	}
	if err := index.Collate(); err != nil {
		t.Fatal(err)
	}
	inputRoot := osfs.New("../../example/asset")
	outputRoot := memfs.New()
	factory := resource.DefaultFactory(inputRoot, outputRoot)
	tree, rsErr := resource.NewTree("website/page/v1/domain/blog", index, factory)
	if rsErr != nil {
		t.Fatal(err)
	}
	return tree
}*/

func testResources() map[string]*resource.Resource {
	post := &resource.Resource{
		Manifest: &manifest.Manifest{
			Meta: &manifest.Meta{
				Href: "post.html",
				Title: "Post",
			},
		},
	}
	topic := &resource.Resource{
		Manifest: &manifest.Manifest{
			Meta: &manifest.Meta{
				HrefBase: "/topic/testing",
				Href: "index.html",
				TitleBase: "Testing",
				Title: "Testing",
			},
		},
		children: []*resource.Resource{post},
	}
	post.Parent = topic
	collection := &resource.Resource{
		Manifest: &manifest.Manifest{
			Meta: &manifest.Meta{
				Href: "/topic/index.html",
				Title: "Topics",
			},
		},
		children: []*resource.Resource{topic},
	}
	topic.Parent = collection
	domain := &resource.Resource{
		Manifest: &manifest.Manifest{
			Meta: &manifest.Meta{
				HrefBase: "/",
				Href: "index.html",
				TitleBase: "Domain",
				Title: "Home",
			},
		},
		children: []*resource.Resource{collection},
	}
	collection.Parent = domain
	return map[string]*resource.Resource{
		"post": post,
		"topic": topic,
		"collection": collection,
		"domain": domain,
	}
}

func TestResource_Parents(t *testing.T) {
	r := testResources()
	table := map[string]struct{
		resource *resource.Resource
		expected []*resource.Resource
	}{
		"post": {
			resource: r["post"],
			expected: []*resource.Resource{r["topic"], r["collection"], r["domain"]},
		},
		"topic": {
			resource: r["topic"],
			expected: []*resource.Resource{r["collection"], r["domain"]},
		},
		"collection": {
			resource: r["collection"],
			expected: []*resource.Resource{r["domain"]},
		},
		"domain": {
			resource: r["domain"],
			expected: []*resource.Resource{},
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			actual := test.resource.Parents()
			if len(test.expected) == 0 && len(actual) == 0 {
				return
			}
			if !reflect.DeepEqual(test.expected, actual) {
				t.Fatalf("expected %s got %s", test.expected, actual)
			}
		})
	}
}

func TestResource_Titles(t *testing.T) {
	r := testResources()
	table := map[string]struct{
		resource *resource.Resource
		expected []string
	}{
		"post": {
			resource: r["post"],
			expected: []string{"Post","Testing","Domain"},
		},
		"topic": {
			resource: r["topic"],
			expected: []string{"Testing","Domain"},
		},
		"collection": {
			resource: r["collection"],
			expected: []string{"Topics", "Domain"},
		},
		"domain": {
			resource: r["domain"],
			expected: []string{"Home Domain"},
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			actual := test.resource.Titles()
			if len(test.expected) == 0 && len(actual) == 0 {
				return
			}
			if !reflect.DeepEqual(test.expected, actual) {
				t.Fatalf("expected %s got %s", test.expected, actual)
			}
		})
	}
}

func TestResource_Href(t *testing.T) {
	r := testResources()
	table := map[string]struct{
		resource *resource.Resource
		expected string
	}{
		"post": {
			resource: r["post"],
			expected: "/topic/testing/post.html",
		},
		"topic": {
			resource: r["topic"],
			expected: "/topic/testing/index.html",
		},
		"collection": {
			resource: r["collection"],
			expected: "/topic/index.html",
		},
		"domain": {
			resource: r["domain"],
			expected: "/index.html",
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			actual := test.resource.Href()
			if len(test.expected) == 0 && len(actual) == 0 {
				return
			}
			if !reflect.DeepEqual(test.expected, actual) {
				t.Fatalf("expected %s got %s", test.expected, actual)
			}
		})
	}
}