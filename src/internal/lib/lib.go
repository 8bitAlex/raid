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
}

var context *Context

func Compile() error {
	if context == nil {
		return ForceCompile()
	}
	return nil
}

func ForceCompile() error {
	profile, err := BuildProfile(GetProfile())
	if err != nil {
		return err
	}
	context = &Context{
		Profile: profile,
	}
	return nil
}

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

	return nil
}
