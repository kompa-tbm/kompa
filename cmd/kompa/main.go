// Command kompa is the Kompa cross-platform developer toolchain manager.
//
// Build:
//
//	go build ./cmd/kompa
//
// Install:
//
//	go install github.com/kompa-tbm/kompa/cmd/kompa@latest
package main

import (
	"github.com/kompa-tbm/kompa/internal/cli"
)

func main() {
	cli.Execute()
}
