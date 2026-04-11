// Command raid is a declarative multi-repo development environment orchestrator.
//
// Raid is a cross-platform CLI that turns a team's commands, environments, and
// multi-repo workflows into version-controlled YAML. A profile (*.raid.yaml)
// describes repositories, environments, and custom commands; each repository
// can also commit its own raid.yaml that merges with the profile at load time.
//
// See https://github.com/8bitalex/raid for documentation.
package main

import "github.com/8bitalex/raid/src/cmd"

// this is just a prototype so don't judge me too harshly
func main() {
	cmd.Execute()
}
