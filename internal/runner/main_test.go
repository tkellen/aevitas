package runner_test

import (
	"context"
	"github.com/tkellen/aevitas/internal/runner"
	"testing"
)

func TestElement_Render(t *testing.T) {
	if err := runner.Render(context.Background(), "default/website/domain/v1/test"); err != nil {
		t.Fatal(err)
	}
}
