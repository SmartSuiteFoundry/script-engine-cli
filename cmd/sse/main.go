package main

import (
	"fmt"
	"os"

	appcmd "sse-cli/internal/cmd"
)

// Version is overridden at link time when building ./cmd/sse, e.g. -X main.Version=1.0.0
var Version = "dev"

func main() {
	appcmd.Version = Version
	if err := appcmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
