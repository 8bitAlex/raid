package lib

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvIsZero(t *testing.T) {
	tests := []struct {
		name string
		env  Env
		want bool
	}{
		{"empty env", Env{}, true},
		{"named env", Env{Name: "dev"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.env.IsZero(); got != tt.want {
				t.Errorf("IsZero() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestListEnvs_nilContext(t *testing.T) {
	setupTestConfig(t)

	envs := ListEnvs()
	if len(envs) != 0 {
		t.Errorf("ListEnvs() with nil context = %v, want empty slice", envs)
	}
}

func TestListEnvs_emptyEnvironments(t *testing.T) {
	setupTestConfig(t)

	storeContext(&Context{
		Profile: Profile{Name: "test", Path: "/path"},
	})

	envs := ListEnvs()
	if len(envs) != 0 {
		t.Errorf("ListEnvs() with no environments = %v, want empty slice", envs)
	}
}

func TestListEnvs_withEnvironments(t *testing.T) {
	setupTestConfig(t)

	storeContext(&Context{
		Profile: Profile{
			Environments: []Env{
				{Name: "dev"},
				{Name: "prod"},
			},
		},
	})

	envs := ListEnvs()
	if len(envs) != 2 {
		t.Fatalf("ListEnvs() = %v, want 2 environments", envs)
	}
}

func TestContainsEnv(t *testing.T) {
	setupTestConfig(t)

	storeContext(&Context{
		Profile: Profile{
			Environments: []Env{
				{Name: "dev"},
			},
		},
	})

	if !ContainsEnv("dev") {
		t.Error("ContainsEnv(\"dev\") = false, want true")
	}
	if ContainsEnv("prod") {
		t.Error("ContainsEnv(\"prod\") = true, want false")
	}
}

func TestSetEnv_emptyName(t *testing.T) {
	setupTestConfig(t)

	err := SetEnv("")
	if err == nil {
		t.Fatal("SetEnv(\"\") expected error, got nil")
	}
}

func TestSetEnv_notFound(t *testing.T) {
	setupTestConfig(t)

	storeContext(&Context{
		Profile: Profile{
			Environments: []Env{{Name: "dev"}},
		},
	})

	err := SetEnv("nonexistent")
	if err == nil {
		t.Fatal("SetEnv() expected error for nonexistent env")
	}
}

func TestSetAndGetEnv(t *testing.T) {
	setupTestConfig(t)

	storeContext(&Context{
		Profile: Profile{
			Environments: []Env{{Name: "dev"}},
		},
	})

	if err := SetEnv("dev"); err != nil {
		t.Fatalf("SetEnv() error: %v", err)
	}

	got := GetEnv()
	if got != "dev" {
		t.Errorf("GetEnv() = %q, want %q", got, "dev")
	}
}

func TestExecuteEnv_buildEnvPathError(t *testing.T) {
	setupTestConfig(t)

	// Use a regular file as the repo path; .env cannot be created inside a file.
	tmpFile, err := os.CreateTemp("", "raid-test-*")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	storeContext(&Context{
		Profile: Profile{
			Name: "test",
			Path: "/test",
			Repositories: []Repo{
				{Name: "repo1", Path: tmpFile.Name(), URL: "http://x.com"},
			},
		},
	})

	err = ExecuteEnv("dev")
	if err == nil {
		t.Fatal("ExecuteEnv() expected error when buildEnvPath fails")
	}
}

func TestExecuteEnv_taskFailure(t *testing.T) {
	setupTestConfig(t)

	dir := t.TempDir()

	storeContext(&Context{
		Profile: Profile{
			Name: "test",
			Path: "/test",
			Repositories: []Repo{
				{Name: "repo1", Path: dir, URL: "http://x.com"},
			},
			Environments: []Env{
				{
					Name:  "dev",
					Tasks: []Task{{Type: Shell, Cmd: "exit 1"}},
				},
			},
		},
	})

	err := ExecuteEnv("dev")
	if err == nil {
		t.Fatal("ExecuteEnv() expected error from failing task")
	}
}

func TestExecuteEnv_success(t *testing.T) {
	setupTestConfig(t)

	dir := t.TempDir()

	storeContext(&Context{
		Profile: Profile{
			Name: "test",
			Path: "/test",
			Repositories: []Repo{
				{Name: "repo1", Path: dir, URL: "http://x.com"},
			},
			Environments: []Env{
				{
					Name:      "dev",
					Variables: []EnvVar{{Name: "APP_ENV", Value: "development"}},
				},
			},
		},
	})

	if err := ExecuteEnv("dev"); err != nil {
		t.Errorf("ExecuteEnv() error: %v", err)
	}
}

func TestExecuteEnv_noMatchingEnvTasks(t *testing.T) {
	setupTestConfig(t)

	dir := t.TempDir()

	storeContext(&Context{
		Profile: Profile{
			Name: "test",
			Path: "/test",
			Repositories: []Repo{
				{Name: "repo1", Path: dir, URL: "http://x.com"},
			},
			// No environments defined — env name won't match anything.
		},
	})

	// Runs setEnvVariablesForRepos (empty vars) then runTasksForEnv (zero env, returns nil).
	if err := ExecuteEnv("nonexistent"); err != nil {
		t.Errorf("ExecuteEnv() with no matching env error: %v", err)
	}
}

func TestLoadEnv_nilContext(t *testing.T) {
	storeContext(nil)

	err := LoadEnv()
	if err == nil {
		t.Fatal("LoadEnv() expected error for nil context")
	}
}

func TestLoadEnv_noEnvFiles(t *testing.T) {
	setupTestConfig(t)

	storeContext(&Context{
		Profile: Profile{
			Repositories: []Repo{
				{Name: "repo1", Path: "/nonexistent/path"},
			},
		},
	})

	if err := LoadEnv(); err != nil {
		t.Errorf("LoadEnv() with no .env files error: %v", err)
	}
}

func TestLoadEnv_withEnvFiles(t *testing.T) {
	setupTestConfig(t)

	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	if err := os.WriteFile(envFile, []byte("RAID_LOAD_TEST=hello\n"), 0644); err != nil {
		t.Fatal(err)
	}
	defer os.Unsetenv("RAID_LOAD_TEST")

	storeContext(&Context{
		Profile: Profile{
			Repositories: []Repo{
				{Name: "repo1", Path: dir},
			},
		},
	})

	if err := LoadEnv(); err != nil {
		t.Errorf("LoadEnv() with .env files error: %v", err)
	}
}

func TestExecuteEnv_repoEnvVars(t *testing.T) {
	setupTestConfig(t)

	dir := t.TempDir()

	storeContext(&Context{
		Profile: Profile{
			Name: "test",
			Path: "/test",
			Repositories: []Repo{
				{
					Name: "repo1",
					Path: dir,
					URL:  "http://x.com",
					// Repo has its own env vars for "dev" — exercises the repoVars loop.
					Environments: []Env{
						{
							Name:      "dev",
							Variables: []EnvVar{{Name: "REPO_SPECIFIC", Value: "repo_val"}},
						},
					},
				},
			},
			Environments: []Env{
				{
					Name:      "dev",
					Variables: []EnvVar{{Name: "PROFILE_VAR", Value: "prof_val"}},
				},
			},
		},
	})

	if err := ExecuteEnv("dev"); err != nil {
		t.Errorf("ExecuteEnv() with repo env vars error: %v", err)
	}
}

func TestExecuteEnv_setEnvWriteError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("file permissions not enforced as root")
	}
	setupTestConfig(t)

	dir := t.TempDir()
	// Pre-create .env as read-only so godotenv.Write fails.
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte(""), 0444); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(envPath, 0644)

	storeContext(&Context{
		Profile: Profile{
			Name: "test",
			Path: "/test",
			Repositories: []Repo{
				{Name: "repo1", Path: dir, URL: "http://x.com"},
			},
			Environments: []Env{
				{Name: "dev", Variables: []EnvVar{{Name: "KEY", Value: "val"}}},
			},
		},
	})

	if err := ExecuteEnv("dev"); err == nil {
		t.Fatal("ExecuteEnv() expected error when .env is read-only")
	}
}

func TestLoadEnv_loadFailure(t *testing.T) {
	setupTestConfig(t)

	dir := t.TempDir()
	// Create a directory named .env — godotenv.Load will fail reading it as a file.
	fakeEnvPath := filepath.Join(dir, ".env")
	if err := os.MkdirAll(fakeEnvPath, 0755); err != nil {
		t.Fatal(err)
	}

	storeContext(&Context{
		Profile: Profile{
			Repositories: []Repo{
				{Name: "repo1", Path: dir},
			},
		},
	})

	if err := LoadEnv(); err == nil {
		t.Fatal("LoadEnv() expected error when .env is a directory")
	}
}

func TestLoadEnv_emptyRepositories(t *testing.T) {
	setupTestConfig(t)

	storeContext(&Context{
		Profile: Profile{},
	})

	if err := LoadEnv(); err != nil {
		t.Errorf("LoadEnv() with empty repositories error: %v", err)
	}
}

func TestMergeEnvironments_baseWinsOnConflict(t *testing.T) {
	base := []Env{
		{Name: "dev", Tasks: []Task{{Cmd: "echo base-dev"}}},
		{Name: "prod"},
	}
	additional := []Env{
		// "dev" conflicts — base must win, additional dropped.
		{Name: "dev", Tasks: []Task{{Cmd: "echo additional-dev"}}},
		// "staging" is new — appended.
		{Name: "staging"},
	}

	got := mergeEnvironments(base, additional)

	if len(got) != 3 {
		t.Fatalf("merged len = %d, want 3", len(got))
	}
	if got[0].Name != "dev" || len(got[0].Tasks) == 0 || got[0].Tasks[0].Cmd != "echo base-dev" {
		t.Errorf("base.dev not preserved on conflict: %+v", got[0])
	}
	if got[1].Name != "prod" {
		t.Errorf("got[1].Name = %q, want prod", got[1].Name)
	}
	if got[2].Name != "staging" {
		t.Errorf("got[2].Name = %q, want staging (non-conflict appended)", got[2].Name)
	}
}

func TestMergeEnvironments_emptyBase(t *testing.T) {
	additional := []Env{{Name: "dev"}}
	got := mergeEnvironments(nil, additional)
	if len(got) != 1 || got[0].Name != "dev" {
		t.Errorf("merge into nil base = %+v, want [{Name:dev}]", got)
	}
}

func TestMergeEnvironments_emptyAdditional(t *testing.T) {
	base := []Env{{Name: "dev"}}
	got := mergeEnvironments(base, nil)
	if len(got) != 1 || got[0].Name != "dev" {
		t.Errorf("merge with nil additional = %+v, want [{Name:dev}]", got)
	}
}

func TestExecuteEnv_nilContext(t *testing.T) {
	old := loadContext()
	storeContext(nil)
	defer func() { storeContext(old) }()

	err := ExecuteEnv("dev")
	if err == nil {
		t.Fatal("ExecuteEnv() expected error with nil context, got nil")
	}
}

func TestListEnvs_includesRepoDeclaredEnvs(t *testing.T) {
	storeContext(&Context{
		Profile: Profile{
			Name: "test",
			Path: "/test",
			Environments: []Env{
				{Name: "dev"},
			},
			Repositories: []Repo{
				{Name: "repo1", Path: "/tmp/repo1", Environments: []Env{
					{Name: "dev"},        // duplicate — folded
					{Name: "production"}, // repo-only — must appear
				}},
			},
		},
	})
	defer func() { storeContext(nil) }()

	got := ListEnvs()
	want := []string{"dev", "production"}
	if len(got) != len(want) {
		t.Fatalf("ListEnvs() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ListEnvs()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if !ContainsEnv("production") {
		t.Error("ContainsEnv(production) = false, want true for repo-declared env")
	}
}

func TestExecuteEnv_runsRepoDeclaredEnvTasks(t *testing.T) {
	setupTestConfig(t)

	repoDir := t.TempDir()
	marker := "repo-env-task-ran"

	storeContext(&Context{
		Profile: Profile{
			Name: "test",
			Path: "/test/profile.yaml",
			Repositories: []Repo{
				{Name: "repo1", Path: repoDir, URL: "http://x.com", Environments: []Env{
					{
						Name: "dev",
						// No explicit path: must default to the repo dir.
						Tasks: []Task{{Type: Shell, Cmd: "echo done > " + marker}},
					},
				}},
			},
		},
	})
	defer func() { storeContext(nil) }()

	if err := ExecuteEnv("dev"); err != nil {
		t.Fatalf("ExecuteEnv() error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoDir, marker)); err != nil {
		t.Error("repo-declared env task did not run in the repo directory")
	}
}

func TestExecuteEnv_repoEnvTaskFailurePropagates(t *testing.T) {
	setupTestConfig(t)

	repoDir := t.TempDir()
	storeContext(&Context{
		Profile: Profile{
			Name: "test",
			Path: "/test/profile.yaml",
			Repositories: []Repo{
				{Name: "repo1", Path: repoDir, URL: "http://x.com", Environments: []Env{
					{Name: "dev", Tasks: []Task{{Type: Shell, Cmd: "exit 1"}}},
				}},
			},
		},
	})
	defer func() { storeContext(nil) }()

	if err := ExecuteEnv("dev"); err == nil {
		t.Fatal("expected error from failing repo env task")
	}
}

func TestExecuteEnv_skipsUninstalledRepos(t *testing.T) {
	setupTestConfig(t)

	missing := filepath.Join(t.TempDir(), "not-cloned-yet")
	var outBuf, errBuf bytes.Buffer
	restore := SetCommandOutput(&outBuf, &errBuf)
	defer restore()

	storeContext(&Context{
		Profile: Profile{
			Name: "test",
			Path: "/test/profile.yaml",
			Repositories: []Repo{
				{Name: "repo1", Path: missing, URL: "http://x.com"},
			},
			Environments: []Env{
				{Name: "dev", Variables: []EnvVar{{Name: "A", Value: "1"}}},
			},
		},
	})
	defer func() { storeContext(nil) }()

	if err := ExecuteEnv("dev"); err != nil {
		t.Fatalf("ExecuteEnv() error: %v", err)
	}
	// The repo dir must NOT have been created — a pre-created dir would
	// make the eventual `git clone` fail.
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Error("uninstalled repo dir was created by raid env")
	}
	if !strings.Contains(errBuf.String(), "Skipping repo repo1") {
		t.Errorf("expected skip notice on stderr, got %q", errBuf.String())
	}
}

func TestRunTasksForEnv_singleRepoRunsOnceInRepoDir(t *testing.T) {
	setupTestConfig(t)

	repoDir := t.TempDir()
	raidPath := filepath.Join(repoDir, RaidConfigFileName)
	if err := os.WriteFile(raidPath, []byte("name: solo\n"), 0644); err != nil {
		t.Fatal(err)
	}

	env := Env{
		Name:  "dev",
		Tasks: []Task{{Type: Shell, Cmd: "echo run >> single-repo-count"}},
	}
	// Mirror ForceLoad's single-repo hoist: env present at BOTH the
	// profile level and the repo level. It must run exactly once.
	storeContext(&Context{
		Profile: Profile{
			Name:         "solo",
			Path:         raidPath, // basename raid.yaml → IsSingleRepo
			Environments: []Env{env},
			Repositories: []Repo{
				{Name: "solo", Path: repoDir, Environments: []Env{env}},
			},
		},
	})
	defer func() { storeContext(nil) }()

	if err := ExecuteEnv("dev"); err != nil {
		t.Fatalf("ExecuteEnv() error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(repoDir, "single-repo-count"))
	if err != nil {
		t.Fatalf("marker not written in repo dir: %v", err)
	}
	if got := strings.Count(string(data), "run"); got != 1 {
		t.Errorf("env tasks ran %d times, want exactly once", got)
	}
}
