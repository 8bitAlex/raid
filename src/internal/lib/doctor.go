package lib

import (
	"fmt"
	"path/filepath"

	sys "github.com/8bitalex/raid/src/internal/sys"
)

// Severity indicates the importance of a doctor finding.
type Severity int

const (
	SeverityOK    Severity = iota
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

	if err := ValidateProfile(profile.Path); err != nil {
		return append(findings, Finding{
			Severity:   SeverityError,
			Check:      "profile schema",
			Message:    err.Error(),
			Suggestion: "fix the profile file to match the schema",
		})
	}
	findings = append(findings, Finding{Severity: SeverityOK, Check: "profile schema", Message: "valid"})

	fullProfile, err := ExtractProfile(profile.Name, profile.Path)
	if err != nil {
		return append(findings, Finding{
			Severity: SeverityError,
			Check:    "profile load",
			Message:  err.Error(),
		})
	}

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

	if !isGitRepository(repoPath) {
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
	} else {
		findings = append(findings, Finding{
			Severity: SeverityOK,
			Check:    fmt.Sprintf("repo/%s raid.yaml", repo.Name),
			Message:  "valid",
		})
	}
	return findings
}
