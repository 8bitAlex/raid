package errs

// Each constructor produces an error message that matches the prior
// fmt.Errorf wording so tests doing substring matches against error
// strings keep working unchanged. New callers should rely on Code()
// and Category() rather than parsing Error().

// Unknown wraps an arbitrary cause as a generic raid error.
func Unknown(cause error) *RaidError {
	msg := "unknown error"
	if cause != nil {
		msg = cause.Error()
	}
	return newRaidError(CodeUnknown, CategoryGeneric, msg, "", nil, cause)
}

// Internal flags a logic error inside raid itself.
func Internal(msg string) *RaidError {
	return newRaidError(CodeInternal, CategoryGeneric, msg,
		"This is a raid bug — please file an issue with the command that triggered it.", nil, nil)
}

// GitNotInstalled — git binary missing.
func GitNotInstalled() *RaidError {
	return newRaidError(CodeGitNotInstalled, CategoryGeneric,
		"git is not installed or not in the PATH",
		"Install git (https://git-scm.com) and re-run.",
		nil, nil)
}

// LockFailed — couldn't acquire ~/.raid/.lock.
func LockFailed(cause error) *RaidError {
	msg := "failed to acquire raid mutation lock"
	if cause != nil {
		msg = formatMsg("failed to acquire raid mutation lock: %v", cause)
	}
	return newRaidError(CodeLockFailed, CategoryGeneric, msg,
		"Another raid process may be holding ~/.raid/.lock; wait for it to finish or check for stale processes.",
		nil, cause)
}

// ProfileNotFound — name isn't registered.
func ProfileNotFound(name string) *RaidError {
	return newRaidError(CodeProfileNotFound, CategoryNotFound,
		formatMsg("profile '%s' not found", name),
		"Run `raid profile list` to see registered profiles.",
		map[string]any{"profile": name}, nil)
}

// ProfileNotActive — no active profile is set.
func ProfileNotActive() *RaidError {
	return newRaidError(CodeProfileNotActive, CategoryNotFound,
		"no active profile",
		"Run `raid profile <name>` to set an active profile, or `raid profile add <path-or-url>` to register one.",
		nil, nil)
}

// ProfileFileMissing — registered profile path doesn't exist.
func ProfileFileMissing(path string) *RaidError {
	return newRaidError(CodeProfileFileMissing, CategoryNotFound,
		formatMsg("profile file not found at %s", path),
		"The profile is registered but its file is missing on disk. Re-add it or remove the registration with `raid profile remove`.",
		map[string]any{"path": path}, nil)
}

// ProfileFileRead — couldn't read / parse the profile file.
func ProfileFileRead(path string, cause error) *RaidError {
	msg := formatMsg("failed to read profile %s", path)
	if cause != nil {
		msg = formatMsg("failed to read profile %s: %v", path, cause)
	}
	return newRaidError(CodeProfileFileRead, CategoryConfig, msg,
		"Check that the file exists and is readable.",
		map[string]any{"path": path}, cause)
}

// ProfileInvalid — profile failed schema validation.
func ProfileInvalid(path string, cause error) *RaidError {
	msg := formatMsg("invalid profile at %s", path)
	if cause != nil {
		msg = formatMsg("invalid profile at %s: %v", path, cause)
	}
	return newRaidError(CodeProfileInvalid, CategoryConfig, msg,
		"Fix the profile to match the schema. See `raid doctor` for details.",
		map[string]any{"path": path}, cause)
}

// ProfileAlreadyExists — duplicate name on `raid profile add`.
func ProfileAlreadyExists(name string) *RaidError {
	return newRaidError(CodeProfileAlreadyExists, CategoryConfig,
		formatMsg("profile '%s' already exists", name),
		"Use a different name or `raid profile remove` the existing one first.",
		map[string]any{"profile": name}, nil)
}

// RepoNotFound — repo name not in the active profile.
func RepoNotFound(name string) *RaidError {
	return newRaidError(CodeRepoNotFound, CategoryNotFound,
		formatMsg("repository '%s' not found", name),
		"Run `raid context` to see configured repositories.",
		map[string]any{"repo": name}, nil)
}

// RepoNotCloned — repo path doesn't exist on disk.
func RepoNotCloned(name, path string) *RaidError {
	return newRaidError(CodeRepoNotCloned, CategoryNotFound,
		formatMsg("repository '%s' is not cloned at %s", name, path),
		"Run `raid install` to clone all repos in the active profile.",
		map[string]any{"repo": name, "path": path}, nil)
}

// RepoInvalid — repo entry is malformed.
func RepoInvalid(name string, cause error) *RaidError {
	msg := formatMsg("invalid repository '%s'", name)
	if cause != nil {
		msg = formatMsg("invalid repository '%s': %v", name, cause)
	}
	return newRaidError(CodeRepoInvalid, CategoryConfig, msg,
		"Check the repo's raid.yaml against the published schema.",
		map[string]any{"repo": name}, cause)
}

// CloneFailed — git clone returned non-zero.
func CloneFailed(name, url string, cause error) *RaidError {
	msg := formatMsg("failed to clone repository '%s'", name)
	if cause != nil {
		msg = formatMsg("failed to clone repository '%s': %v", name, cause)
	}
	return newRaidError(CodeCloneFailed, CategoryNetwork, msg,
		"Verify the URL is reachable and you have permission to clone it.",
		map[string]any{"repo": name, "url": url}, cause)
}

// EnvNotFound — environment name not declared.
func EnvNotFound(name string) *RaidError {
	return newRaidError(CodeEnvNotFound, CategoryNotFound,
		formatMsg("environment '%s' not found in active profile", name),
		"Run `raid env list` to see declared environments.",
		map[string]any{"env": name}, nil)
}

// CommandNotFound — `raid <cmd>` not declared.
func CommandNotFound(name string) *RaidError {
	return newRaidError(CodeCommandNotFound, CategoryNotFound,
		formatMsg("command '%s' not found", name),
		"Run `raid --help` to see available commands.",
		map[string]any{"command": name}, nil)
}

// ArgInvalid — CLI argument failed validation.
func ArgInvalid(msg string) *RaidError {
	return newRaidError(CodeArgInvalid, CategoryConfig, msg, "", nil, nil)
}

// ConfigInvalid — root config malformed.
func ConfigInvalid(cause error) *RaidError {
	msg := "invalid raid configuration"
	if cause != nil {
		msg = formatMsg("invalid raid configuration: %v", cause)
	}
	return newRaidError(CodeConfigInvalid, CategoryConfig, msg, "", nil, cause)
}

// ConfigLoadFailed — config read error.
func ConfigLoadFailed(cause error) *RaidError {
	msg := "failed to load raid configuration"
	if cause != nil {
		msg = formatMsg("failed to load raid configuration: %v", cause)
	}
	return newRaidError(CodeConfigLoadFailed, CategoryConfig, msg,
		"Run `raid doctor` to diagnose the configuration.",
		nil, cause)
}

// SchemaValidationFailed — JSONSchema check failed.
func SchemaValidationFailed(path string, cause error) *RaidError {
	msg := formatMsg("schema validation failed for %s", path)
	if cause != nil {
		msg = formatMsg("schema validation failed for %s: %v", path, cause)
	}
	return newRaidError(CodeSchemaValidationFailed, CategoryConfig, msg, "",
		map[string]any{"path": path}, cause)
}

// TaskFailed — generic task-execution wrapper.
func TaskFailed(taskType string, cause error) *RaidError {
	msg := formatMsg("%s task failed", taskType)
	if cause != nil {
		msg = formatMsg("%s task failed: %v", taskType, cause)
	}
	return newRaidError(CodeTaskFailed, CategoryTask, msg, "",
		map[string]any{"task": taskType}, cause)
}

// TaskShellFailed — Shell subprocess non-zero.
func TaskShellFailed(cause error) *RaidError {
	msg := "Shell task failed"
	if cause != nil {
		msg = formatMsg("Shell task failed: %v", cause)
	}
	return newRaidError(CodeTaskShellFailed, CategoryTask, msg, "",
		map[string]any{"task": "Shell"}, cause)
}

// TaskScriptFailed — Script subprocess non-zero.
func TaskScriptFailed(cause error) *RaidError {
	msg := "Script task failed"
	if cause != nil {
		msg = formatMsg("Script task failed: %v", cause)
	}
	return newRaidError(CodeTaskScriptFailed, CategoryTask, msg, "",
		map[string]any{"task": "Script"}, cause)
}

// TaskWaitTimeout — Wait task exceeded timeout.
func TaskWaitTimeout(target string, cause error) *RaidError {
	msg := formatMsg("Wait timed out for %s", target)
	if cause != nil {
		msg = formatMsg("Wait timed out for %s: %v", target, cause)
	}
	return newRaidError(CodeTaskWaitTimeout, CategoryTask, msg, "",
		map[string]any{"task": "Wait", "target": target}, cause)
}

// TaskTemplateFailed — Template render/write failed.
func TaskTemplateFailed(cause error) *RaidError {
	msg := "Template task failed"
	if cause != nil {
		msg = formatMsg("Template task failed: %v", cause)
	}
	return newRaidError(CodeTaskTemplateFailed, CategoryTask, msg, "",
		map[string]any{"task": "Template"}, cause)
}

// TaskGitFailed — Git task (non-clone) failed.
func TaskGitFailed(cause error) *RaidError {
	msg := "Git task failed"
	if cause != nil {
		msg = formatMsg("Git task failed: %v", cause)
	}
	return newRaidError(CodeTaskGitFailed, CategoryTask, msg, "",
		map[string]any{"task": "Git"}, cause)
}

// TaskHTTPFailed — HTTP task (download/GET) failed. Network category.
func TaskHTTPFailed(url string, cause error) *RaidError {
	msg := formatMsg("HTTP task failed for %s", url)
	if cause != nil {
		msg = formatMsg("HTTP task failed for %s: %v", url, cause)
	}
	return newRaidError(CodeTaskHTTPFailed, CategoryNetwork, msg, "",
		map[string]any{"task": "HTTP", "url": url}, cause)
}
