package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	appcli "github.com/lz-wang/m2h/internal/cli"
	"github.com/lz-wang/m2h/internal/version"
)

// M2HVersion is replaced by the Makefile through -ldflags.
var M2HVersion = version.Development

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(runContext(ctx, os.Args, os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	return runContext(context.Background(), args, stdout, stderr)
}

func runContext(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	command, err := appcli.New(M2HVersion, stdout, stderr)
	if err == nil {
		err = command.Run(ctx, args)
	}
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
