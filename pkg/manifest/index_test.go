package manifest_test

import (
	"bytes"
	"github.com/google/go-cmp/cmp"
	"github.com/tkellen/aevitas/pkg/manifest"
	"testing"
)

func TestList_IndexShards(t *testing.T) {
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
	list := manifest.NewList()
	for _, manifest := range m {
		list.Insert(manifest)
	}
	expected := manifest.Index{
		Shard: map[string]*manifest.List{
			"k/g/v1/a": {
				Manifests: []*manifest.Manifest{m["kgv1aa"], m["kgv1ab"]},
				ByID: map[string]*manifest.Manifest{
					s["kgv1aa"].ID(): m["kgv1aa"],
					s["kgv1ab"].ID(): m["kgv1ab"],
				},
			},
			"k/g/v1/b": {
				Manifests: []*manifest.Manifest{m["kgv1ba"], m["kgv1bb"]},
				ByID: map[string]*manifest.Manifest{
					s["kgv1ba"].ID(): m["kgv1ba"],
					s["kgv1bb"].ID(): m["kgv1bb"],
				},
			},
			"k/g/v2/a": {
				Manifests: []*manifest.Manifest{m["kgv2aa"], m["kgv2ab"]},
				ByID: map[string]*manifest.Manifest{
					s["kgv2aa"].ID(): m["kgv2aa"],
					s["kgv2ab"].ID(): m["kgv2ab"],
				},
			},
			"k/g/v2/b": {
				Manifests: []*manifest.Manifest{m["kgv2ba"], m["kgv2bb"]},
				ByID: map[string]*manifest.Manifest{
					s["kgv2ba"].ID(): m["kgv2ba"],
					s["kgv2bb"].ID(): m["kgv2bb"],
				},
			},
		},
	}
	actual, err := list.Indexed(false)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(expected.String(), actual.String()); diff != "" {
		t.Fatal(diff)
	}
}

func TestList_IndexRelations(t *testing.T) {
	s := map[string]*manifest.Selector{
		"meat":      {Kind: "k", Group: "g", Version: "v1", Namespace: "ns", Name: "meat"},
		"cheese":    {Kind: "k", Group: "g", Version: "v1", Namespace: "ns", Name: "cheese"},
		"tomato":    {Kind: "k", Group: "g", Version: "v1", Namespace: "ns", Name: "tomato"},
		"onion":     {Kind: "k", Group: "g", Version: "v1", Namespace: "ns", Name: "onion"},
		"noodles":   {Kind: "k", Group: "g", Version: "v1", Namespace: "ns", Name: "noodles"},
		"taco":      {Kind: "k", Group: "g", Version: "v1", Namespace: "ns", Name: "taco"},
		"spaghetti": {Kind: "k", Group: "g", Version: "v1", Namespace: "ns", Name: "spaghetti"},
		"dishes":    {Kind: "k", Group: "g", Version: "v1", Namespace: "ns", Name: "dishes"},
	}
	m := map[string]*manifest.Manifest{
		"meat": {Selector: s["meat"], Meta: &manifest.Meta{
			Related: []*manifest.Selector{
				s["taco"], // meat is also related to taco by way of the taco
				// including it. this helps assert that during the
				// aggregation of relations the same manifest is not
				// collected more than once.
			}, Include: []*manifest.Include{},
		}},
		"cheese": {Selector: s["cheese"], Meta: &manifest.Meta{
			Related: []*manifest.Selector{}, Include: []*manifest.Include{},
		}},
		"tomato": {Selector: s["tomato"], Meta: &manifest.Meta{
			Related: []*manifest.Selector{}, Include: []*manifest.Include{},
		}},
		"onion": {Selector: s["onion"], Meta: &manifest.Meta{
			Related: []*manifest.Selector{}, Include: []*manifest.Include{},
		}},
		"noodles": {Selector: s["noodles"], Meta: &manifest.Meta{
			Related: []*manifest.Selector{}, Include: []*manifest.Include{},
		}},
		"taco": {Selector: s["taco"], Meta: &manifest.Meta{
			Related: []*manifest.Selector{}, Include: []*manifest.Include{
				{Resource: s["meat"]},
				{Resource: s["cheese"]},
				{Resource: s["tomato"]},
				{Resource: s["onion"]},
			},
		}},
		"spaghetti": {Selector: s["spaghetti"], Meta: &manifest.Meta{
			Related: []*manifest.Selector{}, Include: []*manifest.Include{
				{Resource: s["meat"]},
				{Resource: s["tomato"]},
				{Resource: s["onion"]},
				{Resource: s["noodles"]},
			},
		}},
		"dishes": {Selector: s["dishes"], Meta: &manifest.Meta{
			Related: []*manifest.Selector{s["taco"], s["spaghetti"]}, Include: []*manifest.Include{},
		}},
	}
	expected := manifest.Relations{
		m["meat"]: {
			Manifests: []*manifest.Manifest{m["taco"], m["spaghetti"]},
		},
		m["cheese"]: {
			Manifests: []*manifest.Manifest{m["taco"]},
		},
		m["tomato"]: {
			Manifests: []*manifest.Manifest{m["taco"], m["spaghetti"]},
		},
		m["onion"]: {
			Manifests: []*manifest.Manifest{m["taco"], m["spaghetti"]},
		},
		m["noodles"]: {
			Manifests: []*manifest.Manifest{m["spaghetti"]},
		},
		m["taco"]: {
			Manifests: []*manifest.Manifest{m["meat"], m["cheese"], m["tomato"], m["onion"]},
		},
		m["spaghetti"]: {
			Manifests: []*manifest.Manifest{m["meat"], m["tomato"], m["onion"], m["noodles"]},
		},
		m["dishes"]: {
			Manifests: []*manifest.Manifest{m["taco"], m["spaghetti"]},
		},
	}
	list := manifest.NewList()
	for _, manifest := range m {
		list.Insert(manifest)
	}
	actual, err := list.Indexed(true)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(expected.String(), actual.Relations.String()); diff != "" {
		t.Fatal(diff)
	}
}
