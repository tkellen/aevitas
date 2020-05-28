package main

import (
	"context"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/tkellen/aevitas/internal/logging"
	"github.com/tkellen/aevitas/pkg/render"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func Run(_ []string, stdin *os.File, stdout io.Writer, stderr io.Writer) int {
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
	if err := render.Run(ctx, logger, osfs.New("build"), os.Stdin); err != nil {
		logger.Stderr.Print(err)
		return 1
	}
	return 0
}
