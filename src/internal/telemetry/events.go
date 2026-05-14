package telemetry

// Event builders. Each function takes the raw call-site values and
// produces a properties map that's guaranteed to be free of user
// content — no `cmd:` strings, no paths, no environment values, no
// task message bodies.
//
// Tests scan the output of every builder for forbidden substrings
// (TestEventBuilders_neverLeakUserContent) so a future field added
// to one of these maps gets caught if it slips through.

// CommandExecutedProps builds the properties map for a successful
// command run.
//
//   - commandName: the command's `name:` from YAML. Treated as
//     non-sensitive — it's a label the project author chose, not
//     anything the end user typed in.
//   - taskCount: total task entries in the command.
//   - taskTypes: ordered list of task-type strings (Shell, Script, …),
//     one entry per task in the command — duplicates are preserved so
//     the per-command structure stays visible. Types only, never the
//     cmd body or args.
//   - durationMs: wall-clock command duration in milliseconds.
func CommandExecutedProps(commandName string, taskCount int, taskTypes []string, durationMs int64) map[string]any {
	return map[string]any{
		"command_name": commandName,
		"task_count":   taskCount,
		"task_types":   taskTypes,
		"duration_ms":  durationMs,
		"success":      true,
	}
}

// CommandFailedProps is the failure variant. errorCode is the
// structured-error code (`TASK_SHELL_FAILED`, `VERIFY_FAILED`, …)
// from #47 — never the error's message, which can contain paths or
// command bodies.
func CommandFailedProps(commandName string, errorCode string, durationMs int64) map[string]any {
	return map[string]any{
		"command_name": commandName,
		"error_code":   errorCode,
		"duration_ms":  durationMs,
	}
}

// TaskExecutedProps is the per-task variant. Sampled at the call site
// so PostHog isn't flooded for commands with hundreds of tasks. Only
// the task type and outcome leak — never the cmd body, path, URL,
// var name, default value, or any other content.
func TaskExecutedProps(taskType string, durationMs int64, success bool) map[string]any {
	return map[string]any{
		"task_type":   taskType,
		"duration_ms": durationMs,
		"success":     success,
	}
}

// FirstRunProps is fired exactly once, when the user accepts the
// opt-in prompt. install_method is best-effort — empty when the
// invocation doesn't expose how raid was installed (e.g. `go install`,
// custom build).
func FirstRunProps(installMethod string) map[string]any {
	props := map[string]any{}
	if installMethod != "" {
		props["install_method"] = installMethod
	}
	return props
}

// OptOutProps records the reason a user opted out, if they supplied
// one via `raid telemetry off --why "..."`. The reason is a
// free-text field the user controls — they can include whatever
// they want, but we never collect it implicitly.
func OptOutProps(reason string) map[string]any {
	props := map[string]any{}
	if reason != "" {
		props["reason"] = reason
	}
	return props
}
