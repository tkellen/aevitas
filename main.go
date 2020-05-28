package main

import "os"

var version = "dev"

func main() { os.Exit(Run(os.Args, os.Stdin, os.Stdout, os.Stderr)) }
