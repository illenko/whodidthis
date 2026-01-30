package main

import (
	"os"

	"github.com/illenko/metriccost/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
