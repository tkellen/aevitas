package manifest_test

import (
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
		year, month, day := time.Unix(int64(idx)*86400, 0).Date()
		manifest := &manifest.Manifest{
			Selector: &manifest.Selector{
				Kind:      "test",
				Group:     "number",
				Version:   "v1",
				Namespace: "integer",
				Name:      asWord(idx),
			},
			Meta: &manifest.Meta{
				Live: true,
				// Manifest ordering within a given kind/group/version/namespace
				// is governed by publish date. Increment so manifests can be
				// validated to respect this.
				PublishAt: manifest.PublishAt{Year: year, Month: month, Day: day},
				Relations: generateRelatedTo(idx),
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
			Selector: &manifest.Selector{
				Kind:      "test",
				Group:     "number",
				Version:   "v1",
				Namespace: "set",
				Name:      "even",
			},
		})
	} else {
		relations = append(relations, &manifest.Relation{
			Selector: &manifest.Selector{
				Kind:      "test",
				Group:     "number",
				Version:   "v1",
				Namespace: "set",
				Name:      "odd",
			},
		})

	}
	if big.NewInt(int64(number)).ProbablyPrime(0) {
		relations = append(relations, &manifest.Relation{
			Selector: &manifest.Selector{
				Kind:      "test",
				Group:     "number",
				Version:   "v1",
				Namespace: "set",
				Name:      "prime",
			},
		})
	}
	return relations
}

func generateIndex(t *testing.T, manifests []*manifest.Manifest) *manifest.Index {
	// Produce a shuffled copy of the supplied manifests.
	shuffled := append([]*manifest.Manifest{}, manifests...)
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(manifests), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})
	// Insert manifests into an index in a non-deterministic order.
	index := manifest.NewIndex()
	for _, manifest := range shuffled {
		if _, err := index.Insert(manifest); err != nil {
			t.Fatal(err)
		}
	}
	// Insert "sets" (to create relationships).
	if _, err := index.Insert(
		&manifest.Manifest{
			Selector: &manifest.Selector{Kind: "test", Group: "number", Version: "v1", Namespace: "set", Name: "prime"},
			Meta:     &manifest.Meta{Live: true},
		},
		&manifest.Manifest{
			Selector: &manifest.Selector{Kind: "test", Group: "number", Version: "v1", Namespace: "set", Name: "even"},
			Meta:     &manifest.Meta{Live: true},
		},
		&manifest.Manifest{
			Selector: &manifest.Selector{Kind: "test", Group: "number", Version: "v1", Namespace: "set", Name: "odd"},
			Meta:     &manifest.Meta{Live: true},
		},
	); err != nil {
		t.Fatal(err)
	}
	// Insert "oddball" item that is not in the kind/group/version/namespace
	// as numbers.
	if _, err := index.Insert(&manifest.Manifest{
		Selector: &manifest.Selector{Kind: "k", Group: "g", Version: "v1", Namespace: "oddball", Name: "item"},
		Meta:     &manifest.Meta{Live: true},
	}); err != nil {
		t.Fatal(err)
	}
	return index
}

func TestIndex_NextPrevious(t *testing.T) {
	// Generate numbers array of manifests.
	numbers := generateManifests(1000)
	index := generateIndex(t, numbers)
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
		Selector: &manifest.Selector{Kind: "nope", Group: "nope", Version: "nope", Namespace: "nope", Name: "nope"},
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
	fmt.Fprintf(os.Stdout, "%s")

	m := map[string]*Manifest{
		"meat": {Selector: s["meat"], Meta: &Meta{
			Live: true,
			relations: []*Selector{
				s["taco"], // meat is also related to taco by way of the taco
				// including it. this helps assert that during the
				// aggregation of relations the same manifest is not
				// collected more than once.
			},
		}},
		"cheese": {Selector: s["cheese"], Meta: &Meta{
			Live: true,
			relations: []*Selector{},
		}},
		"tomato": {Selector: s["tomato"], Meta: &Meta{
			Live: true,
			relations: []*Selector{},
		}},
		"onion": {Selector: s["onion"], Meta: &Meta{
			Live: true,
			relations: []*Selector{},
		}},
		"noodles": {Selector: s["noodles"], Meta: &Meta{
			Live: true,
			relations: []*Selector{},
		}},
		"taco": {Selector: s["taco"], Meta: &Meta{
			Live: true,
			relations: []*Selector{},
			Embed: map[string]*RenderTarget{
				"meat": {Selector: s["meat"]},
				"cheese": {Selector: s["cheese"]},
				"tomato": {Selector: s["tomato"]},
				"onion": {Selector: s["onion"]},
			},
		}},
		"spaghetti": {Selector: s["spaghetti"], Meta: &Meta{
			Live: true,
			relations: []*Selector{},
			Embed: map[string]*RenderTarget{
				"meat": {Selector: s["meat"]},
				"tomato": {Selector: s["tomato"]},
				"onion": {Selector: s["onion"]},
				"noodles": {Selector: s["noodles"]},
			},
		}},
		"dishes": {Selector: s["dishes"], Meta: &Meta{
			Live: true,
			relations: []*Selector{s["taco"], s["spaghetti"]},
			Embed: map[string]*RenderTarget{},
		}},
	}
	list := newShard()
	for _, manifest := range m {
		list.insert(manifest)
	}
	expected := Relations{
		Index: index(t, list.manifests),
		Relationships: map[*Manifest]*Index{
			m["meat"]:      index(t, []*Manifest{m["taco"], m["spaghetti"]}),
			m["cheese"]:    index(t, []*Manifest{m["taco"]}),
			m["tomato"]:    index(t, []*Manifest{m["taco"], m["spaghetti"]}),
			m["onion"]:     index(t, []*Manifest{m["taco"], m["spaghetti"]}),
			m["noodles"]:   index(t, []*Manifest{m["spaghetti"]}),
			m["taco"]:      index(t, []*Manifest{m["meat"], m["cheese"], m["tomato"], m["onion"]}),
			m["spaghetti"]: index(t, []*Manifest{m["meat"], m["tomato"], m["onion"], m["noodles"]}),
			m["dishes"]:    index(t, []*Manifest{m["taco"], m["spaghetti"]}),
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

	s := map[string]*manifest.Selector{
		"kgv1aa": {Kind: "k", Group: "g", Version: "v1", Namespace: "a", Name: "a"},
		"kgv1ab": {Kind: "k", Group: "g", Version: "v1", Namespace: "a", Name: "b"},
		"kgv1ba": {Kind: "k", Group: "g", Version: "v1", Namespace: "b", Name: "a"},
		"kgv1bb": {Kind: "k", Group: "g", Version: "v1", Namespace: "b", Name: "b"},
		"kgv2aa": {Kind: "k", Group: "g", Version: "v2", Namespace: "a", Name: "a"},
		"kgv2ab": {Kind: "k", Group: "g", Version: "v2", Namespace: "a", Name: "b"},
		"kgv2ba": {Kind: "k", Group: "g", Version: "v2", Namespace: "b", Name: "a"},
		"kgv2bb": {Kind: "k", Group: "g", Version: "v2", Namespace: "b", Name: "b"},
	}
	m := map[string]*manifest.Manifest{
		"kgv1aa": {Selector: s["kgv1aa"]},
		"kgv1ab": {Selector: s["kgv1ab"]},
		"kgv1ba": {Selector: s["kgv1ba"]},
		"kgv1bb": {Selector: s["kgv1bb"]},
		"kgv2aa": {Selector: s["kgv2aa"]},
		"kgv2ab": {Selector: s["kgv2ab"]},
		"kgv2ba": {Selector: s["kgv2ba"]},
		"kgv2bb": {Selector: s["kgv2bb"]},
	}
	index := manifest.NewIndex()
	for _, item := range m {
		index.Insert(item)
	}
	expectedShards := map[*manifest.Selector][]*manifest.Manifest{
		&manifest.Selector{Kind:"k", Group:"g", Version:"v1", Namespace: "a", Name: "*"}: {m["kgv1aa"],m["kgv1ab"]},
		&manifest.Selector{Kind:"k", Group:"g", Version:"v1", Namespace: "b", Name: "*"}: {m["kgv1ba"],m["kgv1bb"]},
		&manifest.Selector{Kind:"k", Group:"g", Version:"v2", Namespace: "a", Name: "*"}: {m["kgv2aa"],m["kgv2ab"]},
		&manifest.Selector{Kind:"k", Group:"g", Version:"v2", Namespace: "b", Name: "*"}: {m["kgv2ba"],m["kgv2bb"]},
	}
	index.FindMany(&Selector{Kind:"k", "G"})/*
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
