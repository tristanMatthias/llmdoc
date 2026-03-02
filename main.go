package main

import "github.com/tristanmatthias/llmdoc/cmd"

// version is set at build time via -ldflags "-X main.version=v1.2.3".
// It defaults to "dev" when built without goreleaser.
var version = "dev"

func main() {
	cmd.Execute(version)
}
