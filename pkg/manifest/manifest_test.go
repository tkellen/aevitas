package manifest_test

import (
	"bytes"
	"github.com/tkellen/aevitas/internal/selector"
	"github.com/tkellen/aevitas/pkg/manifest"
	"reflect"
	"testing"
)

func TestNew(t *testing.T) {
	type testCase struct {
		input             []byte
		expectedSelector  *selector.Selector
		expectedMeta      *manifest.Meta
		expectedRelations []*manifest.Relation
		expectedRender    *manifest.Render
		expectedErr       bool
	}
	expectedSelector := selector.Must("k/g/v/ns/n")
	expectedMeta := &manifest.Meta{
		File:      "test",
		HrefBase:  "/",
		Href:      "test.html",
		TitleBase: "base",
		Title:     "title",
	}
	expectedRelations := []*manifest.Relation{{
		Selector: selector.Must("a/b/c/d/e"),
	}}
	expectedRender := &manifest.Render{
		Children: []*manifest.Child{{
			Relation: manifest.Relation{
				Selector: selector.Must("e/d/c/b/a"),
			},
			Templates: []*selector.Selector{selector.Must("f/g/h/i/j")},
		}},
	}
	table := map[string]testCase{
		"from json": {
			input:             []byte(`{"kind":"k","group":"g","version":"v","namespace":"ns","name":"n","meta":{"file":"test","hrefBase":"/","href":"test.html","titleBase":"base","title":"title"},"relations":[{"selector":"a/b/c/d/e"}],"render":{"children":[{"selector":"e/d/c/b/a","templates":["f/g/h/i/j"]}]}}`),
			expectedSelector:  expectedSelector,
			expectedMeta:      expectedMeta,
			expectedRelations: expectedRelations,
			expectedRender:    expectedRender,
			expectedErr:       false,
		},
		"with yaml as frontmatter": {
			input:             []byte("---\nkind: k\ngroup: g\nversion: v\nnamespace: ns\nname: \"n\"\nmeta:\n  file: test\n  hrefBase: /\n  href: test.html\n  titleBase: base\n  title: title\nrelations:\n  - selector: a/b/c/d/e\nrender:\n  children:\n  - selector: e/d/c/b/a\n    templates: [f/g/h/i/j]\n\n---\ncontent"),
			expectedSelector:  expectedSelector,
			expectedMeta:      expectedMeta,
			expectedRelations: expectedRelations,
			expectedRender:    expectedRender,
			expectedErr:       false,
		},
		"with invalid yaml as frontmatter": {
			input:       []byte("---\n}::: BAD :::{\n---\ncontent"),
			expectedErr: true,
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			m, err := manifest.New(test.input, "test")
			if test.expectedErr && err == nil {
				t.Fatalf("expected error, got none")
			}
			if !test.expectedErr && err != nil {
				t.Fatalf("unexpected err %s", err)
			}
			if err == nil {
				if len(m) != 1 {
					t.Fatal("expected 1 manifest")
				}
				actual := m[0]
				if !reflect.DeepEqual(test.expectedSelector, m[0].Selector) {
					t.Fatalf("expected selector %s, got %s", test.expectedSelector, actual.Selector)
				}
				if !bytes.Equal(test.input, actual.Raw) {
					t.Fatalf("expected %s %s, got %s", name, test.input, actual.Raw)
				}
				if !reflect.DeepEqual(test.expectedMeta, actual.Meta) {
					t.Fatalf("expected %#v, got %#v", test.expectedMeta, actual.Meta)
				}
				if !reflect.DeepEqual(test.expectedRelations, actual.Relations) {
					t.Fatalf("expected %#v, got %#v", test.expectedRelations, actual.Relations)
				}
				if !reflect.DeepEqual(test.expectedRender, actual.Render) {
					t.Fatalf("expected %#v, got %#v", test.expectedRender, actual.Render)
				}
			}
		})
	}
}

func TestManifest_EqualGreaterLess(t *testing.T) {
	first := &manifest.Manifest{Meta: &manifest.Meta{PublishAt: &manifest.PublishAt{
		Year: 2020, Month: 1, Day: 1,
	}}}
	sameAsFirst := &manifest.Manifest{Meta: &manifest.Meta{PublishAt: &manifest.PublishAt{
		Year: 2020, Month: 1, Day: 1,
	}}}
	last := &manifest.Manifest{Meta: &manifest.Meta{PublishAt: &manifest.PublishAt{
		Year: 2020, Month: 1, Day: 3,
	}}}
	if !first.Equal(sameAsFirst) {
		t.Fatal("expected first and sameAsFirst would be equal")
	}
	if first.Equal(last) {
		t.Fatal("expected first and last would not be equal")
	}
	if !first.Less(last) {
		t.Fatal("expected first to be less than last")
	}
	if last.Less(first) {
		t.Fatal("did not expect last to be less then first")
	}
	if !last.Greater(first) {
		t.Fatal("expected last to be greater than first")
	}
	if first.Greater(last) {
		t.Fatal("did not expect first to be greater than last")
	}
}

func TestManifest_Title(t *testing.T) {
	table := map[string]struct {
		resource *manifest.Manifest
		expected string
	}{
		"both empty": {
			resource: &manifest.Manifest{
				Meta: &manifest.Meta{
					TitleBase: "",
					Title:     "",
				},
			},
			expected: "",
		},
		"base has value, title empty": {
			resource: &manifest.Manifest{
				Meta: &manifest.Meta{
					TitleBase: "base",
					Title:     "",
				},
			},
			expected: "base",
		},
		"base empty, title has value": {
			resource: &manifest.Manifest{
				Meta: &manifest.Meta{
					TitleBase: "",
					Title:     "title",
				},
			},
			expected: "title",
		},
		"base has value, title has value": {
			resource: &manifest.Manifest{
				Meta: &manifest.Meta{
					TitleBase: "base",
					Title:     "title",
				},
			},
			expected: "title base",
		},
		"base and title have same value": {
			resource: &manifest.Manifest{
				Meta: &manifest.Meta{
					TitleBase: "equal",
					Title:     "equal",
				},
			},
			expected: "equal",
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			actual := test.resource.Title()
			if test.expected != actual {
				t.Fatalf("expected %s got %s", test.expected, actual)
			}
		})
	}
}