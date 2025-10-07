// The lib package is the implementation of the core functionality of the raid CLI tool.
package lib

import (
	"fmt"
	"sync"
)

const (
	YAML_SEP = "---"
)

type Context struct {
	Profile Profile
	Env     string
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

	return nil
}
