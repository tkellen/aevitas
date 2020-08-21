package manifest_test

import (
	"fmt"
	"github.com/tkellen/aevitas/internal/selector"
	"github.com/tkellen/aevitas/pkg/manifest"
	"math/big"
	"math/rand"
	"moul.io/number-to-words"
	"strings"
	"testing"
	"time"
)

func asWord(number int) string {
	return strings.Replace(ntw.IntegerToEnUs(number), " ", "-", -1)
}

func generateManifests(count int) []*manifest.Manifest {
	result := make([]*manifest.Manifest, count)
	for idx := 0; idx < count; idx++ {
		// increment by one day for each entry
		year, month, day := time.Unix(int64(idx+1)*86400, 0).Date()
		manifest := &manifest.Manifest{
			Selector: selector.Must(fmt.Sprintf("test/number/v1/integer/%s", asWord(idx))),
			Meta: &manifest.Meta{
				Live: true,
				// Resource ordering within a given kind/group/version/namespace
				// is governed by publish date. Increment so manifests can be
				// validated to respect this.
				PublishAt: &manifest.PublishAt{Year: year, Month: int(month), Day: day},
				Relations: generateRelatedTo(idx),
				Imports:   []*manifest.Relation{},
			},
		}
		result[idx] = manifest
	}
	return result
}

func generateRelatedTo(number int) []*manifest.Relation {
	var relations []*manifest.Relation
	if number%2 == 0 {
		relations = append(relations, &manifest.Relation{
			Selector: selector.Must("test/number/v1/set/even"),
		})
	} else {
		relations = append(relations, &manifest.Relation{
			Selector: selector.Must("test/number/v1/set/odd"),
		})
	}
	if big.NewInt(int64(number)).ProbablyPrime(0) {
		relations = append(relations, &manifest.Relation{
			Selector: selector.Must("test/number/v1/set/prime"),
		})
	}
	return relations
}

func generateIndex(manifests []*manifest.Manifest) *manifest.Index {
	// Produce a shuffled copy of the supplied manifests.
	shuffled := append([]*manifest.Manifest{}, manifests...)
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(manifests), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})
	// Insert manifests into an index in a non-deterministic order.
	index := manifest.NewIndex()
	for _, manifest := range shuffled {
		if err := index.Insert(manifest); err != nil {
			panic(err)
		}
	}
	// Insert "sets" (to create relationships).
	if err := index.Insert(
		&manifest.Manifest{
			Selector: selector.Must("test/number/v1/set/prime"),
			Meta: &manifest.Meta{
				Live:      true,
				Relations: []*manifest.Relation{},
				Imports:   []*manifest.Relation{},
			},
		},
		&manifest.Manifest{
			Selector: selector.Must("test/number/v1/set/even"),
			Meta: &manifest.Meta{
				Live:      true,
				Relations: []*manifest.Relation{},
				Imports:   []*manifest.Relation{},
			},
		},
		&manifest.Manifest{
			Selector: selector.Must("test/number/v1/set/odd"),
			Meta: &manifest.Meta{
				Live:      true,
				Relations: []*manifest.Relation{},
				Imports:   []*manifest.Relation{},
			},
		},
	); err != nil {
		panic(err)
	}
	// Insert "oddball" item that is not in the kind/group/version/namespace
	// as numbers.
	if err := index.Insert(&manifest.Manifest{
		Selector: selector.Must("test/number/v1/oddball/item"),
		Meta: &manifest.Meta{
			Live:      true,
			Relations: []*manifest.Relation{},
			Imports:   []*manifest.Relation{},
		},
	}); err != nil {
		panic(err)
	}
	if err := index.Collate(); err != nil {
		panic(err)
	}
	return index
}

func TestIndex_navTimeious(t *testing.T) {
	// Generate numbers array of manifests.
	numbers := generateManifests(3)
	index := generateIndex(numbers)
	// Check next/previous for every number.
	for idx, number := range numbers {
		// Confirm previous manifest is found in right order.
		var expectedPrevious *manifest.Manifest
		if idx > 0 {
			expectedPrevious = numbers[idx-1]
		}
		actualPrevious := index.Prev(number)
		if expectedPrevious != actualPrevious {
			t.Fatalf("finding previous for %s, expected %s, got %s", number, expectedPrevious, actualPrevious)
		}
		// Confirm next manifest is found in right order.
		var expectedNext *manifest.Manifest
		if idx+1 < len(numbers) {
			expectedNext = numbers[idx+1]
		}
		actualNext := index.Next(number)
		if expectedNext != actualNext {
			t.Fatalf("finding next for %s, expected %s, got %s", number, expectedNext, actualNext)
		}
	}
	// Confirm that searching for next/previous on items that do not exist
	// returns nil (to provide coverage for the case when there is no KGVN shard
	// for the requested search).
	doesNotExist := &manifest.Manifest{
		Selector: selector.Must("nope/nope/nope/nope/nope"),
		Meta:     &manifest.Meta{Live: true},
	}
	nothingNext := index.Next(doesNotExist)
	if nothingNext != nil {
		t.Fatal("expected search for non-existent selector to produce nil response for next")
	}
	nothingPrevious := index.Prev(doesNotExist)
	if nothingPrevious != nil {
		t.Fatal("expected search for non-existent selector to produce nil response for previous")
	}
}

/*
func TestIndex_Relationships(t *testing.T) {
	numbers := generateManifests(1000)
	index := generateIndex(t, numbers)

	m := map[string]*manifest.Resource{
		"meat": {Resource: s["meat"], Meta: &manifest.Meta{
			Live: true,
			relations: []*manifest.Resource{
				s["taco"], // meat is also related to taco by way of the taco
				// including it. this helps assert that during the
				// aggregation of relations the same manifest is not
				// collected more than once.
			},
		}},
		"cheese": {Resource: s["cheese"], Meta: &manifest.Meta{
			Live: true,
			relations: []*manifest.Resource{},
		}},
		"tomato": {Resource: s["tomato"], Meta: &manifest.Meta{
			Live: true,
			relations: []*manifest.Resource{},
		}},
		"onion": {Resource: s["onion"], Meta: &manifest.Meta{
			Live: true,
			relations: []*manifest.Resource{},
		}},
		"noodles": {Resource: s["noodles"], Meta: &manifest.Meta{
			Live: true,
			relations: []*manifest.Resource{},
		}},
		"taco": {Resource: s["taco"], Meta: &manifest.Meta{
			Live: true,
			relations: []*manifest.Resource{},
			imports: map[string]*RenderTarget{
				"meat": {Resource: s["meat"]},
				"cheese": {Resource: s["cheese"]},
				"tomato": {Resource: s["tomato"]},
				"onion": {Resource: s["onion"]},
			},
		}},
		"spaghetti": {Resource: s["spaghetti"], Meta: &manifest.Meta{
			Live: true,
			relations: []*manifest.Resource{},
			imports: map[string]*RenderTarget{
				"meat": {Resource: s["meat"]},
				"tomato": {Resource: s["tomato"]},
				"onion": {Resource: s["onion"]},
				"noodles": {Resource: s["noodles"]},
			},
		}},
		"dishes": {Resource: s["dishes"], Meta: &manifest.Meta{
			Live: true,
			relations: []*manifest.Resource{s["taco"], s["spaghetti"]},
			imports: map[string]*RenderTarget{},
		}},
	}
	list := newShard()
	for _, manifest := range m {
		list.insert(manifest)
	}
	expected := Relations{
		Index: index(t, list.manifests),
		Relationships: map[*manifest.Resource]*Index{
			m["meat"]:      index(t, []*manifest.Resource{m["taco"], m["spaghetti"]}),
			m["cheese"]:    index(t, []*manifest.Resource{m["taco"]}),
			m["tomato"]:    index(t, []*manifest.Resource{m["taco"], m["spaghetti"]}),
			m["onion"]:     index(t, []*manifest.Resource{m["taco"], m["spaghetti"]}),
			m["noodles"]:   index(t, []*manifest.Resource{m["spaghetti"]}),
			m["taco"]:      index(t, []*manifest.Resource{m["meat"], m["cheese"], m["tomato"], m["onion"]}),
			m["spaghetti"]: index(t, []*manifest.Resource{m["meat"], m["tomato"], m["onion"], m["noodles"]}),
			m["dishes"]:    index(t, []*manifest.Resource{m["taco"], m["spaghetti"]}),
		},
	}
	actual, err := NewRelations(index(t, list.manifests))
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintf(os.Stdout, "ACTUAL[%s]", actual.String())
	fmt.Fprintf(os.Stdout, "EXPECTED[%s]", expected.String())

		if diff := cmp.Diff(expected.String(), actual.String()); diff != "" {
			t.Fatalf("expected\n%s\nactual\n%s", expected, actual)
		}

	s := map[string]*manifest.Resource{
		"kgv1aa": {kind: "k", group: "g", version: "v1", namespace: "a", name: "a"},
		"kgv1ab": {kind: "k", group: "g", version: "v1", namespace: "a", name: "b"},
		"kgv1ba": {kind: "k", group: "g", version: "v1", namespace: "b", name: "a"},
		"kgv1bb": {kind: "k", group: "g", version: "v1", namespace: "b", name: "b"},
		"kgv2aa": {kind: "k", group: "g", version: "v2", namespace: "a", name: "a"},
		"kgv2ab": {kind: "k", group: "g", version: "v2", namespace: "a", name: "b"},
		"kgv2ba": {kind: "k", group: "g", version: "v2", namespace: "b", name: "a"},
		"kgv2bb": {kind: "k", group: "g", version: "v2", namespace: "b", name: "b"},
	}
	m := map[string]*manifest.Resource{
		"kgv1aa": {Resource: s["kgv1aa"]},
		"kgv1ab": {Resource: s["kgv1ab"]},
		"kgv1ba": {Resource: s["kgv1ba"]},
		"kgv1bb": {Resource: s["kgv1bb"]},
		"kgv2aa": {Resource: s["kgv2aa"]},
		"kgv2ab": {Resource: s["kgv2ab"]},
		"kgv2ba": {Resource: s["kgv2ba"]},
		"kgv2bb": {Resource: s["kgv2bb"]},
	}
	index := manifest.NewIndex()
	for _, item := range m {
		index.Insert(item)
	}
	expectedShards := map[*manifest.Resource][]*manifest.Resource{
		&manifest.Resource{kind:"k", group:"g", version:"v1", namespace: "a", name: "*"}: {m["kgv1aa"],m["kgv1ab"]},
		&manifest.Resource{kind:"k", group:"g", version:"v1", namespace: "b", name: "*"}: {m["kgv1ba"],m["kgv1bb"]},
		&manifest.Resource{kind:"k", group:"g", version:"v2", namespace: "a", name: "*"}: {m["kgv2aa"],m["kgv2ab"]},
		&manifest.Resource{kind:"k", group:"g", version:"v2", namespace: "b", name: "*"}: {m["kgv2ba"],m["kgv2bb"]},
	}
	index.FindMany(&manifest.Resource{kind:"k", "G"})/*
	actual, err := list.Indexed()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Frintf(os.Stdout, "ACTUAL[%s]", actual)
	if diff := cmp.Diff(expected.String(), actual.String()); diff != "" {
		t.Fatal(diff)
	}
}
*/
