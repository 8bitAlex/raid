// The lib package is the implementation of the core functionality of the raid CLI tool.
package lib

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/8bitalex/raid/schemas"
	sys "github.com/8bitalex/raid/src/internal/sys"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

const (
	yamlSep            = "---"
	RaidConfigFileName = "raid.yaml"
)

// Context holds the active profile and environment for the current raid session.
type Context struct {
	Profile Profile
	Env     string
}

// OnInstall holds the tasks to run during profile installation.
type OnInstall struct {
	Tasks []Task `json:"tasks"`
}

var context *Context

// QuietLoad attempts a best-effort, read-only profile load. It does not create
// config files, does not emit warnings, and returns nil if the config is absent
// or loading fails. Intended for info-command paths (--help, --version) where
// user-command registration is opportunistic and side effects are undesirable.
func QuietLoad() []Command {
	if !initConfigReadOnly() {
		return nil
	}
	if err := ForceLoad(); err != nil {
		return nil
	}
	return GetCommands()
}

// Load initializes the context from the active profile, using cached results if available.
func Load() error {
	if context == nil {
		return ForceLoad()
	}
	return nil
}

// ForceLoad rebuilds the context from the active profile, ignoring any cached state.
func ForceLoad() error {
	p := GetProfile()
	if p.IsZero() {
		context = &Context{Env: GetEnv()}
		return nil
	}

	profile, err := buildProfile(p)
	if err != nil {
		return err
	}

	homeDir := sys.GetHomeDir()
	for i := range profile.Commands {
		profile.Commands[i].Tasks = withDefaultDir(profile.Commands[i].Tasks, homeDir)
	}
	for name, tasks := range profile.Groups {
		profile.Groups[name] = withDefaultDir(tasks, homeDir)
	}

	for i := range profile.Repositories {
		if err := buildRepo(&profile.Repositories[i]); err != nil {
			return err
		}
		repo := &profile.Repositories[i]
		repoDir := sys.ExpandPath(repo.Path)
		for j := range repo.Commands {
			repo.Commands[j].Tasks = withDefaultDir(repo.Commands[j].Tasks, repoDir)
		}
		profile.Commands = mergeCommands(profile.Commands, repo.Commands)
	}

	context = &Context{
		Profile: profile,
		Env:     GetEnv(),
	}
	return nil
}

// Install clones all repositories in the active profile and runs install tasks.
func Install(maxThreads int) error {
	if context == nil {
		return fmt.Errorf("raid context is not initialized")
	}
	profile := context.Profile
	if profile.IsZero() {
		return fmt.Errorf("profile not found")
	}

	var semaphore chan struct{}
	if maxThreads > 0 {
		semaphore = make(chan struct{}, maxThreads)
	}

	var wg sync.WaitGroup
	errorChan := make(chan error, len(profile.Repositories))

	for _, repo := range profile.Repositories {
		wg.Add(1)
		go func(repo Repo) {
			defer wg.Done()

			if semaphore != nil {
				semaphore <- struct{}{}
				defer func() { <-semaphore }()
			}

			if err := CloneRepository(repo); err != nil {
				errorChan <- fmt.Errorf("failed to install repository '%s': %w", repo.Name, err)
			}
		}(repo)
	}

	wg.Wait()
	close(errorChan)

	var errors []error
	for err := range errorChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("some repositories failed to install: %v", errors)
	}

	if err := ExecuteTasks(withDefaultDir(profile.Install.Tasks, sys.GetHomeDir())); err != nil {
		return fmt.Errorf("failed to execute install tasks: %w", err)
	}

	var repoTasks []Task
	for _, r := range profile.Repositories {
		repoTasks = append(repoTasks, withDefaultDir(r.Install.Tasks, sys.ExpandPath(r.Path))...)
	}
	if err := ExecuteTasks(repoTasks); err != nil {
		return fmt.Errorf("failed to execute repository install tasks: %w", err)
	}

	return nil
}

// ValidateSchema validates the file at path against the JSON schema at schemaPath.
// schemaPath must be an absolute or CWD-relative path to a schema file on disk.
func ValidateSchema(path string, schemaPath string) error {
	path = sys.ExpandPath(path)
	schemaPath = sys.ExpandPath(schemaPath)

	if path == "" || !sys.FileExists(path) {
		return fmt.Errorf("file not found at %s", path)
	}
	if schemaPath == "" || !sys.FileExists(schemaPath) {
		return fmt.Errorf("file not found at %s", schemaPath)
	}

	c := jsonschema.NewCompiler()
	sch, err := c.Compile(schemaPath)
	if err != nil {
		return err
	}

	return validateFile(path, sch)
}

// validateWithEmbeddedSchema validates path against a schema embedded in the binary.
// schemaName must be the bare filename of a schema in the embedded schemas directory
// (e.g. "raid-profile.schema.json"). All embedded schemas are registered so that
// cross-schema $ref values resolve correctly.
func validateWithEmbeddedSchema(path, schemaName string) error {
	path = sys.ExpandPath(path)
	if path == "" || !sys.FileExists(path) {
		return fmt.Errorf("file not found at %s", path)
	}

	c := jsonschema.NewCompiler()
	entries, err := schemas.FS.ReadDir(".")
	if err != nil {
		return fmt.Errorf("failed to read embedded schemas: %w", err)
	}
	for _, entry := range entries {
		data, err := schemas.FS.ReadFile(entry.Name())
		if err != nil {
			return fmt.Errorf("failed to read embedded schema %s: %w", entry.Name(), err)
		}
		var doc any
		if err := json.Unmarshal(data, &doc); err != nil {
			return fmt.Errorf("failed to parse embedded schema %s: %w", entry.Name(), err)
		}
		if err := c.AddResource(entry.Name(), doc); err != nil {
			return fmt.Errorf("failed to register embedded schema %s: %w", entry.Name(), err)
		}
	}

	sch, err := c.Compile(schemaName)
	if err != nil {
		return err
	}

	return validateFile(path, sch)
}

func validateFile(path string, sch *jsonschema.Schema) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".yaml" || ext == ".yml" {
		// Validate every document in a multi-doc YAML stream individually so
		// that profile files using --- separators are fully validated.
		dec := yaml.NewDecoder(f)
		count := 0
		for {
			var raw any
			if err := dec.Decode(&raw); err != nil {
				if err == io.EOF {
					break
				}
				return err
			}
			count++
			jsonBytes, err := json.Marshal(raw)
			if err != nil {
				return err
			}
			doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(jsonBytes))
			if err != nil {
				return err
			}
			if err := sch.Validate(doc); err != nil {
				return fmt.Errorf("invalid format: %w", err)
			}
		}
		if count == 0 {
			return fmt.Errorf("invalid format: file contains no YAML documents")
		}
		return nil
	}

	data, err := io.ReadAll(f)
	if err != nil {
		return err
	}

	// Detect a top-level JSON array and validate each element individually,
	// mirroring how extractProfilesFromJSON supports both single-object and
	// array-of-objects JSON profile files.
	var arr []any
	if json.Unmarshal(data, &arr) == nil {
		for _, elem := range arr {
			jsonBytes, err := json.Marshal(elem)
			if err != nil {
				return err
			}
			doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(jsonBytes))
			if err != nil {
				return err
			}
			if err := sch.Validate(doc); err != nil {
				return fmt.Errorf("invalid format: %w", err)
			}
		}
		return nil
	}

	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return err
	}
	if err := sch.Validate(doc); err != nil {
		return fmt.Errorf("invalid format: %w", err)
	}
	return nil
}
