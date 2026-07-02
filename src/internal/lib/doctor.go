package lib

import (
	"fmt"
	"path/filepath"

	sys "github.com/8bitalex/raid/src/internal/sys"
)

// Severity indicates the importance of a doctor finding.
type Severity int

const (
	SeverityOK Severity = iota
	SeverityWarn
	SeverityError
)

// Finding represents the result of a single doctor check.
type Finding struct {
	Severity   Severity
	Check      string
	Message    string
	Suggestion string // shown when non-empty
}

// RunDoctor performs all configuration checks and returns the findings.
func RunDoctor() []Finding {
	var findings []Finding
	findings = append(findings, checkGit()...)
	findings = append(findings, checkProfile()...)
	return findings
}

func checkGit() []Finding {
	if isGitInstalled() {
		return []Finding{{Severity: SeverityOK, Check: "git", Message: "installed"}}
	}
	return []Finding{{
		Severity:   SeverityError,
		Check:      "git",
		Message:    "not installed or not in the PATH",
		Suggestion: "install git from https://git-scm.com",
	}}
}

func checkProfile() []Finding {
	var findings []Finding

	profile := GetProfile()
	if profile.IsZero() {
		return []Finding{{
			Severity:   SeverityError,
			Check:      "profile",
			Message:    "no active profile set",
			Suggestion: "run 'raid profile create' to create one, or 'raid profile <name>' to activate an existing profile",
		}}
	}

	findings = append(findings, Finding{
		Severity: SeverityOK,
		Check:    "profile",
		Message:  fmt.Sprintf("%s (active)", profile.Name),
	})

	if !sys.FileExists(profile.Path) {
		return append(findings, Finding{
			Severity:   SeverityError,
			Check:      "profile file",
			Message:    fmt.Sprintf("not found: %s", profile.Path),
			Suggestion: "re-create the profile file or register its new path with 'raid profile add'",
		})
	}
	findings = append(findings, Finding{
		Severity: SeverityOK,
		Check:    "profile file",
		Message:  fmt.Sprintf("found at %s", profile.Path),
	})

	if profile.IsSingleRepo() {
		fullProfile, err := BuildSingleRepoProfile(profile.Path)
		if err != nil {
			return append(findings, Finding{
				Severity:   SeverityError,
				Check:      "profile schema",
				Message:    err.Error(),
				Suggestion: "fix the raid.yaml to match the repo schema",
			})
		}
		findings = append(findings, Finding{Severity: SeverityOK, Check: "profile schema", Message: "valid (single-repo)"})
		// In single-repo mode, verify entries live on the synthesized
		// repo, not the wrapper profile — checkRepo picks them up.
		for _, repo := range fullProfile.Repositories {
			findings = append(findings, checkRepo(repo)...)
		}
		return findings
	}

	// Record the schema-validation outcome but DON'T short-circuit on
	// failure. The whole value of doctor is the all-at-once health
	// report — a schema error in the wrapping profile shouldn't hide
	// repo-level findings, verify-block findings, or other downstream
	// problems the user would want to triage in the same pass. The
	// behavior mirrors checkVerify, which deliberately walks every
	// entry and never short-circuits.
	schemaErr := ValidateProfile(profile.Path)
	if schemaErr != nil {
		findings = append(findings, Finding{
			Severity:   SeverityError,
			Check:      "profile schema",
			Message:    schemaErr.Error(),
			Suggestion: "fix the profile file to match the schema",
		})
	} else {
		findings = append(findings, Finding{Severity: SeverityOK, Check: "profile schema", Message: "valid"})
	}

	// ExtractProfile may itself fail when the schema check failed.
	// Record the load error too, then stop — there's nothing further
	// to check without a parsed profile (repos, verify entries, etc.
	// all live inside it). This is the one short-circuit we keep:
	// a missing profile struct means no further checks have a
	// meaningful answer.
	fullProfile, err := ExtractProfile(profile.Name, profile.Path)
	if err != nil {
		return append(findings, Finding{
			Severity: SeverityError,
			Check:    "profile load",
			Message:  err.Error(),
		})
	}

	findings = append(findings, checkVerify("verify", fullProfile.Verify, sys.GetHomeDir())...)

	if len(fullProfile.Repositories) == 0 {
		return append(findings, Finding{
			Severity:   SeverityWarn,
			Check:      "repositories",
			Message:    "none configured",
			Suggestion: "add repositories to your profile or run 'raid profile create'",
		})
	}

	for _, repo := range fullProfile.Repositories {
		findings = append(findings, checkRepo(repo)...)
	}
	return findings
}

func checkRepo(repo Repo) []Finding {
	var findings []Finding
	repoPath := sys.ExpandPath(repo.Path)

	if !sys.FileExists(repoPath) {
		if repo.IsLocalOnly() {
			return append(findings, Finding{
				Severity:   SeverityError,
				Check:      fmt.Sprintf("repo/%s", repo.Name),
				Message:    fmt.Sprintf("local-only repo missing at %s", repoPath),
				Suggestion: "create the directory or add a 'url' so 'raid install' can clone it",
			})
		}
		return append(findings, Finding{
			Severity:   SeverityWarn,
			Check:      fmt.Sprintf("repo/%s", repo.Name),
			Message:    fmt.Sprintf("not cloned at %s", repoPath),
			Suggestion: "run 'raid install' to clone all repositories",
		})
	}

	findings = append(findings, Finding{
		Severity: SeverityOK,
		Check:    fmt.Sprintf("repo/%s", repo.Name),
		Message:  fmt.Sprintf("found at %s", repoPath),
	})

	// Local-only repos don't need to be git repositories — that's the whole
	// point. Only warn about a missing .git when a url is configured.
	if !repo.IsLocalOnly() && !isGitRepository(repoPath) {
		findings = append(findings, Finding{
			Severity: SeverityWarn,
			Check:    fmt.Sprintf("repo/%s", repo.Name),
			Message:  "directory exists but is not a git repository",
		})
	}

	raidFile := filepath.Join(repoPath, RaidConfigFileName)
	if !sys.FileExists(raidFile) {
		return findings
	}

	if err := ValidateRepo(raidFile); err != nil {
		findings = append(findings, Finding{
			Severity:   SeverityError,
			Check:      fmt.Sprintf("repo/%s raid.yaml", repo.Name),
			Message:    err.Error(),
			Suggestion: fmt.Sprintf("fix %s to match the repo schema", raidFile),
		})
		return findings
	}
	findings = append(findings, Finding{
		Severity: SeverityOK,
		Check:    fmt.Sprintf("repo/%s raid.yaml", repo.Name),
		Message:  "valid",
	})

	// Merge verify entries from the per-repo raid.yaml. The profile-level
	// Repo only carries what's in the wrapping profile (or, for
	// BuildSingleRepoProfile, just name/path/branch), so without this
	// merge per-repo verify blocks would be silently skipped.
	repoConfig, err := ExtractRepo(repo.Path)
	if err != nil {
		findings = append(findings, Finding{
			Severity: SeverityError,
			Check:    fmt.Sprintf("repo/%s raid.yaml", repo.Name),
			Message:  err.Error(),
		})
		return findings
	}
	repo.Verify = append(repo.Verify, repoConfig.Verify...)

	findings = append(findings, checkVerify(fmt.Sprintf("repo/%s verify", repo.Name), repo.Verify, repoPath)...)
	return findings
}

// checkVerify runs each verify entry and converts the outcome into a
// finding. label is the check-name prefix ("verify" for profile-level,
// "repo/<name> verify" for repo-level). The entry's Name is appended
// so each finding has a unique, human-readable label. defaultDir is
// applied to Shell tasks without an explicit path — the repo dir for
// repo-level entries, home for profile-level — matching the execution
// context install: tasks get, so a verify passes or fails the same way
// regardless of where `raid doctor` was invoked from.
//
// Outcomes map to severities:
//   - VerifyOutcomeOK         → SeverityOK
//   - VerifyOutcomeRemediated → SeverityWarn (the verify holds now, but
//     it didn't on the first try — worth surfacing so the user knows
//     something silently fixed itself)
//   - VerifyOutcomeFailed     → SeverityError
//
// Failures don't short-circuit subsequent entries — doctor reports every
// verify so the user sees the full picture in one pass.
func checkVerify(label string, entries []Verify, defaultDir string) []Finding {
	var findings []Finding
	for _, v := range entries {
		if v.IsZero() {
			continue
		}
		check := fmt.Sprintf("%s/%s", label, v.Name)
		outcome, err := RunVerify(Verify{
			Name:   v.Name,
			Tasks:  withDefaultDir(v.Tasks, defaultDir),
			OnFail: withDefaultDir(v.OnFail, defaultDir),
		})
		switch outcome {
		case VerifyOutcomeOK:
			findings = append(findings, Finding{
				Severity: SeverityOK,
				Check:    check,
				Message:  "passed",
			})
		case VerifyOutcomeRemediated:
			findings = append(findings, Finding{
				Severity:   SeverityWarn,
				Check:      check,
				Message:    "remediated by onFail",
				Suggestion: "investigate why the precondition wasn't already in place",
			})
		case VerifyOutcomeFailed:
			msg := "failed"
			if err != nil {
				msg = err.Error()
			}
			findings = append(findings, Finding{
				Severity:   SeverityError,
				Check:      check,
				Message:    msg,
				Suggestion: "fix the underlying dependency or update the verify block to match reality",
			})
		}
	}
	return findings
}
