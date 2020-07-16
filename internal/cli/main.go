package cli

import (
	"context"
	"github.com/alecthomas/kong"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Cli struct {
	Debug  bool      `help:"Enable debug mode."`
	Render RenderCmd `cmd help:"Render a target manifest."`
}

type Context struct {
	Background context.Context
	Stdin      *os.File
	Logger     *Logger
}

func Run(args []string, stdin *os.File, stdout io.Writer, stderr io.Writer) int {
	log.SetOutput(ioutil.Discard)
	logger := standardLogger(stdout, stderr)
	background, cancel := context.WithCancel(context.Background())
	// Start goroutine to capture user requesting early shutdown (CTRL+C).
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		logger.Stdout.Printf("Shutdown signal recieved, cleaning up...")
		// Tell all goroutines that their context has been cancelled.
		cancel()
		// Give some time to clean up gracefully.
		time.Sleep(time.Second * 30)
	}()
	var run Cli
	cwd, cwdErr := os.Getwd()
	if cwdErr != nil {
		logger.Stdout.Printf("%s", cwdErr)
		return 1
	}
	parser, newErr := kong.New(&run, kong.Vars{
		"cwd": cwd,
	})
	if newErr != nil {
		logger.Stdout.Printf("%s", newErr)
		return 1
	}
	cli, parseErr := parser.Parse(args[1:])
	if parseErr != nil {
		logger.Stdout.Printf("%s", parseErr)
		return 1
	}
	if run.Debug {
		logger = verboseLogger(stdout, stderr)
	}
	if err := cli.Run(&Context{
		Background: background,
		Stdin:      stdin,
		Logger:     logger,
	}); err != nil {
		logger.Stdout.Printf("%s", err)
		return 1
	}
	return 0
}

type Logger struct {
	Stdout  *log.Logger
	Stderr  *log.Logger
	Verbose *log.Logger
}

func standardLogger(stdout io.Writer, stderr io.Writer) *Logger {
	return &Logger{
		Stdout:  log.New(stdout, "", 0),
		Stderr:  log.New(stderr, "", 0),
		Verbose: log.New(ioutil.Discard, "", 0),
	}
}

func verboseLogger(stdout io.Writer, stderr io.Writer) *Logger {
	Logger := standardLogger(stdout, stderr)
	Logger.Verbose.SetOutput(stderr)
	return Logger
}

func silentLogger() *Logger {
	return &Logger{
		Stdout:  log.New(ioutil.Discard, "", 0),
		Stderr:  log.New(ioutil.Discard, "", 0),
		Verbose: log.New(ioutil.Discard, "", 0),
	}
}
