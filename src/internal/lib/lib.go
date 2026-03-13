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
	"github.com/thoas/go-funk"
)

const (
	YAML_SEP           = "---"
	RaidConfigFileName = "raid.yaml"
)

type Context struct {
	Profile Profile
	Env     string
	// Options Options
}

type OnInstall struct {
	Tasks []Task `json:"tasks"`
}

var context *Context

func Load() error {
	if context == nil {
		return ForceLoad()
	}
	return nil
}

func ForceLoad() error {
	profile, err := buildProfile(GetProfile())
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

func Install(maxThreads int) error {

	// todo migrate to task runner
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

	pts := profile.Install.Tasks
	if err := ExecuteTasks(pts); err != nil {
		return fmt.Errorf("failed to execute install tasks: %w", err)
	}

	rts := funk.FlatMap(profile.Repositories, func(r Repo) []Task {
		return r.Install.Tasks
	}).([]Task)
	if err := ExecuteTasks(rts); err != nil {
		return fmt.Errorf("failed to execute repository install tasks: %w", err)
	}

	return nil
}

func ValidateSchema(path string, schemaPath string) error {
	path = sys.ExpandPath(path)
	schemaPath = sys.ExpandPath(schemaPath)

	if !sys.FileExists(path) || path == "" {
		return fmt.Errorf("file not found at %s", path)
	}
	if !sys.FileExists(schemaPath) || schemaPath == "" {
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

	contents := io.Reader(f)

	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".yaml" || ext == ".yml" {
		data, err := utils.YAMLToJSON(f)
		if err != nil {
			return err
		}
		contents = bytes.NewReader(data)
	}

	json, err := jsonschema.UnmarshalJSON(contents)
	if err != nil {
		return err
	}

	err = sch.Validate(json)
	if err != nil {
		return fmt.Errorf("invalid format: %w", err)
	}
	return nil
}
