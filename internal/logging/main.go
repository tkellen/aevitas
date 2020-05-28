package logging

import (
	"io"
	"io/ioutil"
	"log"
)

// Logger defines output streams for interacting with archives.
type Streams struct {
	Stdout  *log.Logger
	Stderr  *log.Logger
	Verbose *log.Logger
}

func Standard(stdout io.Writer, stderr io.Writer) *Streams {
	return &Streams{
		Stdout:  log.New(stdout, "", 0),
		Stderr:  log.New(stderr, "", 0),
		Verbose: log.New(ioutil.Discard, "", 0),
	}
}

func Verbose(stdout io.Writer, stderr io.Writer) *Streams {
	streams := Standard(stdout, stderr)
	streams.Verbose.SetOutput(stderr)
	return streams
}

func Silent() *Streams {
	return &Streams{
		Stdout:  log.New(ioutil.Discard, "", 0),
		Stderr:  log.New(ioutil.Discard, "", 0),
		Verbose: log.New(ioutil.Discard, "", 0),
	}
}
