package manifest_test

import (
	"github.com/tkellen/aevitas/pkg/manifest"
	"testing"
)

func TestNewFromGenerator(t *testing.T) {
	generator := &manifest.Generator{
		Name:      "test",
		RangeOver: [2]int{0, 365},
		Context: map[string]interface{}{
			"monthRollover": []int{31, 60, 91, 121, 152, 182, 213, 244, 274, 305, 335, 366},
		},
		Template: `{{- $month := 1 -}}
{{- $day := (add count 1) -}}
{{- range $idx, $max := .monthRollover -}}
  {{- if ge count ($max | int) -}}
    {{- $month = (add $idx 2) -}}
    {{- $day = (add (sub count $max) 1) -}}
  {{- end -}}
{{- end -}}
{
  "kind": "website",
  "group": "content",
  "version": "v1",
  "namespace": "day",
  "name": "{{ $month }}-{{ $day }}",
  "meta": {
    "live": true,
    "baseHref": "/{{ printf "%02d" $month }}/{{ printf "%02d" $day }}",
    "renderTemplates": [
      "html/template/v1/related/layout",
      "html/template/v1/default/layout"
    ],
    "relations": [{
      "selector": "website/content/v1/post/*",
      "navigationScopedByParent": true,
      "matchExpression": [
        { "key": "meta.publishAt.month", "operator": "In", "values": [{{ $month }}] },
        { "key": "meta.publishAt.day", "operator": "In", "values": [{{ $day }}] }
      ]
    }],
    "renderAsChild": [{
      "selector": "website/content/v1/post/*",
      "matchIfRelatedTo": [
        "website/content/v1/day/{{ $month }}-{{ $day }}"
      ],
      "navigationScopedByParent": true,
      "renderTemplates": [
        "html/template/v1/post/layout",
        "html/template/v1/post/scoped",
        "html/template/v1/default/layout"
      ]
    }]
  },
  "spec": {
    "title": "{{ $month }}-{{ $day }}",
    "titleFragment": "Posts made on {{ $month }} / {{ $day }}",
    "body": "The {{ $day }} day of the {{ $month }} month."
  }
}`,
	}
	manifests, err := manifest.NewFromGenerator(generator, &manifest.Manifest{
		Selector: manifest.NewSelectorMust("k/g/v1/ns/n"),
	})
	expected := 366
	actual := len(manifests)
	if expected != actual {
		t.Fatalf("expected %d manifests, got %d", expected, actual)
	}
	if err != nil {
		t.Fatal(err)
	}
}
