package main

import (
	"context"
	"fmt"
	"io"
	"os"

	appcli "github.com/lz-wang/m2h/internal/cli"
	"github.com/lz-wang/m2h/internal/version"
)

// M2HVersion is replaced by the Makefile through -ldflags.
var M2HVersion = version.Development

func main() {
	os.Exit(run(os.Args, os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	command, err := appcli.New(M2HVersion, stdout, stderr)
	if err == nil {
		err = command.Run(context.Background(), args)
	}
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
