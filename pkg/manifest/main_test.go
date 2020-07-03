package manifest_test

import (
	"github.com/tkellen/aevitas/pkg/manifest"
	"testing"
)

func TestNewFromFile(t *testing.T) {
	manifest, err := manifest.NewFromFile("../../manifests/goingslowly/pages/2007/12/merry-christmas")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s", manifest)
}
