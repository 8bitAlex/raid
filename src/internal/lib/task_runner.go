package lib

// import (
// 	"fmt"
// 	"os"
// 	"os/exec"
// 	"path/filepath"
// 	"sync"
// )

// // TaskRunner handles the execution of tasks
// type TaskRunner struct {
// 	concurrency int
// }

// // NewTaskRunner creates a new task runner with the specified concurrency limit
// func NewTaskRunner(concurrency int) *TaskRunner {
// 	return &TaskRunner{
// 		concurrency: concurrency,
// 	}
// }

// // ExecuteTasks executes a list of tasks concurrently
// func (tr *TaskRunner) ExecuteTasks(tasks []Task) error {
// 	if len(tasks) == 0 {
// 		return nil
// 	}

// 	// Use a semaphore to limit concurrency
// 	var semaphore chan struct{}
// 	if tr.concurrency > 0 {
// 		semaphore = make(chan struct{}, tr.concurrency)
// 	}

// 	var wg sync.WaitGroup
// 	errorChan := make(chan error, len(tasks))
// 	var outputMutex sync.Mutex

// 	for i, task := range tasks {
// 		wg.Add(1)
// 		go func(taskIndex int, task Task) {
// 			defer wg.Done()

// 			// Acquire semaphore if concurrency is limited
// 			if semaphore != nil {
// 				semaphore <- struct{}{}
// 				defer func() { <-semaphore }()
// 			}

// 			// Lock output to prevent interleaved messages
// 			outputMutex.Lock()
// 			fmt.Printf("Executing task %d: %s\n", taskIndex+1, task.Type)
// 			outputMutex.Unlock()

// 			if err := tr.executeTask(task); err != nil {
// 				errorChan <- fmt.Errorf("task %d failed: %w", taskIndex+1, err)
// 			} else {
// 				outputMutex.Lock()
// 				fmt.Printf("Task %d completed successfully\n", taskIndex+1)
// 				outputMutex.Unlock()
// 			}
// 		}(i, task)
// 	}

// 	wg.Wait()
// 	close(errorChan)

// 	// Check for any errors
// 	var errors []error
// 	for err := range errorChan {
// 		errors = append(errors, err)
// 	}

// 	if len(errors) > 0 {
// 		return fmt.Errorf("some tasks failed: %v", errors)
// 	}

// 	return nil
// }

// // executeTask executes a single task
// func (tr *TaskRunner) executeTask(task Task) error {
// 	switch task.Type {
// 	case "Shell":
// 		return tr.executeShellTask(task)
// 	case "Script":
// 		return tr.executeScriptTask(task)
// 	default:
// 		return fmt.Errorf("unknown task type: %s", task.Type)
// 	}
// }

// // executeShellTask executes a shell command
// func (tr *TaskRunner) executeShellTask(task Task) error {
// 	if task.Cmd == "" {
// 		return fmt.Errorf("shell task requires 'cmd' field")
// 	}

// 	cmd := exec.Command("sh", "-c", task.Cmd)
// 	cmd.Stdout = os.Stdout
// 	cmd.Stderr = os.Stderr
// 	cmd.Stdin = os.Stdin

// 	return cmd.Run()
// }

// // executeScriptTask executes a script file
// func (tr *TaskRunner) executeScriptTask(task Task) error {
// 	if task.Path == "" {
// 		return fmt.Errorf("script task requires 'path' field")
// 	}

// 	// Resolve the script path
// 	scriptPath, err := filepath.Abs(task.Path)
// 	if err != nil {
// 		return fmt.Errorf("failed to resolve script path: %w", err)
// 	}

// 	// Check if the script file exists
// 	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
// 		return fmt.Errorf("script file not found: %s", scriptPath)
// 	}

// 	// Make the script executable
// 	if err := os.Chmod(scriptPath, 0755); err != nil {
// 		return fmt.Errorf("failed to make script executable: %w", err)
// 	}

// 	// Execute the script
// 	cmd := exec.Command(scriptPath)
// 	cmd.Stdout = os.Stdout
// 	cmd.Stderr = os.Stderr
// 	cmd.Stdin = os.Stdin

// 	return cmd.Run()
// }
