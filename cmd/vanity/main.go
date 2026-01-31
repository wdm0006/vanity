package main

import (
	"os"

	"github.com/wmcginnis/vanity/internal/cli"
)

// Set via ldflags at build time
var version = "dev"

func main() {
	cli.SetVersion(version)
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
