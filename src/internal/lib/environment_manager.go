package lib

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnvironmentManager handles environment execution
type EnvironmentManager struct {
	taskRunner *TaskRunner
}

// NewEnvironmentManager creates a new environment manager
func NewEnvironmentManager(concurrency int) *EnvironmentManager {
	return &EnvironmentManager{
		taskRunner: NewTaskRunner(concurrency),
	}
}

// ExecuteEnvironment executes an environment by name
func (em *EnvironmentManager) ExecuteEnvironment(envName string) error {
	// Get the active profile
	profile, err := GetActiveProfileContent()
	if err != nil {
		return fmt.Errorf("failed to get active profile: %w", err)
	}

	fmt.Printf("Executing environment '%s' for profile '%s'\n", envName, profile.Name)

	// Find and execute environment from profile first
	profileEnv, found := em.findEnvironmentInProfile(profile, envName)
	if found {
		fmt.Println("Found environment in profile, executing...")
		if err := em.executeEnvironment(profileEnv); err != nil {
			return fmt.Errorf("failed to execute profile environment: %w", err)
		}
	}

	// Find and execute environments from repositories
	for _, repo := range profile.Repositories {
		fmt.Printf("Checking repository '%s' for environment '%s'\n", repo.Name, envName)

		repoEnv, found := em.findEnvironmentInRepository(repo, envName)
		if found {
			fmt.Printf("Found environment in repository '%s', executing...\n", repo.Name)
			if err := em.executeEnvironment(repoEnv); err != nil {
				return fmt.Errorf("failed to execute repository environment '%s': %w", repo.Name, err)
			}
		}
	}

	return nil
}

// findEnvironmentInProfile finds an environment in the profile
func (em *EnvironmentManager) findEnvironmentInProfile(profile *ProfileContent, envName string) (*Environment, bool) {
	for _, env := range profile.Environments {
		if strings.EqualFold(env.Name, envName) {
			return &env, true
		}
	}
	return nil, false
}

// findEnvironmentInRepository finds an environment in a repository
func (em *EnvironmentManager) findEnvironmentInRepository(repo Repository, envName string) (*Environment, bool) {
	// Read the repository configuration file
	repoConfigPath := filepath.Join(repo.Path, "raid.yaml")
	if _, err := os.Stat(repoConfigPath); os.IsNotExist(err) {
		// Try raid.yml
		repoConfigPath = filepath.Join(repo.Path, "raid.yml")
		if _, err := os.Stat(repoConfigPath); os.IsNotExist(err) {
			// Try raid.json
			repoConfigPath = filepath.Join(repo.Path, "raid.json")
			if _, err := os.Stat(repoConfigPath); os.IsNotExist(err) {
				return nil, false
			}
		}
	}

	// Read and parse the repository configuration
	repoProfile, err := ReadProfileFile(repoConfigPath)
	if err != nil {
		fmt.Printf("Warning: failed to read repository config %s: %v\n", repoConfigPath, err)
		return nil, false
	}

	// Look for the environment in the repository
	for _, env := range repoProfile.Environments {
		if strings.EqualFold(env.Name, envName) {
			return &env, true
		}
	}

	return nil, false
}

// executeEnvironment executes an environment
func (em *EnvironmentManager) executeEnvironment(env *Environment) error {
	fmt.Printf("Executing environment '%s'\n", env.Name)

	// Set environment variables first
	if err := em.setEnvironmentVariables(env.Variables); err != nil {
		return fmt.Errorf("failed to set environment variables: %w", err)
	}

	// Execute tasks
	if err := em.taskRunner.ExecuteTasks(env.Tasks); err != nil {
		return fmt.Errorf("failed to execute tasks: %w", err)
	}

	return nil
}

// setEnvironmentVariables sets the environment variables globally
func (em *EnvironmentManager) setEnvironmentVariables(vars []EnvironmentVariable) error {
	for _, v := range vars {
		fmt.Printf("Setting environment variable: %s=%s\n", v.Name, v.Value)
		if err := os.Setenv(v.Name, v.Value); err != nil {
			return fmt.Errorf("failed to set environment variable %s: %w", v.Name, err)
		}
	}
	return nil
}
