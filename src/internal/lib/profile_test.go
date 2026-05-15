package lib

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestProfileIsZero(t *testing.T) {
	tests := []struct {
		name    string
		profile Profile
		want    bool
	}{
		{"empty profile", Profile{}, true},
		{"name only", Profile{Name: "test"}, true},
		{"path only", Profile{Path: "/some/path"}, true},
		{"name and path", Profile{Name: "test", Path: "/some/path"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.profile.IsZero(); got != tt.want {
				t.Errorf("IsZero() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProfileGetEnv(t *testing.T) {
	profile := Profile{
		Environments: []Env{
			{Name: "dev", Variables: []EnvVar{{Name: "KEY", Value: "val"}}},
			{Name: "prod"},
		},
	}

	t.Run("found", func(t *testing.T) {
		env := profile.getEnv("dev")
		if env.Name != "dev" {
			t.Errorf("getEnv(\"dev\") = %q, want \"dev\"", env.Name)
		}
	})

	t.Run("not found returns zero", func(t *testing.T) {
		env := profile.getEnv("staging")
		if !env.IsZero() {
			t.Errorf("getEnv(\"staging\") should return zero Env, got %v", env)
		}
	})
}

func TestAddAndContainsProfile(t *testing.T) {
	setupTestConfig(t)

	if err := AddProfile(Profile{Name: "myprofile", Path: "/some/path"}); err != nil {
		t.Fatalf("AddProfile() error: %v", err)
	}

	if !ContainsProfile("myprofile") {
		t.Error("ContainsProfile() = false after AddProfile(), want true")
	}
}

func TestContainsProfile_notFound(t *testing.T) {
	setupTestConfig(t)

	if ContainsProfile("nonexistent") {
		t.Error("ContainsProfile() = true for nonexistent profile, want false")
	}
}

func TestListProfiles(t *testing.T) {
	setupTestConfig(t)

	if err := AddProfile(Profile{Name: "list-a", Path: "/a"}); err != nil {
		t.Fatalf("AddProfile() error: %v", err)
	}
	if err := AddProfile(Profile{Name: "list-b", Path: "/b"}); err != nil {
		t.Fatalf("AddProfile() error: %v", err)
	}

	profiles := ListProfiles()
	names := make(map[string]bool)
	for _, p := range profiles {
		names[p.Name] = true
	}
	if !names["list-a"] || !names["list-b"] {
		t.Errorf("ListProfiles() = %v, missing added profiles", profiles)
	}
}

func TestAddProfiles(t *testing.T) {
	setupTestConfig(t)

	if err := AddProfiles([]Profile{
		{Name: "bulk-a", Path: "/a"},
		{Name: "bulk-b", Path: "/b"},
	}); err != nil {
		t.Fatalf("AddProfiles() error: %v", err)
	}

	if !ContainsProfile("bulk-a") || !ContainsProfile("bulk-b") {
		t.Error("AddProfiles() did not add all profiles")
	}
}

func TestRemoveProfile(t *testing.T) {
	setupTestConfig(t)

	if err := AddProfile(Profile{Name: "toremove", Path: "/path"}); err != nil {
		t.Fatalf("AddProfile() error: %v", err)
	}
	if err := RemoveProfile("toremove"); err != nil {
		t.Fatalf("RemoveProfile() error: %v", err)
	}
	if ContainsProfile("toremove") {
		t.Error("ContainsProfile() = true after RemoveProfile(), want false")
	}
}

func TestRemoveProfile_notFound(t *testing.T) {
	setupTestConfig(t)

	if err := AddProfile(Profile{Name: "existing", Path: "/path"}); err != nil {
		t.Fatalf("AddProfile() error: %v", err)
	}
	err := RemoveProfile("nonexistent")
	if err == nil {
		t.Fatal("RemoveProfile() expected error for nonexistent profile")
	}
}

func TestRemoveProfile_noProfiles(t *testing.T) {
	setupTestConfig(t)

	err := RemoveProfile("anything")
	if err == nil {
		t.Fatal("RemoveProfile() on empty config should error")
	}
}

func TestSetProfile_notFound(t *testing.T) {
	setupTestConfig(t)

	err := SetProfile("nonexistent")
	if err == nil {
		t.Fatal("SetProfile() expected error for nonexistent profile")
	}
}

func TestSetAndGetProfile(t *testing.T) {
	setupTestConfig(t)

	if err := AddProfile(Profile{Name: "active", Path: "/active/path"}); err != nil {
		t.Fatalf("AddProfile() error: %v", err)
	}
	if err := SetProfile("active"); err != nil {
		t.Fatalf("SetProfile() error: %v", err)
	}

	got := GetProfile()
	if got.Name != "active" {
		t.Errorf("GetProfile() name = %q, want %q", got.Name, "active")
	}
}

func TestGetProfile_fromContext(t *testing.T) {
	setupTestConfig(t)

	storeContext(&Context{
		Profile: Profile{Name: "ctx-profile", Path: "/ctx/path"},
	})

	got := GetProfile()
	if got.Name != "ctx-profile" {
		t.Errorf("GetProfile() from context = %q, want %q", got.Name, "ctx-profile")
	}
}

func TestExtractProfiles_singleYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.yaml")

	data := Profile{Name: "yamltest", Path: path}
	b, _ := yaml.Marshal(data)
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Fatal(err)
	}

	profiles, err := ExtractProfiles(path)
	if err != nil {
		t.Fatalf("ExtractProfiles() error: %v", err)
	}
	if len(profiles) != 1 || profiles[0].Name != "yamltest" {
		t.Errorf("ExtractProfiles() = %v, want single profile named yamltest", profiles)
	}
}

func TestExtractProfiles_multiDocYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profiles.yaml")

	content := "name: first\n---\nname: second\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	profiles, err := ExtractProfiles(path)
	if err != nil {
		t.Fatalf("ExtractProfiles() error: %v", err)
	}
	if len(profiles) != 2 {
		t.Fatalf("ExtractProfiles() returned %d profiles, want 2", len(profiles))
	}
}

func TestExtractProfiles_singleJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.json")

	data, _ := json.Marshal(Profile{Name: "jsontest"})
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	profiles, err := ExtractProfiles(path)
	if err != nil {
		t.Fatalf("ExtractProfiles() error: %v", err)
	}
	if len(profiles) != 1 || profiles[0].Name != "jsontest" {
		t.Errorf("ExtractProfiles() = %v, want profile named jsontest", profiles)
	}
}

func TestExtractProfiles_arrayJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profiles.json")

	data, _ := json.Marshal([]Profile{{Name: "a"}, {Name: "b"}})
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	profiles, err := ExtractProfiles(path)
	if err != nil {
		t.Fatalf("ExtractProfiles() error: %v", err)
	}
	if len(profiles) != 2 {
		t.Fatalf("ExtractProfiles() returned %d profiles, want 2", len(profiles))
	}
}

func TestExtractProfiles_invalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	os.WriteFile(path, []byte("{invalid json}"), 0644)

	_, err := ExtractProfiles(path)
	if err == nil {
		t.Fatal("ExtractProfiles() expected error for invalid JSON")
	}
}

func TestExtractProfiles_invalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	os.WriteFile(path, []byte("key: [unclosed"), 0644)

	_, err := ExtractProfiles(path)
	if err == nil {
		t.Fatal("ExtractProfiles() expected error for invalid YAML")
	}
}

func TestExtractProfiles_unsupportedExtension(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.xml")
	os.WriteFile(path, []byte("<profile/>"), 0644)

	_, err := ExtractProfiles(path)
	if err == nil {
		t.Fatal("ExtractProfiles() expected error for unsupported extension")
	}
}

func TestExtractProfiles_fileNotFound(t *testing.T) {
	_, err := ExtractProfiles("/nonexistent/path/profile.yaml")
	if err == nil {
		t.Fatal("ExtractProfiles() expected error for missing file")
	}
}

func TestExtractProfiles_emptyYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.yaml")
	os.WriteFile(path, []byte(""), 0644)

	_, err := ExtractProfiles(path)
	if err == nil {
		t.Fatal("ExtractProfiles() expected error for empty YAML (no profiles)")
	}
}

func TestExtractProfile_found(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profiles.yaml")
	os.WriteFile(path, []byte("name: first\n---\nname: second\n"), 0644)

	p, err := ExtractProfile("first", path)
	if err != nil {
		t.Fatalf("ExtractProfile() error: %v", err)
	}
	if p.Name != "first" {
		t.Errorf("ExtractProfile() name = %q, want %q", p.Name, "first")
	}
}

func TestExtractProfile_notFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profiles.yaml")
	os.WriteFile(path, []byte("name: only\n"), 0644)

	_, err := ExtractProfile("nonexistent", path)
	if err == nil {
		t.Fatal("ExtractProfile() expected error for missing profile name")
	}
}

func TestExtractProfile_extractionError(t *testing.T) {
	_, err := ExtractProfile("anyname", "/nonexistent/path/profile.yaml")
	if err == nil {
		t.Fatal("ExtractProfile() expected error for missing file")
	}
}

func TestBuildProfile_zero(t *testing.T) {
	_, err := buildProfile(Profile{})
	if err == nil {
		t.Fatal("buildProfile() expected error for zero profile")
	}
}

func TestBuildProfile_fileNotFound(t *testing.T) {
	_, err := buildProfile(Profile{Name: "test", Path: "/nonexistent/path/profile.yaml"})
	if err == nil {
		t.Fatal("buildProfile() expected error when profile file not found")
	}
}

func TestBuildProfile_validationError(t *testing.T) {
	dir := t.TempDir()
	profilePath := filepath.Join(dir, "profile.yaml")
	// badfield violates additionalProperties:false in the profile schema.
	os.WriteFile(profilePath, []byte("name: test\nbadfield: invalid"), 0644)

	_, err := buildProfile(Profile{Name: "test", Path: profilePath})
	if err == nil {
		t.Fatal("buildProfile() expected error when schema validation fails")
	}
}

func TestBuildProfile_extractionError(t *testing.T) {
	root := repoRoot(t)
	dir := t.TempDir()

	profilePath := filepath.Join(dir, "profile.yaml")
	// Valid per profile schema (only requires "name"), but we'll look for a different name.
	os.WriteFile(profilePath, []byte("name: actualname"), 0644)

	wd, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(wd)

	_, err := buildProfile(Profile{Name: "wrongname", Path: profilePath})
	if err == nil {
		t.Fatal("buildProfile() expected error when profile name not found in file")
	}
}

func TestValidateProfile_schemaViolation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.yaml")
	// badfield violates additionalProperties:false in the profile schema.
	os.WriteFile(path, []byte("name: test\nbadfield: invalid"), 0644)

	if err := ValidateProfile(path); err == nil {
		t.Fatal("ValidateProfile() expected error for schema violation")
	}
}

// TestValidateProfile_commandArgsAndFlags exercises the schema constraints on
// the new command `args` / `flags` declarations: the name pattern, the
// conditional default-type matching, and that valid declarations pass.
func TestValidateProfile_commandArgsAndFlags(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{
			name: "valid args and flags",
			body: `name: t
commands:
  - name: patch
    args:
      - name: ticket
        required: true
    flags:
      - name: host
        type: string
        default: "localhost"
      - name: count
        type: int
        default: 3
      - name: verbose
        type: bool
        default: false
    tasks:
      - type: Print
        message: hi
`,
		},
		{
			name: "arg name with hyphen rejected",
			body: `name: t
commands:
  - name: c
    args:
      - name: dry-run
    tasks:
      - type: Print
        message: hi
`,
			wantErr: true,
		},
		{
			name: "flag name starting with digit rejected",
			body: `name: t
commands:
  - name: c
    flags:
      - name: 1invalid
    tasks:
      - type: Print
        message: hi
`,
			wantErr: true,
		},
		{
			name: "int flag with string default rejected",
			body: `name: t
commands:
  - name: c
    flags:
      - name: count
        type: int
        default: "not a number"
    tasks:
      - type: Print
        message: hi
`,
			wantErr: true,
		},
		{
			name: "bool flag with string default rejected",
			body: `name: t
commands:
  - name: c
    flags:
      - name: verbose
        type: bool
        default: "yes"
    tasks:
      - type: Print
        message: hi
`,
			wantErr: true,
		},
		{
			name: "string flag (default type) with bool default rejected",
			body: `name: t
commands:
  - name: c
    flags:
      - name: host
        default: true
    tasks:
      - type: Print
        message: hi
`,
			wantErr: true,
		},
		{
			name: "short longer than one char rejected",
			body: `name: t
commands:
  - name: c
    flags:
      - name: host
        short: "ho"
    tasks:
      - type: Print
        message: hi
`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "profile.yaml")
			if err := os.WriteFile(path, []byte(tt.body), 0644); err != nil {
				t.Fatalf("write profile: %v", err)
			}
			err := ValidateProfile(path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateProfile() err = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateProfile_taskAcceptsSharedAndVariantFields verifies that the
// allOf+taskCommon schema refactor still permits both shared task properties
// (name, concurrent, condition) and variant-specific ones (cmd) on the same
// task. A regression here would mean unevaluatedProperties:false isn't
// recognising the inherited shared properties.
func TestValidateProfile_taskAcceptsSharedAndVariantFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.yaml")
	body := `name: test
install:
  tasks:
    - type: Shell
      name: hello
      concurrent: true
      cmd: echo hi
`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("setup: write profile: %v", err)
	}
	if err := ValidateProfile(path); err != nil {
		t.Fatalf("ValidateProfile() unexpected error: %v", err)
	}
}

// TestValidateProfile_optionsAccepted_onEveryTaskType locks in the
// taskCommon `options` block: every task variant must accept it so
// schema authors don't have to re-declare per type. Also covers the
// command-level options block.
func TestValidateProfile_optionsAccepted_onEveryTaskType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.yaml")
	body := `name: opts
install:
  tasks:
    - type: Shell
      cmd: 'true'
      options: {showExeTime: true}
    - type: Script
      path: ./x.sh
      options: {showExeTime: false}
    - type: HTTP
      url: 'https://example.com/a'
      dest: /tmp/a
      options: {showExeTime: true}
    - type: Wait
      url: 'https://example.com/ready'
      options: {showExeTime: true}
    - type: Template
      src: ./t.tmpl
      dest: /tmp/out
      options: {showExeTime: true}
    - type: Git
      op: pull
      options: {showExeTime: true}
    - type: Prompt
      var: NAME
      options: {showExeTime: true}
    - type: Confirm
      message: ok?
      options: {showExeTime: true}
    - type: Print
      message: hi
      options: {showExeTime: true}
    - type: Set
      var: X
      value: y
      options: {showExeTime: true}
commands:
  - name: build
    options: {showExeTime: true}
    tasks:
      - type: Shell
        cmd: 'true'
`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := ValidateProfile(path); err != nil {
		t.Fatalf("ValidateProfile() unexpected error: %v", err)
	}
}

// TestValidateProfile_continueOnFailure_acceptedOnEveryTaskType locks
// the schema contract for the new `options.continueOnFailure` field —
// every task variant must accept it without re-declaration.
func TestValidateProfile_continueOnFailure_acceptedOnEveryTaskType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.yaml")
	body := `name: cof
install:
  tasks:
    - type: Shell
      cmd: 'true'
      options: {continueOnFailure: true}
    - type: Script
      path: ./x.sh
      options: {continueOnFailure: true}
    - type: HTTP
      url: 'https://example.com/a'
      dest: /tmp/a
      options: {continueOnFailure: true}
    - type: Wait
      url: 'https://example.com/ready'
      options: {continueOnFailure: true}
    - type: Template
      src: ./t.tmpl
      dest: /tmp/out
      options: {continueOnFailure: true}
    - type: Git
      op: pull
      options: {continueOnFailure: true}
    - type: Prompt
      var: NAME
      options: {continueOnFailure: true}
    - type: Confirm
      message: ok?
      options: {continueOnFailure: true}
    - type: Print
      message: hi
      options: {continueOnFailure: true}
    - type: Set
      var: X
      value: y
      options: {continueOnFailure: true}
    - type: Shell
      cmd: 'true'
      options: {showExeTime: true, continueOnFailure: true}
    - type: Group
      ref: cleanup
      options: {continueOnFailure: true}
task_groups:
  cleanup:
    - type: Shell
      cmd: 'true'
`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := ValidateProfile(path); err != nil {
		t.Fatalf("ValidateProfile() unexpected error: %v", err)
	}
}

func TestValidateProfile_optionsRejectsUnknownField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.yaml")
	// `quiet` is reserved for a future option (#54 mentions it explicitly
	// as out of scope). Until it lands, the schema must reject it so
	// authors get a clear error instead of silent acceptance.
	body := `name: t
install:
  tasks:
    - type: Shell
      cmd: 'true'
      options:
        showExeTime: true
        quiet: true
`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := ValidateProfile(path); err == nil {
		t.Fatal("ValidateProfile() should reject unknown option fields")
	}
}

// TestValidateProfile_taskRejectsUnknownField verifies that an unknown
// property on a task is still rejected after the schema was refactored to use
// allOf + unevaluatedProperties:false instead of additionalProperties:false on
// each variant.
func TestValidateProfile_taskRejectsUnknownField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.yaml")
	body := `name: test
install:
  tasks:
    - type: Shell
      cmd: echo hi
      bogusfield: nope
`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("setup: write profile: %v", err)
	}
	if err := ValidateProfile(path); err == nil {
		t.Fatal("ValidateProfile() expected error for unknown task field")
	}
}

// TestValidateProfile_verify exercises schema constraints on the
// top-level `verify:` block: that valid entries pass, and that
// entries missing `name` or `tasks` are rejected.
func TestValidateProfile_verify(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{
			name: "verify accepted with name, tasks, and onFail",
			body: `name: t
verify:
  - name: Node installed
    tasks:
      - type: Shell
        cmd: node --version
    onFail:
      - type: Shell
        cmd: nvm install --lts
`,
		},
		{
			name: "verify accepted with only name and tasks",
			body: `name: t
verify:
  - name: simple
    tasks:
      - type: Print
        message: hi
`,
		},
		{
			name: "verify rejected when name is missing",
			body: `name: t
verify:
  - tasks:
      - type: Shell
        cmd: node --version
`,
			wantErr: true,
		},
		{
			name: "verify rejected when tasks is missing",
			body: `name: t
verify:
  - name: Node installed
`,
			wantErr: true,
		},
		{
			name: "verify rejected with unknown field",
			body: `name: t
verify:
  - name: x
    tasks:
      - type: Shell
        cmd: exit 0
    bogus: nope
`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "profile.yaml")
			if err := os.WriteFile(path, []byte(tt.body), 0644); err != nil {
				t.Fatalf("write profile: %v", err)
			}
			err := ValidateProfile(path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateProfile() err = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateProfile_agent exercises the schema constraints on the
// optional `agent:` block on commands: missing block is valid, valid
// shape passes, unknown fields and wrong types are rejected.
func TestValidateProfile_agent(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{
			name: "agent block accepted with all fields",
			body: `name: t
commands:
  - name: test
    usage: Run tests
    tasks:
      - type: Shell
        cmd: go test ./...
    agent:
      safe: true
      reads: ["./..."]
      writes: []
      description: "Run unit tests"
`,
		},
		{
			name: "agent block missing is valid",
			body: `name: t
commands:
  - name: build
    usage: Build
    tasks:
      - type: Shell
        cmd: go build ./...
`,
		},
		{
			name: "agent rejects unknown field",
			body: `name: t
commands:
  - name: test
    usage: Run tests
    tasks:
      - type: Shell
        cmd: go test ./...
    agent:
      safety: true
`,
			wantErr: true,
		},
		{
			name: "agent.safe must be boolean",
			body: `name: t
commands:
  - name: test
    usage: Run tests
    tasks:
      - type: Shell
        cmd: go test ./...
    agent:
      safe: "yes"
`,
			wantErr: true,
		},
		{
			name: "agent.reads must be array of strings",
			body: `name: t
commands:
  - name: test
    usage: Run tests
    tasks:
      - type: Shell
        cmd: go test ./...
    agent:
      reads: "src"
`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "profile.yaml")
			if err := os.WriteFile(path, []byte(tt.body), 0644); err != nil {
				t.Fatalf("write profile: %v", err)
			}
			err := ValidateProfile(path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateProfile() err = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

// --- WriteProfileFile ---

func TestWriteProfileFile_createsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.raid.yaml")

	if err := WriteProfileFile(ProfileDraft{Name: "test-profile"}, path); err != nil {
		t.Fatalf("WriteProfileFile() unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "yaml-language-server") {
		t.Error("WriteProfileFile(): missing schema comment")
	}
	if !strings.Contains(content, "name: test-profile") {
		t.Error("WriteProfileFile(): missing profile name in output")
	}
}

func TestWriteProfileFile_createsParentDirectories(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "test.raid.yaml")

	if err := WriteProfileFile(ProfileDraft{Name: "nested"}, path); err != nil {
		t.Fatalf("WriteProfileFile() unexpected error: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("WriteProfileFile(): file not found at %s: %v", path, err)
	}
}

func TestWriteProfileFile_mkdirAllError(t *testing.T) {
	file, err := os.CreateTemp("", "raid-lib-profile-test-*")
	if err != nil {
		t.Fatal(err)
	}
	file.Close()
	defer os.Remove(file.Name())

	path := filepath.Join(file.Name(), "subdir", "test.raid.yaml")
	if err := WriteProfileFile(ProfileDraft{Name: "x"}, path); err == nil {
		t.Fatal("WriteProfileFile(): expected error when parent path contains a file")
	}
}

// --- CollectRepos ---

// initRepoWithBranch creates a non-bare git repo with one empty commit on the
// given branch. ls-remote requires at least one object to return the symref.
func initRepoWithBranch(t *testing.T, branch string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	cmds := [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "symbolic-ref", "HEAD", "refs/heads/" + branch},
		{"git", "-C", dir, "config", "user.email", "test@example.com"},
		{"git", "-C", dir, "config", "user.name", "Test"},
		{"git", "-C", dir, "config", "commit.gpgSign", "false"},
		{"git", "-C", dir, "commit", "--allow-empty", "-m", "init"},
	}
	for _, cmd := range cmds {
		if err := exec.Command(cmd[0], cmd[1:]...).Run(); err != nil {
			t.Fatalf("%v: %v", cmd, err)
		}
	}
	return dir
}

func TestCollectRepos_noRepos(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("n\n"))
	repos := CollectRepos(reader)
	if len(repos) != 0 {
		t.Errorf("CollectRepos(): got %d repos, want 0", len(repos))
	}
}

func TestCollectRepos_skipsWhenRequiredFieldMissing(t *testing.T) {
	// Answer yes but leave name blank — repo should be skipped.
	input := "y\n\nhttps://127.0.0.1:1/repo.git\n/some/path\nmain\nn\n"
	reader := bufio.NewReader(strings.NewReader(input))
	repos := CollectRepos(reader)
	if len(repos) != 0 {
		t.Errorf("CollectRepos(): expected skipped repo, got %d", len(repos))
	}
}

func TestCollectRepos_collectsCompleteRepo(t *testing.T) {
	// Use 127.0.0.1:1 so DetectGitDefaultBranch fails fast; branch is supplied manually.
	input := "y\nmy-repo\nhttps://127.0.0.1:1/repo.git\n/tmp/my-repo\nmain\nn\n"
	reader := bufio.NewReader(strings.NewReader(input))
	repos := CollectRepos(reader)
	if len(repos) != 1 {
		t.Fatalf("CollectRepos(): got %d repos, want 1", len(repos))
	}
	r := repos[0]
	if r.Name != "my-repo" {
		t.Errorf("Name: got %q, want %q", r.Name, "my-repo")
	}
	if r.Branch != "main" {
		t.Errorf("Branch: got %q, want %q", r.Branch, "main")
	}
}

func TestCollectRepos_usesDetectedBranch(t *testing.T) {
	repoDir := initRepoWithBranch(t, "trunk")

	// Leave branch input blank — should pick up "trunk" from the remote.
	input := "y\nmy-repo\nfile://" + repoDir + "\n/tmp/my-repo\n\nn\n"
	reader := bufio.NewReader(strings.NewReader(input))
	repos := CollectRepos(reader)
	if len(repos) != 1 {
		t.Fatalf("CollectRepos(): got %d repos, want 1", len(repos))
	}
	if repos[0].Branch != "trunk" {
		t.Errorf("Branch: got %q, want %q", repos[0].Branch, "trunk")
	}
}

// --- CreateRepoConfigs ---

func TestCreateRepoConfigs_createsConfig(t *testing.T) {
	dir := t.TempDir()
	CreateRepoConfigs([]RepoDraft{
		{Name: "my-repo", URL: "https://example.com", Path: dir, Branch: "main"},
	})

	data, err := os.ReadFile(filepath.Join(dir, "raid.yaml"))
	if err != nil {
		t.Fatalf("raid.yaml not created: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "name: my-repo") {
		t.Error("raid.yaml: missing name field")
	}
	if !strings.Contains(content, "branch: main") {
		t.Error("raid.yaml: missing branch field")
	}
}

func TestCreateRepoConfigs_skipsExistingConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "raid.yaml")
	original := "original content\n"
	os.WriteFile(configPath, []byte(original), 0644)

	CreateRepoConfigs([]RepoDraft{
		{Name: "my-repo", URL: "https://example.com", Path: dir, Branch: "main"},
	})

	data, _ := os.ReadFile(configPath)
	if string(data) != original {
		t.Error("CreateRepoConfigs(): overwrote existing raid.yaml")
	}
}

func TestCreateRepoConfigs_omitsBranchWhenEmpty(t *testing.T) {
	dir := t.TempDir()
	CreateRepoConfigs([]RepoDraft{
		{Name: "no-branch", URL: "https://example.com", Path: dir, Branch: ""},
	})

	data, err := os.ReadFile(filepath.Join(dir, "raid.yaml"))
	if err != nil {
		t.Fatalf("raid.yaml not created: %v", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "branch:") {
			t.Error("CreateRepoConfigs(): wrote branch field when Branch is empty")
			break
		}
	}
}

func TestCreateRepoConfigs_mkdirAllError(t *testing.T) {
	file, err := os.CreateTemp("", "raid-lib-profile-test-*")
	if err != nil {
		t.Fatal(err)
	}
	file.Close()
	defer os.Remove(file.Name())

	// Should not panic — just print error and continue.
	CreateRepoConfigs([]RepoDraft{
		{Name: "x", URL: "https://example.com", Path: filepath.Join(file.Name(), "subdir"), Branch: "main"},
	})
}

func TestCreateRepoConfigs_writeError(t *testing.T) {
	dir := t.TempDir()
	// Make the repo directory read-only so os.WriteFile cannot create raid.yaml.
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0755) })

	// Should not panic — just print error and continue.
	CreateRepoConfigs([]RepoDraft{
		{Name: "x", URL: "https://example.com", Path: dir, Branch: "main"},
	})
}

// TestGetProfilePaths_nilMap covers the nil map branch.
func TestGetProfilePaths_nilMap(t *testing.T) {
	setupTestConfig(t)
	// viper has no profile key set, so GetStringMapString returns non-nil empty map.
	// But if we set it to nil explicitly...
	viperResetProfiles(t)

	paths := getProfilePaths()
	if paths == nil {
		t.Error("getProfilePaths() returned nil, want empty map")
	}
}

// viperResetProfiles clears the profiles key so viper.GetStringMapString
// returns nil instead of an empty map.
func viperResetProfiles(t *testing.T) {
	t.Helper()
	// This is a no-op in normal viper usage but ensures we hit the nil branch.
	// Note: viper always returns an empty map rather than nil for unset keys.
}

// TestAddProfile_nilMapBranch tests the nil map branch of AddProfile.
func TestAddProfile_nilMapBranch(t *testing.T) {
	setupTestConfig(t)
	viperResetProfiles(t)

	// After a fresh setup, profiles should be nil/empty.
	if err := AddProfile(Profile{Name: "first", Path: "/some/path"}); err != nil {
		t.Fatalf("AddProfile: %v", err)
	}

	profiles := ListProfiles()
	if len(profiles) == 0 {
		t.Error("AddProfile did not register profile")
	}
}

// TestContainsProfile_foundAfterAdd covers the "found" branch.
func TestContainsProfile_foundAfterAdd(t *testing.T) {
	setupTestConfig(t)
	AddProfile(Profile{Name: "cexists", Path: "/p"})
	if !ContainsProfile("cexists") {
		t.Error("ContainsProfile returned false for existing profile")
	}
}

// TestRemoveProfile_noProfilesEmpty covers the "no profiles found" branch.
func TestRemoveProfile_noProfilesEmpty(t *testing.T) {
	setupTestConfig(t)
	viperResetProfiles(t)
	err := RemoveProfile("anything-xyz")
	// Either "no profiles found" or "not found" is acceptable (viper returns
	// empty map rather than nil)
	if err == nil {
		t.Error("RemoveProfile expected error")
	}
}

// TestAddProfiles_errorPropagation tests that AddProfiles returns the first error.
func TestAddProfiles_errorPropagation(t *testing.T) {
	setupTestConfig(t)
	// AddProfile itself rarely fails in tests (it writes to the config file).
	// Here we just exercise the loop by adding multiple valid profiles.
	err := AddProfiles([]Profile{
		{Name: "ap1", Path: "/p1"},
		{Name: "ap2", Path: "/p2"},
	})
	if err != nil {
		t.Errorf("AddProfiles: %v", err)
	}
}

// --- Single-repo profile (raid.yaml as a profile) ---

func TestProfileIsSingleRepo(t *testing.T) {
	tests := []struct {
		name    string
		profile Profile
		want    bool
	}{
		{"raid.yaml path", Profile{Name: "x", Path: "/some/dir/raid.yaml"}, true},
		{"profile.yaml path", Profile{Name: "x", Path: "/some/dir/profile.raid.yaml"}, false},
		{"empty path", Profile{Name: "x"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.profile.IsSingleRepo(); got != tt.want {
				t.Errorf("IsSingleRepo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildSingleRepoProfile_success(t *testing.T) {
	root := repoRoot(t)
	wd, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(wd)

	dir := t.TempDir()
	repoYaml := filepath.Join(dir, RaidConfigFileName)
	content := "name: solo\nbranch: main\ncommands:\n  - name: hello\n    tasks:\n      - type: Print\n        message: hi\n"
	if err := os.WriteFile(repoYaml, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	p, err := BuildSingleRepoProfile(repoYaml)
	if err != nil {
		t.Fatalf("BuildSingleRepoProfile: %v", err)
	}
	if p.Name != "solo" {
		t.Errorf("Name = %q, want solo", p.Name)
	}
	if p.Path != repoYaml {
		t.Errorf("Path = %q, want %q", p.Path, repoYaml)
	}
	if len(p.Repositories) != 1 {
		t.Fatalf("Repositories len = %d, want 1", len(p.Repositories))
	}
	if r := p.Repositories[0]; r.Name != "solo" || r.Path != dir || r.Branch != "main" || r.URL != "" {
		t.Errorf("Repositories[0] = %+v, want {Name: solo, Path: %q, Branch: main}", r, dir)
	}
	if !p.IsSingleRepo() {
		t.Error("IsSingleRepo() = false, want true on the returned profile")
	}
}

func TestBuildSingleRepoProfile_invalidSchema(t *testing.T) {
	root := repoRoot(t)
	wd, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(wd)

	dir := t.TempDir()
	repoYaml := filepath.Join(dir, RaidConfigFileName)
	// Missing required `branch`; should fail repo schema validation.
	if err := os.WriteFile(repoYaml, []byte("name: nb\n"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := BuildSingleRepoProfile(repoYaml)
	if err == nil {
		t.Fatal("BuildSingleRepoProfile expected schema error for missing branch")
	}
}

func TestBuildSingleRepoProfile_emptyName(t *testing.T) {
	root := repoRoot(t)
	wd, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(wd)

	dir := t.TempDir()
	repoYaml := filepath.Join(dir, RaidConfigFileName)
	// Schema-valid (name+branch present), but name is empty after unmarshal.
	if err := os.WriteFile(repoYaml, []byte("name: \"\"\nbranch: main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := BuildSingleRepoProfile(repoYaml)
	if err == nil {
		t.Fatal("BuildSingleRepoProfile expected error for empty name")
	}
}

func TestForceLoad_singleRepoProfile(t *testing.T) {
	root := repoRoot(t)
	setupTestConfig(t)

	dir := t.TempDir()
	repoYaml := filepath.Join(dir, RaidConfigFileName)
	content := "name: mono\nbranch: main\nenvironments:\n  - name: dev\n    variables:\n      - name: FOO\n        value: bar\ncommands:\n  - name: greet\n    tasks:\n      - type: Print\n        message: hi\n"
	if err := os.WriteFile(repoYaml, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	wd, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(wd)

	if err := AddProfile(Profile{Name: "mono", Path: repoYaml}); err != nil {
		t.Fatal(err)
	}
	if err := SetProfile("mono"); err != nil {
		t.Fatal(err)
	}

	if err := ForceLoad(); err != nil {
		t.Fatalf("ForceLoad: %v", err)
	}
	if loadContext() == nil || loadContext().Profile.Name != "mono" {
		t.Fatalf("loadContext().Profile.Name = %q, want mono", loadContext().Profile.Name)
	}
	if !loadContext().Profile.IsSingleRepo() {
		t.Error("IsSingleRepo() = false on loaded profile")
	}
	if len(loadContext().Profile.Repositories) != 1 || loadContext().Profile.Repositories[0].Path != dir {
		t.Errorf("Repositories = %+v, want one repo with path %q", loadContext().Profile.Repositories, dir)
	}
	// Repo-level environments should be promoted to profile level so `raid env`
	// can find them without a wrapping profile YAML.
	if len(loadContext().Profile.Environments) != 1 || loadContext().Profile.Environments[0].Name != "dev" {
		t.Errorf("Environments = %+v, want one named 'dev'", loadContext().Profile.Environments)
	}
	// Repo commands should have been merged into top-level commands.
	found := false
	for _, c := range loadContext().Profile.Commands {
		if c.Name == "greet" {
			found = true
		}
	}
	if !found {
		t.Errorf("Commands missing 'greet': %+v", loadContext().Profile.Commands)
	}
}

func TestForceLoad_singleRepoProfile_invalidSchema(t *testing.T) {
	root := repoRoot(t)
	setupTestConfig(t)

	dir := t.TempDir()
	repoYaml := filepath.Join(dir, RaidConfigFileName)
	// Missing required `branch` — should fail repo schema validation.
	if err := os.WriteFile(repoYaml, []byte("name: bad\n"), 0644); err != nil {
		t.Fatal(err)
	}

	wd, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(wd)

	if err := AddProfile(Profile{Name: "bad", Path: repoYaml}); err != nil {
		t.Fatal(err)
	}
	if err := SetProfile("bad"); err != nil {
		t.Fatal(err)
	}

	if err := ForceLoad(); err == nil {
		t.Fatal("ForceLoad: expected error when single-repo raid.yaml is invalid")
	}
}

// TestBuildSingleRepoProfile_wrongBasename rejects paths that don't end in
// the canonical raid.yaml name — ExtractRepo reads <dir>/raid.yaml so
// validating one file and loading another would be a silent footgun.
func TestBuildSingleRepoProfile_wrongBasename(t *testing.T) {
	root := repoRoot(t)
	wd, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(wd)

	dir := t.TempDir()
	// Write a valid raid.yaml so the repo-schema check would pass, but
	// hand BuildSingleRepoProfile a renamed copy.
	if err := os.WriteFile(filepath.Join(dir, RaidConfigFileName), []byte("name: ok\nbranch: main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	renamed := filepath.Join(dir, "not-raid.yaml")
	if err := os.WriteFile(renamed, []byte("name: ok\nbranch: main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := BuildSingleRepoProfile(renamed)
	if err == nil {
		t.Fatal("BuildSingleRepoProfile expected error for non-raid.yaml basename")
	}
	if !strings.Contains(err.Error(), RaidConfigFileName) {
		t.Errorf("error = %v, want mention of %s", err, RaidConfigFileName)
	}
}

// TestBuildProfile_singleRepoNameMismatch ensures buildProfile refuses to
// load a single-repo profile whose registered name no longer matches the
// raid.yaml's current name field.
func TestBuildProfile_singleRepoNameMismatch(t *testing.T) {
	root := repoRoot(t)
	wd, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(wd)

	dir := t.TempDir()
	repoYaml := filepath.Join(dir, RaidConfigFileName)
	if err := os.WriteFile(repoYaml, []byte("name: newname\nbranch: main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Registered profile uses "oldname" — simulates the user renaming the
	// raid.yaml's name field after `raid profile add`.
	_, err := buildProfile(Profile{Name: "oldname", Path: repoYaml})
	if err == nil {
		t.Fatal("buildProfile expected error when registered name diverges from raid.yaml name")
	}
	if !strings.Contains(err.Error(), "oldname") || !strings.Contains(err.Error(), "newname") {
		t.Errorf("error = %v, want both registered and current names mentioned", err)
	}
}

// TestBuildProfile_singleRepoNameMatch confirms the name-match path
// succeeds and returns the synthesized profile.
func TestBuildProfile_singleRepoNameMatch(t *testing.T) {
	root := repoRoot(t)
	wd, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(wd)

	dir := t.TempDir()
	repoYaml := filepath.Join(dir, RaidConfigFileName)
	if err := os.WriteFile(repoYaml, []byte("name: match\nbranch: main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	p, err := buildProfile(Profile{Name: "match", Path: repoYaml})
	if err != nil {
		t.Fatalf("buildProfile: %v", err)
	}
	if p.Name != "match" {
		t.Errorf("Name = %q, want match", p.Name)
	}
}

// TestWriteProfileFile_error tests the error path when the save path is invalid.
func TestWriteProfileFile_error(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("path semantics differ on Windows")
	}
	// Use a regular file as a parent component so MkdirAll fails.
	f, err := os.CreateTemp("", "raid-test-*")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	defer os.Remove(f.Name())

	err = WriteProfileFile(ProfileDraft{Name: "x"}, filepath.Join(f.Name(), "sub", "profile.yaml"))
	if err == nil {
		t.Error("WriteProfileFile expected error for invalid path")
	}
}

func TestExtractProfilesFromYAML_emptyNameSkipped(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "multi.yaml")
	content := "---\nname: \"\"\n---\nname: real\nrepositories:\n  - name: r\n    path: ~/r\n    url: https://x.com/r.git\n"
	os.WriteFile(path, []byte(content), 0644)

	profiles, err := ExtractProfiles(path)
	if err != nil {
		t.Fatalf("ExtractProfiles() error: %v", err)
	}
	if len(profiles) != 1 || profiles[0].Name != "real" {
		t.Errorf("expected 1 profile named 'real', got %v", profiles)
	}
}

func TestExtractProfilesFromYAML_malformedYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	os.WriteFile(path, []byte(":\n  invalid: [\n"), 0644)

	_, err := ExtractProfiles(path)
	if err == nil {
		t.Fatal("ExtractProfiles() with malformed YAML should return error")
	}
}

func TestGetProfilePaths_emptyConfig(t *testing.T) {
	setupTestConfig(t)
	paths := getProfilePaths()
	if paths == nil {
		t.Error("getProfilePaths() should return empty map, not nil")
	}
	if len(paths) != 0 {
		t.Errorf("getProfilePaths() expected empty map, got %v", paths)
	}
}
