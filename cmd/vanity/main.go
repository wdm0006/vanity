package main

import (
	"os"

	"github.com/wdm0006/vanity/internal/cli"
)

// Set via ldflags at build time
var version = "dev"

func main() {
	cli.SetVersion(version)
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
