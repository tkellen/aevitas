package manifest_test

/*
func TestGenerator_Generate(t *testing.T) {
	generator := &manifest.Generator{
		Name: "test",
		Loops: []manifest.GeneratorRange{
			{Name: "years", Range: [2]int{2008, 2020}},
			{Name: "months", Range: [2]int{1, 12}},
			{Name: "days", Range: [2]int{1, 31}},
		},
	}
	manifests, err := generator.Generate(&manifest.Manifest{
		Selector: selector.Must("kind/group/version/ns/name"),
	})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintf(os.Stdout, "%s", manifests)
}

func TestNewFromGenerator(t *testing.T) {
	generator := &manifest.Generator{
		Name:      "test",
		RangeOver: [2]int{0, 365},
		Context: map[string]interface{}{
			"monthRollover": []int{31, 60, 91, 121, 152, 182, 213, 244, 274, 305, 335, 366},
		},
		Template: `{{- $month := 1 -}}
{{- $day := (add idx 1) -}}
{{- range $idx, $max := .monthRollover -}}
  {{- if ge idx ($max | int) -}}
    {{- $month = (add $idx 2) -}}
    {{- $day = (add (sub idx $max) 1) -}}
  {{- end -}}
{{- end -}}
{
  "kind": "website",
  "group": "page",
  "version": "v1",
  "namespace": "day",
  "name": "{{ $month }}-{{ $day }}",
  "meta": {
    "live": true,
    "hrefBase": "/{{ printf "%02d" $month }}/{{ printf "%02d" $day }}",
    "templates": [
      "html/template/v1/post/related-layout",
      "html/template/v1/default/layout"
    ],
    "relations": [{
      "selector": "website/page/v1/post/*",
      "navigationScopedByParent": true,
      "matchExpression": [
        { "key": "meta.publishAt.month", "operator": "In", "values": [{{ $month }}] },
        { "key": "meta.publishAt.day", "operator": "In", "values": [{{ $day }}] }
      ]
    }],
    "renderAsChild": [{
      "selector": "website/page/v1/post/*",
      "matchIfRelatedTo": [
        "website/page/v1/day/{{ $month }}-{{ $day }}"
      ],
      "navigationScopedByParent": true,
      "templates": [
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
*/
