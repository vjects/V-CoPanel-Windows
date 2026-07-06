package envmanager

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
	"vcopanel-bridge/internal/database"
)

const (
	BlockStart = "# --- V-COPANEL SANDBOX (DO NOT MODIFY SYSTEM VARIABLES BELOW) ---"
	BlockEnd   = "# --- END V-COPANEL SANDBOX ---"
)

var (
	watchers   = make(map[string]chan struct{})
	watchersMu sync.Mutex
)

// SyncSandboxBlock updates or creates the V-CoPanel sandbox block in the given .env file
// without modifying any user-defined variables outside the block, except commenting them out if they overlap.
func SyncSandboxBlock(envPath string, vars map[string]string) error {
	data, err := os.ReadFile(envPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	lines := strings.Split(string(data), "\n")
	var newLines []string
	inBlock := false

	// Create a map of variables we are about to inject to easily check for overlaps
	injectKeys := make(map[string]bool)
	for k := range vars {
		injectKeys[k] = true
	}

	for _, line := range lines {
		// Clean up carriage returns
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(line)

		if trimmed == BlockStart {
			inBlock = true
			continue
		}
		if trimmed == BlockEnd {
			inBlock = false
			continue
		}
		
		if !inBlock {
			// Check if this line defines a variable we are about to inject
			if !strings.HasPrefix(trimmed, "#") && strings.Contains(trimmed, "=") {
				parts := strings.SplitN(trimmed, "=", 2)
				key := strings.TrimSpace(parts[0])
				if injectKeys[key] {
					// We need to back this up by commenting it out with our specific prefix
					newLines = append(newLines, fmt.Sprintf("#VCO_BACKUP:%s", line))
					continue
				}
			}
			newLines = append(newLines, line)
		}
	}

	// Remove trailing empty lines
	for len(newLines) > 0 && strings.TrimSpace(newLines[len(newLines)-1]) == "" {
		newLines = newLines[:len(newLines)-1]
	}

	// Append the new block at the end
	if len(newLines) > 0 {
		newLines = append(newLines, "")
	}
	newLines = append(newLines, BlockStart)
	newLines = append(newLines, "# These variables are automatically managed by the system.")
	newLines = append(newLines, "# Any manual changes inside this block will be overwritten.")
	
	for k, v := range vars {
		newLines = append(newLines, fmt.Sprintf("%s=\"%s\"", k, v))
	}
	
	newLines = append(newLines, BlockEnd)
	newLines = append(newLines, "") // Final newline

	output := strings.Join(newLines, "\n")
	return os.WriteFile(envPath, []byte(output), 0644)
}

// EjectSandboxBlock completely removes the sandbox block from the .env file
// and restores any variables that were backed up during SyncSandboxBlock.
func EjectSandboxBlock(envPath string) error {
	data, err := os.ReadFile(envPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	lines := strings.Split(string(data), "\n")
	var newLines []string
	inBlock := false

	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(line)

		if trimmed == BlockStart {
			inBlock = true
			continue
		}
		if trimmed == BlockEnd {
			inBlock = false
			continue
		}

		if !inBlock {
			// Restore backed up variables
			if strings.HasPrefix(trimmed, "#VCO_BACKUP:") {
				restored := strings.TrimPrefix(trimmed, "#VCO_BACKUP:")
				newLines = append(newLines, restored)
			} else {
				newLines = append(newLines, line)
			}
		}
	}

	// Clean trailing newlines
	for len(newLines) > 0 && strings.TrimSpace(newLines[len(newLines)-1]) == "" {
		newLines = newLines[:len(newLines)-1]
	}
	newLines = append(newLines, "")

	output := strings.Join(newLines, "\n")
	return os.WriteFile(envPath, []byte(output), 0644)
}

// ExtractSandboxBlock reads the current variables from the V-CoPanel sandbox block.
func ExtractSandboxBlock(envPath string) map[string]string {
	res := make(map[string]string)
	data, err := os.ReadFile(envPath)
	if err != nil {
		return res
	}

	inBlock := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == BlockStart {
			inBlock = true
			continue
		}
		if trimmed == BlockEnd {
			inBlock = false
			continue
		}
		if inBlock {
			if strings.HasPrefix(trimmed, "#") || trimmed == "" {
				continue
			}
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(strings.Trim(parts[1], "\"'"))
				res[key] = val
			}
		}
	}
	return res
}

// StartWatcher starts a polling file watcher that detects manual edits to the .env file.
// If changes to the sandbox block are detected, it updates the database for two-way sync.
func StartWatcher(projectUUID, envPath string, mariaRoot, dbPort string) {
	watchersMu.Lock()
	if _, exists := watchers[projectUUID]; exists {
		watchersMu.Unlock()
		return
	}
	stopChan := make(chan struct{})
	watchers[projectUUID] = stopChan
	watchersMu.Unlock()

	go func() {
		var lastModTime time.Time
		for {
			select {
			case <-stopChan:
				return
			case <-time.After(2 * time.Second):
				info, err := os.Stat(envPath)
				if err != nil {
					continue
				}
				if lastModTime.IsZero() {
					lastModTime = info.ModTime()
					continue
				}
				if info.ModTime().After(lastModTime) {
					lastModTime = info.ModTime()
					// File changed manually! Extract sandbox variables and sync to DB.
					currentVars := ExtractSandboxBlock(envPath)
					
					// Insert/Update into DB (project_env_variables table)
					// This implements the TWO-WAY SYNC from .env to Database.
					for k, v := range currentVars {
						q := fmt.Sprintf("REPLACE INTO `project_env_variables` (`project_uuid`, `env_key`, `env_value`) VALUES ('%s', '%s', '%s');", projectUUID, k, v)
						database.ExecuteDynamicQuery(mariaRoot, dbPort, q)
					}
				}
			}
		}
	}()
}

// StopWatcher stops the file watcher for the specified project.
func StopWatcher(projectUUID string) {
	watchersMu.Lock()
	defer watchersMu.Unlock()
	if ch, exists := watchers[projectUUID]; exists {
		close(ch)
		delete(watchers, projectUUID)
	}
}
