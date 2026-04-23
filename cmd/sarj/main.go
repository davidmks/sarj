// Package main is the entrypoint for the sarj CLI.
package main

import (
	"os"
	"runtime/debug"

	"github.com/davidmks/sarj/internal/cli"
)

var version = "dev"

func init() {
	if version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "(devel)" && info.Main.Version != "" {
			version = info.Main.Version
		}
	}
}

func main() {
	if err := cli.Execute(version); err != nil {
		os.Exit(1)
	}
}
