package main_test

import (
	"context"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/tkellen/aevitas/pkg/manifest"
	"github.com/tkellen/aevitas/pkg/registry"
	"os"
	"testing"
)

func TestPerf(t *testing.T) {
	// Get list of all possible resources to render.
	reader, err := os.Open("testdata/index")
	if err != nil {
		t.Fatal(err)
	}
	fromStdin, fromStdinErr := manifest.NewListFromReader(reader)
	if fromStdinErr != nil {
		t.Fatal(fromStdinErr)
	}
	fromDisk, fromDirErr := manifest.NewListFromDirectory("resources")
	if fromDirErr != nil {
		t.Fatal(fromDirErr)
	}
	// Index manifests by namespace and kind.
	index := append(fromStdin, fromDisk...).Indexed()
	registry := factory.NewDefault(
		index,
		osfs.New("/home/tkellen/memorybox"),
		osfs.New("build"),
	)
	// Create a root element to render.
	if err := registry.Render(context.Background(), "goingslowly/website/domain/v1/journal"); err != nil {
		t.Fatal(err)
	}
}
