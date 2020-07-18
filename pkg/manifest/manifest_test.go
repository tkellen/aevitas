package manifest_test

import (
	"bytes"
	"github.com/google/go-cmp/cmp"
	"github.com/tkellen/aevitas/pkg/manifest"
	"reflect"
	"testing"
)

func TestNew(t *testing.T) {
	type testCase struct {
		input            []byte
		expectedSelector *manifest.Selector
		expectedMeta     *manifest.Meta
		expectedErr      bool
	}
	expectedSelector := &manifest.Selector{Kind: "k", Group: "g", Version: "v", Namespace: "ns", Name: "n"}
	expectedMeta := &manifest.Meta{
		File:    "test",
		Related: []*manifest.Selector{{Kind: "a", Group: "b", Version: "c", Namespace: "d", Name: "e"}},
		Include: []*manifest.Include{{
			Resource:  &manifest.Selector{Kind: "e", Group: "d", Version: "c", Namespace: "b", Name: "a"},
			Templates: []*manifest.Selector{{Kind: "f", Group: "g", Version: "h", Namespace: "i", Name: "j"}},
			As:        "test",
		}},
	}
	table := map[string]testCase{
		"from json": {
			input:            []byte(`{"kind":"k","group":"g","version":"v","namespace":"ns","name":"n","meta":{"file":"test","related":["a/b/c/d/e"],"include":[{"resource":"e/d/c/b/a","templates":["f/g/h/i/j"],"as":"test"}]}}`),
			expectedSelector: expectedSelector,
			expectedMeta:     expectedMeta,
			expectedErr:      false,
		},
		"with yaml as frontmatter": {
			input:            []byte("---\nkind: k\ngroup: g\nversion: v\nnamespace: ns\nname: \"n\"\nmeta:\n  file: test\n  related:\n  - a/b/c/d/e\n  include:\n    - resource: e/d/c/b/a\n      templates: [f/g/h/i/j]\n      as: test\n---\ncontent"),
			expectedSelector: expectedSelector,
			expectedMeta:     expectedMeta,
			expectedErr:      false,
		},
		"with invalid yaml as frontmatter": {
			input:       []byte("---\n}::: BAD :::{\n---\ncontent"),
			expectedErr: true,
		},
		"with invalid wildcard on include": {
			input:            []byte(`{"kind":"k","group":"g","version":"v","namespace":"ns","name":"n","meta":{"include":[{"resource":"a/b/c/d/e", "templates":["a/b/c/d/*"]}]}}`),
			expectedSelector: expectedSelector,
			expectedErr:      true,
		},
		"with invalid wildcard resource and alias": {
			input:            []byte(`{"kind":"k","group":"g","version":"v","namespace":"ns","name":"n","meta":{"include":[{"resource":"a/b/c/d/*", "as":"alias"}]}}`),
			expectedSelector: expectedSelector,
			expectedErr:      true,
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			m, err := manifest.New(test.input)
			if test.expectedErr && err == nil {
				t.Fatalf("expected error, got none")
			}
			if !test.expectedErr && err != nil {
				t.Fatalf("unexpected err %s", err)
			}
			if err == nil {
				if !reflect.DeepEqual(test.expectedSelector, m.Selector) {
					t.Fatalf("expected selector %s, got %s", test.expectedSelector, m.Selector)
				}
				if !bytes.Equal(test.input, m.Raw) {
					t.Fatalf("expected %s %s, got %s", name, test.input, m.Raw)
				}
				if diff := cmp.Diff(test.expectedMeta, m.Meta); diff != "" {
					t.Fatal(diff)
				}
			}
		})
	}
}

func TestNewFromDirectory(t *testing.T) {
	list, err := manifest.NewFromDirectory("../../example/templates")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 8 {
		t.Fatal("expected 8 items, this test is junk.")
	}
}

func TestNewFromReader(t *testing.T) {
	list, err := manifest.NewFromReader(bytes.NewReader([]byte(`
      {"kind":"k","group":"g","version":"v","namespace":"ns","name":"first"}
	  {"kind":"k","group":"g","version":"v","namespace":"ns","name":"second"}
    `)))
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatal("expected 2 items, this test is junk")
	}
}
