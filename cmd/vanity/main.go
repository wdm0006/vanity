package main

import (
	"os"

	"github.com/wmcginnis/vanity/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
