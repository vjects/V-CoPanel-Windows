package shims

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"vcopanel-bridge/internal/logger"
)

// GenerateShims writes .tools-version and creates local wrappers (php.bat, npm.cmd, node.cmd, composer.bat, go.bat) in project root.
func GenerateShims(projectPath, stack, phpVersion, nodeVersion, goVersion, workspaceDir, assetsDir string) error {
	// Step 1: Write .tools-version with comments and managed block
	SyncToolsVersionBlock(filepath.Join(projectPath, ".tools-version"), stack, phpVersion, nodeVersion, goVersion)

	var phpDir string
	if phpVersion != "" && phpVersion != "None" {
		phpDir = findBinaryDir(filepath.Join(workspaceDir, "runtimes", phpVersion), "php.exe")
		if phpDir == "" {
			phpDir = filepath.Join(workspaceDir, "runtimes", phpVersion)
		}
	}

	var nodeDir string
	if nodeVersion != "" && nodeVersion != "None" {
		nodeDir = findBinaryDir(filepath.Join(workspaceDir, "runtimes", nodeVersion), "node.exe")
		if nodeDir == "" {
			nodeDir = filepath.Join(workspaceDir, "runtimes", nodeVersion)
		}
	}

	var goBinDir string
	if goVersion != "" && goVersion != "None" {
		goBinDir = findBinaryDir(filepath.Join(workspaceDir, "runtimes", goVersion), "go.exe")
		if goBinDir == "" {
			goBinDir = filepath.Join(workspaceDir, "runtimes", goVersion, "bin")
		}
	}



	composerPhar := filepath.Join(workspaceDir, "shared-services", "composer", "composer.phar")
	if _, err := os.Stat(composerPhar); os.IsNotExist(err) {
		composerPhar = filepath.Join(workspaceDir, "runtimes", "composer", "composer.phar")
		if _, err := os.Stat(composerPhar); os.IsNotExist(err) {
			composerPhar = filepath.Join(assetsDir, "composer", "composer.phar")
			if _, err := os.Stat(composerPhar); os.IsNotExist(err) {
				composerPhar = filepath.Join(assetsDir, "composer.phar")
			}
		}
	}



	if phpDir != "" {
		phpRel := rel(projectPath, phpDir)
		nodeRel := ""
		if nodeDir != "" {
			nodeRel = rel(projectPath, nodeDir)
		}
		
		phpBatContent := fmt.Sprintf(`@echo off
set PATH=%s;%s;%%PATH%%
"%s\php.exe" %%*
`, phpRel, nodeRel, phpRel)
		os.WriteFile(filepath.Join(projectPath, "php.bat"), []byte(phpBatContent), 0755)

		composerPharRel := rel(projectPath, composerPhar)
		composerBatContent := fmt.Sprintf(`@echo off
set PATH=%s;%%PATH%%
"%s\php.exe" "%s" %%*
`, phpRel, phpRel, composerPharRel)
		os.WriteFile(filepath.Join(projectPath, "composer.bat"), []byte(composerBatContent), 0755)

		queueBatContent := `@echo off
chcp 65001 >nul 2>&1
echo [+] Starting Laravel Queue Listener (Intermediary Script)...
"%~dp0php.bat" artisan queue:listen --tries=3 --timeout=90 --sleep=1
`
		os.WriteFile(filepath.Join(projectPath, "queue.bat"), []byte(queueBatContent), 0755)

		scheduleBatContent := `@echo off
chcp 65001 >nul 2>&1
echo [+] Starting Laravel Schedule Worker (Intermediary Script)...
"%~dp0php.bat" artisan schedule:work
`
		os.WriteFile(filepath.Join(projectPath, "schedule.bat"), []byte(scheduleBatContent), 0755)
	} else {
		os.Remove(filepath.Join(projectPath, "php.bat"))
		os.Remove(filepath.Join(projectPath, "composer.bat"))
		os.Remove(filepath.Join(projectPath, "queue.bat"))
		os.Remove(filepath.Join(projectPath, "schedule.bat"))
	}

	if nodeDir != "" {
		nodeRel := rel(projectPath, nodeDir)
		npmCmdContent := fmt.Sprintf(`@echo off
set PATH=%s;%%PATH%%
"%s\npm.cmd" %%*
`, nodeRel, nodeRel)
		os.WriteFile(filepath.Join(projectPath, "npm.cmd"), []byte(npmCmdContent), 0755)
		os.WriteFile(filepath.Join(projectPath, "npm.bat"), []byte(npmCmdContent), 0755)

		nodeCmdContent := fmt.Sprintf(`@echo off
set PATH=%s;%%PATH%%
"%s\node.exe" %%*
`, nodeRel, nodeRel)
		os.WriteFile(filepath.Join(projectPath, "node.cmd"), []byte(nodeCmdContent), 0755)
		os.WriteFile(filepath.Join(projectPath, "node.bat"), []byte(nodeCmdContent), 0755)
	} else {
		os.Remove(filepath.Join(projectPath, "npm.cmd"))
		os.Remove(filepath.Join(projectPath, "npm.bat"))
		os.Remove(filepath.Join(projectPath, "node.cmd"))
		os.Remove(filepath.Join(projectPath, "node.bat"))
	}

	if goBinDir != "" {
		goRoot := filepath.Dir(goBinDir)
		goRootRel := rel(projectPath, goRoot)
		goBinRel := rel(projectPath, goBinDir)
		goBatContent := fmt.Sprintf(`@echo off
set GOROOT=%s
set PATH=%s;%%PATH%%
"%s\go.exe" %%*
`, goRootRel, goBinRel, goBinRel)
		os.WriteFile(filepath.Join(projectPath, "go.bat"), []byte(goBatContent), 0755)

		runBatContent := `@echo off
chcp 65001 >nul 2>&1
echo [+] Running Go application via V-CoPanel Shim...
"%~dp0go.bat" run . %%*
`
		os.WriteFile(filepath.Join(projectPath, "run.bat"), []byte(runBatContent), 0755)
	} else {
		os.Remove(filepath.Join(projectPath, "go.bat"))
		os.Remove(filepath.Join(projectPath, "run.bat"))
	}

	logger.Log("Generated local shims in: %s", projectPath)
	return nil
}

// OpenTerminal opens a new Windows Terminal (wt) or falls back to CMD in the project directory.
func OpenTerminal(projectPath string) error {
	// Try Windows Terminal first (wt.exe) — available on Win10+
	wtPath, err := exec.LookPath("wt.exe")
	if err == nil {
		cmd := exec.Command(wtPath, "--title", "V-CoPanel Terminal", "--startingDirectory", projectPath)
		cmd.Dir = projectPath
		return cmd.Start()
	}
	// Fallback: standard cmd.exe
	// NOTE: the first quoted arg after "start" is the window title.
	// It MUST be wrapped in quotes (passed as a separate arg) so cmd doesn't
	// mistake it for an executable name.
	cmd := exec.Command("cmd.exe", "/c", "start", "\"V-CoPanel\"", "cmd.exe", "/k", "cd /d \""+projectPath+"\"")
	return cmd.Run()
}


func findBinaryDir(root, binaryName string) string {
	var foundDir string
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && info.Name() == binaryName {
			foundDir = filepath.Dir(path)
			return filepath.SkipAll
		}
		return nil
	})
	return foundDir
}

func rel(base, target string) string {
	r, err := filepath.Rel(base, target)
	if err != nil {
		return target
	}
	return "%~dp0" + r
}

// SyncToolsVersionBlock safely updates the managed block in .tools-version
func SyncToolsVersionBlock(tvPath, stack, phpVer, nodeVer, goVer string) error {
	const blockStart = "# --- V-COPANEL MANAGED RUNTIME BLOCK ---"
	const blockEnd = "# --- END V-COPANEL MANAGED RUNTIME BLOCK ---"

	data, err := os.ReadFile(tvPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	lines := strings.Split(string(data), "\n")
	var newLines []string
	inBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == blockStart {
			inBlock = true
			continue
		}
		if trimmed == blockEnd {
			inBlock = false
			continue
		}
		if !inBlock {
			newLines = append(newLines, line)
		}
	}

	// Remove trailing empty lines
	for len(newLines) > 0 && strings.TrimSpace(newLines[len(newLines)-1]) == "" {
		newLines = newLines[:len(newLines)-1]
	}

	if len(newLines) > 0 {
		newLines = append(newLines, "")
	}

	newLines = append(newLines, blockStart)
	newLines = append(newLines, "# Do not edit this section manually unless from V-CoPanel UI.")
	
	if stack != "" && stack != "other" {
		newLines = append(newLines, fmt.Sprintf("stack: %s", stack))
	}
	
	if phpVer != "" && phpVer != "None" {
		newLines = append(newLines, fmt.Sprintf("php: %s", phpVer))
	}
	if nodeVer != "" && nodeVer != "None" {
		newLines = append(newLines, fmt.Sprintf("node: %s", nodeVer))
	}
	if goVer != "" && goVer != "None" {
		newLines = append(newLines, fmt.Sprintf("go: %s", goVer))
	}
	
	newLines = append(newLines, blockEnd)
	newLines = append(newLines, "") // Final newline

	return os.WriteFile(tvPath, []byte(strings.Join(newLines, "\n")), 0644)
}

// EjectToolsVersionBlock safely removes the managed block from .tools-version
func EjectToolsVersionBlock(tvPath string) error {
	const blockStart = "# --- V-COPANEL MANAGED RUNTIME BLOCK ---"
	const blockEnd = "# --- END V-COPANEL MANAGED RUNTIME BLOCK ---"

	data, err := os.ReadFile(tvPath)
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
		trimmed := strings.TrimSpace(line)
		if trimmed == blockStart {
			inBlock = true
			continue
		}
		if trimmed == blockEnd {
			inBlock = false
			continue
		}
		if !inBlock {
			newLines = append(newLines, line)
		}
	}

	// Remove trailing empty lines
	for len(newLines) > 0 && strings.TrimSpace(newLines[len(newLines)-1]) == "" {
		newLines = newLines[:len(newLines)-1]
	}

	// If the file only contained our block, we can just delete it
	if len(newLines) == 0 {
		return os.Remove(tvPath)
	}

	newLines = append(newLines, "") // Final newline
	return os.WriteFile(tvPath, []byte(strings.Join(newLines, "\n")), 0644)
}

