package lib

import (
	"os"
	"path/filepath"
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

	context = &Context{
		Profile: Profile{Name: "test", Path: "/path"},
	}

	envs := ListEnvs()
	if len(envs) != 0 {
		t.Errorf("ListEnvs() with no environments = %v, want empty slice", envs)
	}
}

func TestListEnvs_withEnvironments(t *testing.T) {
	setupTestConfig(t)

	context = &Context{
		Profile: Profile{
			Environments: []Env{
				{Name: "dev"},
				{Name: "prod"},
			},
		},
	}

	envs := ListEnvs()
	if len(envs) != 2 {
		t.Fatalf("ListEnvs() = %v, want 2 environments", envs)
	}
}

func TestContainsEnv(t *testing.T) {
	setupTestConfig(t)

	context = &Context{
		Profile: Profile{
			Environments: []Env{
				{Name: "dev"},
			},
		},
	}

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

	context = &Context{
		Profile: Profile{
			Environments: []Env{{Name: "dev"}},
		},
	}

	err := SetEnv("nonexistent")
	if err == nil {
		t.Fatal("SetEnv() expected error for nonexistent env")
	}
}

func TestSetAndGetEnv(t *testing.T) {
	setupTestConfig(t)

	context = &Context{
		Profile: Profile{
			Environments: []Env{{Name: "dev"}},
		},
	}

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

	context = &Context{
		Profile: Profile{
			Name: "test",
			Path: "/test",
			Repositories: []Repo{
				{Name: "repo1", Path: tmpFile.Name(), URL: "http://x.com"},
			},
		},
	}

	err = ExecuteEnv("dev")
	if err == nil {
		t.Fatal("ExecuteEnv() expected error when buildEnvPath fails")
	}
}

func TestExecuteEnv_taskFailure(t *testing.T) {
	setupTestConfig(t)

	dir := t.TempDir()

	context = &Context{
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
	}

	err := ExecuteEnv("dev")
	if err == nil {
		t.Fatal("ExecuteEnv() expected error from failing task")
	}
}

func TestExecuteEnv_success(t *testing.T) {
	setupTestConfig(t)

	dir := t.TempDir()

	context = &Context{
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
	}

	if err := ExecuteEnv("dev"); err != nil {
		t.Errorf("ExecuteEnv() error: %v", err)
	}
}

func TestExecuteEnv_noMatchingEnvTasks(t *testing.T) {
	setupTestConfig(t)

	dir := t.TempDir()

	context = &Context{
		Profile: Profile{
			Name: "test",
			Path: "/test",
			Repositories: []Repo{
				{Name: "repo1", Path: dir, URL: "http://x.com"},
			},
			// No environments defined — env name won't match anything.
		},
	}

	// Runs setEnvVariablesForRepos (empty vars) then runTasksForEnv (zero env, returns nil).
	if err := ExecuteEnv("nonexistent"); err != nil {
		t.Errorf("ExecuteEnv() with no matching env error: %v", err)
	}
}

func TestLoadEnv_nilContext(t *testing.T) {
	context = nil

	err := LoadEnv()
	if err == nil {
		t.Fatal("LoadEnv() expected error for nil context")
	}
}

func TestLoadEnv_noEnvFiles(t *testing.T) {
	setupTestConfig(t)

	context = &Context{
		Profile: Profile{
			Repositories: []Repo{
				{Name: "repo1", Path: "/nonexistent/path"},
			},
		},
	}

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

	context = &Context{
		Profile: Profile{
			Repositories: []Repo{
				{Name: "repo1", Path: dir},
			},
		},
	}

	if err := LoadEnv(); err != nil {
		t.Errorf("LoadEnv() with .env files error: %v", err)
	}
}

func TestExecuteEnv_repoEnvVars(t *testing.T) {
	setupTestConfig(t)

	dir := t.TempDir()

	context = &Context{
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
	}

	if err := ExecuteEnv("dev"); err != nil {
		t.Errorf("ExecuteEnv() with repo env vars error: %v", err)
	}
}

func TestExecuteEnv_setEnvWriteError(t *testing.T) {
	setupTestConfig(t)

	dir := t.TempDir()
	// Pre-create .env as read-only so godotenv.Write fails.
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte(""), 0444); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(envPath, 0644)

	context = &Context{
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
	}

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

	context = &Context{
		Profile: Profile{
			Repositories: []Repo{
				{Name: "repo1", Path: dir},
			},
		},
	}

	if err := LoadEnv(); err == nil {
		t.Fatal("LoadEnv() expected error when .env is a directory")
	}
}

func TestLoadEnv_emptyRepositories(t *testing.T) {
	setupTestConfig(t)

	context = &Context{
		Profile: Profile{},
	}

	if err := LoadEnv(); err != nil {
		t.Errorf("LoadEnv() with empty repositories error: %v", err)
	}
}
