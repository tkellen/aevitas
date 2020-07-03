package main

import (
	"context"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/tkellen/aevitas/internal/logging"
	"github.com/tkellen/aevitas/internal/runner"
	"github.com/tkellen/aevitas/pkg/manifest"
	"github.com/tkellen/aevitas/pkg/resource"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func Run(args []string, stdin *os.File, stdout io.Writer, stderr io.Writer) int {
	log.SetOutput(ioutil.Discard)
	logger := logging.Standard(stdout, stderr)
	ctx, cancel := context.WithCancel(context.Background())
	// Start goroutine to capture user requesting early shutdown (CTRL+C).
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		// Tell all goroutines that their context has been cancelled.
		cancel()
		// Give some time to clean up gracefully.
		time.Sleep(time.Second * 30)
	}()
	stat, _ := stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		logger.Stderr.Print("index must be provided on stdin")
	}
	// Get list of all possible resources to render.
	fromStdin, fromStdinErr := manifest.NewListFromReader(os.Stdin)
	if fromStdinErr != nil {
		logger.Stderr.Print(fromStdinErr)
		return 1
	}
	fromDisk, fromDirErr := manifest.NewListFromDirectory("resources")
	if fromDirErr != nil {
		logger.Stderr.Print(fromDirErr)
		return 1
	}
	// Establish registry to start rendering.
	factory := runner.DefaultFactory(
		osfs.New("/home/tkellen/memorybox"),
		osfs.New("build/goingslowly"),
	)
	// Get element to render
	root, err := resource.New(args[1], append(fromStdin, fromDisk...).Indexed(), factory)
	if err != nil {
		logger.Stderr.Print(err)
		return 1
	}
	// Render element.
	if err := root.Render(ctx); err != nil {
		logger.Stderr.Print(err)
		return 1
	}
	return 0
}
