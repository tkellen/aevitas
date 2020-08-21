package cli

import (
	"os"
	"strings"
	"testing"
)

func Test_Run(t *testing.T) {
	//cmd := strings.Fields("test render -a ../../example/asset -l ../../example/website -l ../../example/templates -o ../../build/ website/content/v1/domain/blog")
	cmd := strings.Fields("test render -a ~/memorybox -l ../../../goingslowly/templates -l ../../../goingslowly/resources -o ../../../goingslowly/build/goingslowly.com website/content/v1/domain/goingslowly")
	stdin, err := os.Open("../../../goingslowly/build/index")
	if err != nil {
		t.Fatal(err)
	}
	if code := Run(cmd, stdin, os.Stdout, os.Stderr); code != 0 {
		t.Fatalf("exited with %d", code)
	}
}
