// The lib package is the implementation of the core functionality of the raid CLI tool.
package lib

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	sys "github.com/8bitalex/raid/src/internal/sys"
	"github.com/8bitalex/raid/src/internal/utils"
	"github.com/santhosh-tekuri/jsonschema/v6"
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

	for i := range profile.Repositories {
		if err := buildRepo(&profile.Repositories[i]); err != nil {
			return err
		}
	}

	context = &Context{
		Profile: profile,
		Env:     GetEnv(),
	}
	return nil
}

// Install clones all repositories in the active profile and runs install tasks.
func Install(maxThreads int) error {
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

	if err := ExecuteTasks(profile.Install.Tasks); err != nil {
		return fmt.Errorf("failed to execute install tasks: %w", err)
	}

	var repoTasks []Task
	for _, r := range profile.Repositories {
		repoTasks = append(repoTasks, r.Install.Tasks...)
	}
	if err := ExecuteTasks(repoTasks); err != nil {
		return fmt.Errorf("failed to execute repository install tasks: %w", err)
	}

	return nil
}

// ValidateSchema validates the file at path against the JSON schema at schemaPath.
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

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	var reader io.Reader = f
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".yaml" || ext == ".yml" {
		data, err := utils.YAMLToJSON(f)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}

	doc, err := jsonschema.UnmarshalJSON(reader)
	if err != nil {
		return err
	}

	if err := sch.Validate(doc); err != nil {
		return fmt.Errorf("invalid format: %w", err)
	}
	return nil
}
