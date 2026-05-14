package lib

// Agent is the optional safety / metadata hint on a user-defined
// command. MCP clients read the resolved form (see WorkspaceAgent)
// from the `raid://workspace/commands` resource and use it to decide
// whether to auto-execute a command or prompt the user for approval.
//
// Raid never gates execution on Agent — it surfaces the hint and the
// client implements the policy. Absence of the block is equivalent to
// `{Safe: false}` so unannotated commands stay opt-in to automation.
type Agent struct {
	// Safe declares the command idempotent and side-effect free. MCP
	// clients that respect the hint may auto-run safe commands; unsafe
	// commands require explicit user approval. Defaults to false.
	Safe bool `json:"safe" yaml:"safe"`
	// Reads lists the paths or globs the command reads. Informational
	// only — raid does not parse or enforce these. Mirrors Claude
	// Code's permission-model fields so authors can reason about
	// safety the same way across tools.
	Reads []string `json:"reads,omitempty" yaml:"reads,omitempty"`
	// Writes lists the paths or globs the command writes. Same
	// semantics as Reads — informational, not enforced.
	Writes []string `json:"writes,omitempty" yaml:"writes,omitempty"`
	// Description is an agent-facing description of the command. When
	// set, it overrides the command's `usage` field in the workspace
	// resource — useful when the human-facing usage line is too terse
	// to give an agent enough context to choose the command.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}
