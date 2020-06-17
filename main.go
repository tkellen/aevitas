package main

import (
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
)

var version = "dev"

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()
	os.Exit(Run(os.Args, os.Stdin, os.Stdout, os.Stderr))
}
