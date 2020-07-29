package selector_test

import (
	"fmt"
	json "github.com/json-iterator/go"
	"github.com/tkellen/aevitas/internal/selector"
	"testing"
)

func TestNewSelector(t *testing.T) {
	table := map[string]bool{
		"invalid////": true,
		"/invalid///": true,
		"//invalid//": true,
		"///invalid/": true,
		"////invalid": true,
		"/////":       true,
		"k/g/v/ns/n":  false,
		"k/g/v/ns/*":  false,
	}
	for input, expectedErr := range table {
		input, expectedErr := input, expectedErr
		t.Run(input, func(t *testing.T) {
			_, err := selector.New(input)
			if err != nil && !expectedErr {
				t.Fatalf("expected no error, saw %s", err)
			}
			if err == nil && expectedErr {
				t.Fatal("expected error")
			}
		})
	}
}

func TestSelector_ID(t *testing.T) {
	type testCase struct {
		selector *selector.Selector
		expected string
	}
	table := []testCase{
		{
			selector: selector.Must("k/g/v/ns/n"),
			expected: "k/g/v/ns/n",
		},
		{
			selector: &selector.Selector{},
			expected: "",
		},
	}
	for _, test := range table {
		test := test
		t.Run(test.expected, func(t *testing.T) {
			actual := test.selector.ID()
			if test.expected != actual {
				t.Fatalf("expected %s, got %s", test.expected, actual)
			}
		})
	}
}

func TestSelector_String(t *testing.T) {
	type testCase struct {
		selector *selector.Selector
		expected string
	}
	table := []testCase{
		{
			selector: selector.Must("k/g/v/ns/n"),
			expected: "k/g/v/ns/n",
		},
		{
			selector: &selector.Selector{},
			expected: "",
		},
	}
	for _, test := range table {
		test := test
		t.Run(test.expected, func(t *testing.T) {
			actual := test.selector.String()
			if test.expected != actual {
				t.Fatalf("expected %s, got %s", test.expected, actual)
			}
		})
	}
}

func TestSelector_KGV(t *testing.T) {
	type testCase struct {
		selector *selector.Selector
		expected string
	}
	table := []testCase{
		{
			selector: selector.Must("k/g/v/ns/n"),
			expected: "k/g/v",
		},
	}
	for _, test := range table {
		test := test
		t.Run(test.expected, func(t *testing.T) {
			actual := test.selector.KGV()
			if test.expected != actual {
				t.Fatalf("expected %s, got %s", test.expected, actual)
			}
		})
	}
}

func TestSelector_KGVN(t *testing.T) {
	type testCase struct {
		selector *selector.Selector
		expected string
	}
	table := []testCase{
		{
			selector: selector.Must("k/g/v/ns/n"),
			expected: "k/g/v/ns",
		},
	}
	for _, test := range table {
		test := test
		t.Run(test.expected, func(t *testing.T) {
			actual := test.selector.KGVN()
			if test.expected != actual {
				t.Fatalf("expected %s, got %s", test.expected, actual)
			}
		})
	}
}

func TestSelector_NameIsWildcard(t *testing.T) {
	type testCase struct {
		selector *selector.Selector
		expected bool
	}
	table := []testCase{
		{
			selector: selector.Must("k/g/v/ns/n"),
			expected: false,
		},
		{
			selector: selector.Must("k/g/v/ns/*"),
			expected: true,
		},
	}
	for _, test := range table {
		test := test
		t.Run(test.selector.String(), func(t *testing.T) {
			actual := test.selector.IsWildcard()
			if test.expected != actual {
				t.Fatalf("expected %v, got %v", test.expected, actual)
			}
		})
	}
}

func TestSelector_Matches(t *testing.T) {
	type testCase struct {
		a        *selector.Selector
		b        *selector.Selector
		expected bool
	}
	table := []testCase{
		{
			a:        selector.Must("k/g/v/ns/n"),
			b:        selector.Must("k/g/v/ns/n"),
			expected: true,
		},
		{
			a:        &selector.Selector{},
			b:        &selector.Selector{},
			expected: true,
		},
		{
			a:        selector.Must("k/g/v/ns/*"),
			b:        selector.Must("k/g/v/ns/n"),
			expected: true,
		},
		{
			a:        selector.Must("k/g/v/ns/*"),
			b:        selector.Must("k/g/v/test/n"),
			expected: false,
		},
	}
	for _, test := range table {
		test := test
		t.Run(fmt.Sprintf("%s vs %s", test.a, test.b), func(t *testing.T) {
			actual := test.a.Matches(test.b)
			if test.expected != actual {
				t.Fatalf("expected %v, got %v", test.expected, actual)
			}
			commutativeActual := test.b.Matches(test.a)
			if test.expected != commutativeActual {
				t.Fatalf("expected %v, got %v", test.expected, actual)
			}
		})
	}
}

func TestSelector_UnmarshalJSON(t *testing.T) {
	type testCase struct {
		input    string
		expected *selector.Selector
	}
	table := []testCase{
		{input: "invalid////"},
		{input: "/invalid///"},
		{input: "//invalid//"},
		{input: "///invalid/"},
		{input: "////invalid"},
		{input: "/////"},
		{
			input:    "k/g/v/ns/n",
			expected: selector.Must("k/g/v/ns/n"),
		},
	}
	type testStruct struct {
		Selector *selector.Selector
	}
	var temp testStruct
	if err := json.Unmarshal([]byte(`{"selector"`), &temp); err == nil {
		t.Fatal("expected error on bad json")
	}

	for _, test := range table {
		test := test
		t.Run(test.input, func(t *testing.T) {
			var result testStruct
			err := json.Unmarshal([]byte(fmt.Sprintf(`{"selector":"%s"}`, test.input)), &result)
			if err != nil && test.expected != nil {
				t.Fatal("did not expect error")
			}
			if err == nil {
				if *test.expected != *result.Selector {
					t.Fatalf("expected %v, got %v", test.expected, result.Selector)
				}
			}
		})
	}
}
