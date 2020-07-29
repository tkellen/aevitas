package cli

import (
	"os"
	"strings"
	"testing"
)

func Test_Run(t *testing.T) {
	cmd := strings.Fields("test render -a ../../example/asset -l ../../example/website -l ../../example/templates -o ../../build/ website/page/v1/domain/blog")
	//cmd := strings.Fields("test render -a ~/memorybox -l ../../example/templates -l ../../resources -o ../../build/ website/page/v1/domain/goingslowly")
	//stdin, err := os.Open("../../build/index")
	//if err != nil {
	//	t.Fatal(err)
	//}
	if code := Run(cmd, os.Stdin, os.Stdout, os.Stderr); code != 0 {
		t.Fatalf("exited with %d", code)
	}
}
