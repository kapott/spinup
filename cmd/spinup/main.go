// Package main provides the entry point for the spinup CLI tool.
// spinup spins up ephemeral GPU instances with code-assist LLMs.
package main

import (
	"github.com/tmeurs/spinup/internal/cli"
)

// Version information set at build time via ldflags
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	// Set version information in the CLI package
	cli.SetVersion(version, commit, date)

	// Execute the Cobra CLI
	cli.Execute()
}
