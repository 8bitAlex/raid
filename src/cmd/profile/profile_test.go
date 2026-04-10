package profile

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/8bitalex/raid/src/internal/lib"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func setupConfig(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	old := lib.CfgPath
	t.Cleanup(func() {
		lib.CfgPath = old
		viper.Reset()
	})
	lib.CfgPath = filepath.Join(dir, "config.toml")
	if err := lib.InitConfig(); err != nil {
		t.Fatalf("setupConfig: %v", err)
	}
}

// captureStdout redirects os.Stdout while fn runs and returns the captured output.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

// validProfileFile creates a minimal schema-valid profile YAML and returns its path.
// Only the "name" field is written; "path" is not in the profile schema and would
// fail additionalProperties:false validation.
func validProfileFile(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name+".raid.yaml")
	if err := os.WriteFile(path, []byte("name: "+name+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

// --- Command (profile root) ---

func TestCommand_noArgs_noProfile(t *testing.T) {
	setupConfig(t)
	out := captureStdout(t, func() {
		root := &cobra.Command{Use: "raid"}
		root.AddCommand(Command)
		root.SetArgs([]string{"profile"})
		_ = root.Execute()
	})
	if !strings.Contains(out, "No active profile found") {
		t.Errorf("Command no args: got %q, want 'No active profile found'", out)
	}
}

func TestCommand_noArgs_withActiveProfile(t *testing.T) {
	setupConfig(t)
	if err := lib.AddProfile(lib.Profile{Name: "myprofile", Path: "/p"}); err != nil {
		t.Fatal(err)
	}
	if err := lib.SetProfile("myprofile"); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() {
		root := &cobra.Command{Use: "raid"}
		root.AddCommand(Command)
		root.SetArgs([]string{"profile"})
		_ = root.Execute()
	})
	if !strings.Contains(out, "myprofile") {
		t.Errorf("Command with active profile: got %q, want 'myprofile'", out)
	}
}

// --- ListProfileCmd ---

func TestListProfileCmd_noProfiles(t *testing.T) {
	setupConfig(t)
	out := captureStdout(t, func() {
		root := &cobra.Command{Use: "raid"}
		root.AddCommand(ListProfileCmd)
		root.SetArgs([]string{"list"})
		_ = root.Execute()
	})
	if !strings.Contains(out, "No profiles found") {
		t.Errorf("ListProfileCmd no profiles: got %q, want 'No profiles found'", out)
	}
}

func TestListProfileCmd_withProfiles(t *testing.T) {
	setupConfig(t)
	if err := lib.AddProfile(lib.Profile{Name: "listed", Path: "/listed"}); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() {
		root := &cobra.Command{Use: "raid"}
		root.AddCommand(ListProfileCmd)
		root.SetArgs([]string{"list"})
		_ = root.Execute()
	})
	if !strings.Contains(out, "listed") {
		t.Errorf("ListProfileCmd with profiles: got %q, want 'listed'", out)
	}
}

func TestListProfileCmd_marksActiveProfile(t *testing.T) {
	setupConfig(t)
	if err := lib.AddProfile(lib.Profile{Name: "activep", Path: "/ap"}); err != nil {
		t.Fatal(err)
	}
	if err := lib.SetProfile("activep"); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() {
		root := &cobra.Command{Use: "raid"}
		root.AddCommand(ListProfileCmd)
		root.SetArgs([]string{"list"})
		_ = root.Execute()
	})
	if !strings.Contains(out, "active") {
		t.Errorf("ListProfileCmd active marker: got %q, want '(active)'", out)
	}
}

// --- RemoveProfileCmd ---

func TestRemoveProfileCmd_notFound(t *testing.T) {
	setupConfig(t)
	out := captureStdout(t, func() {
		root := &cobra.Command{Use: "raid"}
		root.AddCommand(RemoveProfileCmd)
		root.SetArgs([]string{"remove", "ghost"})
		_ = root.Execute()
	})
	if !strings.Contains(out, "not found") {
		t.Errorf("RemoveProfileCmd missing: got %q, want 'not found'", out)
	}
}

func TestRemoveProfileCmd_found(t *testing.T) {
	setupConfig(t)
	if err := lib.AddProfile(lib.Profile{Name: "todel", Path: "/p"}); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() {
		root := &cobra.Command{Use: "raid"}
		root.AddCommand(RemoveProfileCmd)
		root.SetArgs([]string{"remove", "todel"})
		_ = root.Execute()
	})
	if !strings.Contains(out, "removed") {
		t.Errorf("RemoveProfileCmd found: got %q, want 'removed'", out)
	}
}

func TestRemoveProfileCmd_multipleArgs(t *testing.T) {
	setupConfig(t)
	if err := lib.AddProfile(lib.Profile{Name: "rm1", Path: "/p1"}); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() {
		root := &cobra.Command{Use: "raid"}
		root.AddCommand(RemoveProfileCmd)
		root.SetArgs([]string{"remove", "rm1", "rm2"})
		_ = root.Execute()
	})
	// rm1 should be removed, rm2 should say not found.
	if !strings.Contains(out, "removed") {
		t.Errorf("RemoveProfileCmd multi: got %q", out)
	}
	if !strings.Contains(out, "not found") {
		t.Errorf("RemoveProfileCmd multi not found: got %q", out)
	}
}

// --- AddProfileCmd ---

func TestAddProfileCmd_newProfile(t *testing.T) {
	setupConfig(t)
	path := validProfileFile(t, "fresh")
	out := captureStdout(t, func() {
		root := &cobra.Command{Use: "raid"}
		root.AddCommand(AddProfileCmd)
		root.SetArgs([]string{"add", path})
		_ = root.Execute()
	})
	if !strings.Contains(out, "fresh") {
		t.Errorf("AddProfileCmd new profile: got %q, want 'fresh'", out)
	}
}

func TestAddProfileCmd_multipleNewProfiles(t *testing.T) {
	setupConfig(t)
	// Pre-set an active profile so AddProfileCmd doesn't try to set the new one.
	if err := lib.AddProfile(lib.Profile{Name: "existing-active", Path: "/ea"}); err != nil {
		t.Fatal(err)
	}
	if err := lib.SetProfile("existing-active"); err != nil {
		t.Fatal(err)
	}

	// Write two profiles in a single YAML multi-doc file (no "path" field — it's
	// not in the profile schema and would fail additionalProperties:false).
	dir := t.TempDir()
	path := filepath.Join(dir, "multi.raid.yaml")
	content := "name: alpha\n---\nname: beta\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		root := &cobra.Command{Use: "raid"}
		root.AddCommand(AddProfileCmd)
		root.SetArgs([]string{"add", path})
		_ = root.Execute()
	})
	if !strings.Contains(out, "alpha") && !strings.Contains(out, "beta") {
		t.Errorf("AddProfileCmd multi: got %q, want profile names in output", out)
	}
}

// --- Command with one arg (set active profile by name) ---

func TestCommand_setActiveProfileByArg(t *testing.T) {
	setupConfig(t)
	if err := lib.AddProfile(lib.Profile{Name: "setme", Path: "/setme"}); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() {
		root := &cobra.Command{Use: "raid"}
		root.AddCommand(Command)
		root.SetArgs([]string{"profile", "setme"})
		_ = root.Execute()
	})
	if !strings.Contains(out, "setme") || !strings.Contains(out, "active") {
		t.Errorf("Command set active: got %q, want 'setme' and 'active'", out)
	}
}

// --- runCreateWizard ---

// TestRunCreateWizard_basicFlow exercises the interactive profile-creation wizard
// by replacing os.Stdin with a pipe that provides scripted input.
func TestRunCreateWizard_basicFlow(t *testing.T) {
	setupConfig(t)

	savePath := filepath.Join(t.TempDir(), "wizard-profile.raid.yaml")

	// Input: profile name, custom save path, answer "n" to add repositories.
	input := "wizard-profile\n" + savePath + "\nn\n"

	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stdinW.WriteString(input); err != nil {
		t.Fatal(err)
	}
	stdinW.Close()

	oldStdin := os.Stdin
	os.Stdin = stdinR
	t.Cleanup(func() {
		os.Stdin = oldStdin
		stdinR.Close()
	})

	// Suppress stdout noise from the wizard prompts.
	out := captureStdout(t, func() {
		cmd := &cobra.Command{}
		runCreateWizard(cmd, nil)
	})

	if !strings.Contains(out, "wizard-profile") {
		t.Errorf("runCreateWizard: got %q, want 'wizard-profile' in output", out)
	}

	if _, err := os.Stat(savePath); err != nil {
		t.Errorf("runCreateWizard: profile file not created at %s: %v", savePath, err)
	}
}

// TestRunCreateWizard_withRepo exercises the repo-collection and repo-config-creation branches.
func TestRunCreateWizard_withRepo(t *testing.T) {
	setupConfig(t)

	saveDir := t.TempDir()
	savePath := filepath.Join(saveDir, "repoprofile.raid.yaml")
	repoPath := t.TempDir()

	// Input: profile name, save path, add a repo (y), repo details, no more repos, create raid.yaml (n)
	input := "repoprofile\n" +
		savePath + "\n" +
		"y\n" + // add a repository?
		"myrepo\n" + // repo name
		"https://127.0.0.1:1/repo.git\n" + // URL (unreachable → no auto-branch detect)
		repoPath + "\n" + // local path
		"main\n" + // branch
		"n\n" + // add another?
		"n\n" // create raid.yaml for each repo?

	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdinW.WriteString(input)
	stdinW.Close()

	oldStdin := os.Stdin
	os.Stdin = stdinR
	t.Cleanup(func() {
		os.Stdin = oldStdin
		stdinR.Close()
	})

	_ = captureStdout(t, func() {
		cmd := &cobra.Command{}
		runCreateWizard(cmd, nil)
	})

	if _, err := os.Stat(savePath); err != nil {
		t.Errorf("runCreateWizard with repo: profile file not created at %s: %v", savePath, err)
	}
}
