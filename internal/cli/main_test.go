package cli

import (
	"os"
	"strings"
	"testing"
)

func Test_Run(t *testing.T) {
	cmd := strings.Fields("test render -l ../../example/templates -l ../../resources -o ../../build/goingslowly website/domain/v1/goingslowly/journal")
	stdin, err := os.Open("../../testdata/index")
	if err != nil {
		t.Fatal(err)
	}
	if code := Run(cmd, stdin, os.Stdout, os.Stderr); code != 0 {
		t.Fatalf("exited with %d", code)
	}
}
