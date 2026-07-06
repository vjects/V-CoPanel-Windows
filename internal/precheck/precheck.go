package precheck

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"vcopanel-bridge/internal/database"
	"vcopanel-bridge/internal/extractor"
	"vcopanel-bridge/internal/goconfig"
	"vcopanel-bridge/internal/logger"
	"vcopanel-bridge/internal/nodeconfig"
	"vcopanel-bridge/internal/notifier"
	"vcopanel-bridge/internal/phpini"
)

type Engine struct {
	mu            sync.Mutex
	WorkspaceDir  string
	IsConfigured  bool
	IsRunning     bool
	Stage1Done    bool
	Stage2Done    bool
	ProgressLogs  []string
	OnComplete    func() // Optional callback fired after provisioning completes
}

var Instance = &Engine{
	ProgressLogs: make([]string, 0),
}

func (e *Engine) SetWorkspace(path string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		return fmt.Errorf("invalid projects workspace directory: %s", path)
	}

	e.WorkspaceDir = path
	e.IsConfigured = true
	return nil
}

func (e *Engine) AddLog(msg string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	logger.Log("%s", msg)
	e.ProgressLogs = append(e.ProgressLogs, msg)
}

func (e *Engine) GetLogs() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	logsCopy := make([]string, len(e.ProgressLogs))
	copy(logsCopy, e.ProgressLogs)
	return logsCopy
}

func (e *Engine) Status() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.IsRunning
}

func (e *Engine) IsStage1Done() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.Stage1Done
}

func (e *Engine) IsStage2Done() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.Stage2Done
}

// copyFile copies a single file
func copyFile(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()
	os.MkdirAll(filepath.Dir(dst), 0755)
	d, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer d.Close()
	_, err = io.Copy(d, s)
	return err
}

// RunStage1 extracts Core infrastructure and Shared Services before UI/server launch.
func (e *Engine) RunStage1(assetsDir string, coreWorkspace string) {
	e.mu.Lock()
	if e.IsRunning {
		e.mu.Unlock()
		return
	}
	e.IsRunning = true
	e.ProgressLogs = []string{}
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		e.IsRunning = false
		e.Stage1Done = true
		e.mu.Unlock()
	}()

	notifier.Info("Precheck Stage 1 Started", "Unpacking core infrastructure and shared services...")
	e.AddLog("Starting Pre-check Stage 1 (Core & Shared Services)...")

	// Migrate core-assets to core if it exists
	oldCoreAssets := filepath.Join(coreWorkspace, "core-assets")
	coreDir := filepath.Join(coreWorkspace, "core")
	if info, err := os.Stat(oldCoreAssets); err == nil && info.IsDir() {
		os.MkdirAll(coreDir, 0755)
		if entries, errRead := os.ReadDir(oldCoreAssets); errRead == nil {
			for _, entry := range entries {
				oldPath := filepath.Join(oldCoreAssets, entry.Name())
				newPath := filepath.Join(coreDir, entry.Name())
				os.Rename(oldPath, newPath)
			}
		}
		os.RemoveAll(oldCoreAssets)
	}
	os.MkdirAll(coreDir, 0755)

	processAssetStage1 := func(assetPath, name, category string) {
		lower := strings.ToLower(name)
		if !strings.HasSuffix(lower, ".zip") {
			return
		}

		if (strings.HasPrefix(lower, "go") || category == "go") && !strings.Contains(lower, "1.26.4") {
			goTarget := filepath.Join(coreWorkspace, "core")
			goExe := filepath.Join(goTarget, "go", "bin", "go.exe")
			if info, err := os.Stat(goExe); err != nil || info.IsDir() {
				logger.Log("Unpacking %s -> workspace/core/go", name)
			}
			skipped, err := extractor.EnsureGoAsset(assetPath, goTarget)
			if err != nil {
				e.AddLog(fmt.Sprintf("[Error] Failed extracting Go runtime into core/go: %v", err))
			} else if skipped {
				e.AddLog(fmt.Sprintf("[Skip] Asset %s is already verified in core/go", name))
			} else {
				e.AddLog(fmt.Sprintf("[Success] Extracted asset %s into core/go", name))
			}
			return
		}

		var targetDir string
		if category == "phpmyadmin" || strings.HasPrefix(lower, "phpmyadmin") {
			targetDir = filepath.Join(coreWorkspace, "shared-services", "phpmyadmin")
		} else if category == "mailpit" || strings.HasPrefix(lower, "mailpit") {
			targetDir = filepath.Join(coreWorkspace, "shared-services", "mailpit")
		} else if category == "redis" || strings.HasPrefix(lower, "redis") {
			targetDir = filepath.Join(coreWorkspace, "shared-services", "redis")
		} else if category == "mariadb" || strings.HasPrefix(lower, "mariadb") {
			targetDir = filepath.Join(coreWorkspace, "shared-services", "mariadb")
		} else if (category == "php" || strings.HasPrefix(lower, "php-8.3") || strings.HasPrefix(lower, "php-8.4") || strings.HasPrefix(lower, "php-8.5") || strings.HasPrefix(lower, "php-8.2")) && !func() bool { _, err := os.Stat(filepath.Join(coreWorkspace, "core", "php", "php.exe")); return err == nil }() {
			targetDir = filepath.Join(coreWorkspace, "core", "php")
		} else {
			return // Not a Stage 1 asset
		}

		if info, err := os.Stat(targetDir); err != nil || !info.IsDir() {
			relPath := filepath.Base(targetDir)
			if filepath.Base(filepath.Dir(targetDir)) == "core" {
				relPath = "core/" + relPath
			} else {
				relPath = "shared-services/" + relPath
			}
			logger.Log("Unpacking Stage 1 asset %s -> workspace/%s", name, relPath)
		}
		skipped, err := extractor.EnsureAsset(assetPath, targetDir)
		if err != nil {
			e.AddLog(fmt.Sprintf("[Error] Failed extracting %s: %v", name, err))
		} else if skipped {
			e.AddLog(fmt.Sprintf("[Skip] Asset %s is already verified in %s", name, filepath.Base(targetDir)))
		} else {
			e.AddLog(fmt.Sprintf("[Success] Extracted asset %s into %s successfully", name, filepath.Base(targetDir)))
		}
	}

	if entries, err := os.ReadDir(assetsDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				category := entry.Name()
				if category == "Wallpapers" || category == "Stack-Logo" {
					continue
				}
				catDir := filepath.Join(assetsDir, category)
				if subFiles, errSub := os.ReadDir(catDir); errSub == nil {
					for _, sf := range subFiles {
						if !sf.IsDir() && strings.HasSuffix(strings.ToLower(sf.Name()), ".zip") {
							processAssetStage1(filepath.Join(catDir, sf.Name()), sf.Name(), category)
						}
					}
				}
			} else if strings.HasSuffix(strings.ToLower(entry.Name()), ".zip") {
				processAssetStage1(filepath.Join(assetsDir, entry.Name()), entry.Name(), "")
			}
		}
	}

	// Create or update Desktop Shortcut
	launcherPath := filepath.Join(coreWorkspace, "..", "internal", "desktop_launcher.bat")
	if absLauncher, errAbs := filepath.Abs(launcherPath); errAbs == nil {
		workDir := filepath.Dir(filepath.Dir(absLauncher))
		if errSc := EnsureDesktopShortcut(absLauncher, workDir); errSc != nil {
			e.AddLog(fmt.Sprintf("[Warning] Could not create desktop shortcut: %v", errSc))
		} else {
			e.AddLog("[Success] Verified and created Desktop Shortcut (V-CoPanel Windows.lnk)")
		}
	}

	e.AddLog("Precheck Stage 1 (Core & Shared Services) completed successfully!")
	notifier.Success("Precheck Stage 1 Complete", "Core infrastructure and shared services verified!")
}

// RunStage2 extracts project runtime infrastructure in the background after UI is live.
func (e *Engine) RunStage2(assetsDir string, coreWorkspace string) {
	e.mu.Lock()
	if e.IsRunning {
		e.mu.Unlock()
		return
	}
	e.IsRunning = true
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		e.IsRunning = false
		e.Stage2Done = true
		e.mu.Unlock()
	}()

	notifier.Info("Precheck Stage 2 Started", "Unpacking project runtime infrastructure in background...")
	e.AddLog("Starting Pre-check Stage 2 (Project Runtime Infrastructure)...")

	processAssetStage2 := func(assetPath, name, category string) {
		lower := strings.ToLower(name)

		if strings.HasSuffix(lower, ".phar") || category == "composer" || strings.HasPrefix(lower, "composer") {
			composerDir := filepath.Join(coreWorkspace, "runtimes", "composer")
			targetPhar := filepath.Join(composerDir, "composer.phar")
			if _, errStat := os.Stat(targetPhar); errStat == nil {
				e.AddLog("[Skip] Asset composer (composer.phar) is already verified in runtimes/composer")
				return
			}
			if err := copyFile(assetPath, targetPhar); err == nil {
				batContent := `@echo off
"%~dp0..\php-8.3\php.exe" "%~dp0composer.phar" %*
`
				os.WriteFile(filepath.Join(composerDir, "composer.bat"), []byte(batContent), 0644)
				e.AddLog("[Success] Provisioned asset composer.phar into workspace/runtimes/composer")
			} else {
				e.AddLog(fmt.Sprintf("[Error] Failed copying composer.phar: %v", err))
			}
			return
		}

		if !strings.HasSuffix(lower, ".zip") {
			return
		}

		if (strings.HasPrefix(lower, "go") || category == "go") && strings.Contains(lower, "1.26.4") {
			subDir := strings.TrimSuffix(name, ".zip")
			subDir = strings.TrimSuffix(subDir, "-win64")
			goRuntimeTarget := filepath.Join(coreWorkspace, "runtimes", subDir)
			goRtExe := filepath.Join(goRuntimeTarget, "bin", "go.exe")
			if info, err := os.Stat(goRtExe); err != nil || info.IsDir() {
				logger.Log("Unpacking %s -> workspace/runtimes/%s", name, subDir)
			}
			skippedRt, errRt := extractor.EnsureGoRuntimeAsset(assetPath, goRuntimeTarget)
			if errRt != nil {
				e.AddLog(fmt.Sprintf("[Error] Failed extracting isolated Go runtime into runtimes/%s: %v", subDir, errRt))
			} else if skippedRt {
				e.AddLog(fmt.Sprintf("[Skip] Isolated Go runtime is already verified in runtimes/%s", subDir))
			} else {
				e.AddLog(fmt.Sprintf("[Success] Provisioned isolated Go runtime into runtimes/%s", subDir))
			}
			return
		}

		var targetDir string
		if category == "phpmyadmin" || strings.HasPrefix(lower, "phpmyadmin") ||
			category == "mailpit" || strings.HasPrefix(lower, "mailpit") ||
			category == "redis" || strings.HasPrefix(lower, "redis") ||
			category == "mariadb" || strings.HasPrefix(lower, "mariadb") {
			return // Already handled in Stage 1
		} else if strings.HasPrefix(lower, "php-8.2") {
			targetDir = filepath.Join(coreWorkspace, "runtimes", "php-8.2")
		} else if strings.HasPrefix(lower, "php-8.3") {
			targetDir = filepath.Join(coreWorkspace, "runtimes", "php-8.3")
		} else if strings.HasPrefix(lower, "php-8.4") {
			targetDir = filepath.Join(coreWorkspace, "runtimes", "php-8.4")
		} else if strings.HasPrefix(lower, "php-8.5") {
			targetDir = filepath.Join(coreWorkspace, "runtimes", "php-8.5")
		} else if strings.HasPrefix(lower, "node-v20") {
			targetDir = filepath.Join(coreWorkspace, "runtimes", "node-v20")
		} else if strings.HasPrefix(lower, "node-v22") {
			targetDir = filepath.Join(coreWorkspace, "runtimes", "node-v22")
		} else if strings.HasPrefix(lower, "node-v24") {
			targetDir = filepath.Join(coreWorkspace, "runtimes", "node-v24")
		} else {
			subDir := strings.TrimSuffix(name, ".zip")
			subDir = strings.TrimSuffix(subDir, "-win64")
			targetDir = filepath.Join(coreWorkspace, "runtimes", subDir)
		}

		if info, err := os.Stat(targetDir); err != nil || !info.IsDir() {
			logger.Log("Unpacking Stage 2 runtime %s -> %s", name, filepath.ToSlash(filepath.Join("workspace", "runtimes", filepath.Base(targetDir))))
		}
		skipped, err := extractor.EnsureAsset(assetPath, targetDir)
		if err != nil {
			e.AddLog(fmt.Sprintf("[Error] Failed extracting %s: %v", name, err))
		} else if skipped {
			e.AddLog(fmt.Sprintf("[Skip] Asset %s is already verified in %s", name, filepath.Base(targetDir)))
		} else {
			e.AddLog(fmt.Sprintf("[Success] Extracted asset %s into %s successfully", name, filepath.Base(targetDir)))
		}
	}

	if entries, err := os.ReadDir(assetsDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				category := entry.Name()
				if category == "Wallpapers" || category == "Stack-Logo" {
					continue
				}
				catDir := filepath.Join(assetsDir, category)
				if subFiles, errSub := os.ReadDir(catDir); errSub == nil {
					for _, sf := range subFiles {
						if !sf.IsDir() && (strings.HasSuffix(strings.ToLower(sf.Name()), ".zip") || strings.HasSuffix(strings.ToLower(sf.Name()), ".phar")) {
							processAssetStage2(filepath.Join(catDir, sf.Name()), sf.Name(), category)
						}
					}
				}
			} else if strings.HasSuffix(strings.ToLower(entry.Name()), ".zip") || strings.HasSuffix(strings.ToLower(entry.Name()), ".phar") {
				processAssetStage2(filepath.Join(assetsDir, entry.Name()), entry.Name(), "")
			}
		}
	}

	// Sync dynamic database metadata now that all runtimes are present
	mariaRoot := filepath.Join(coreWorkspace, "shared-services", "mariadb")
	if dirs, err := os.ReadDir(mariaRoot); err == nil {
		for _, d := range dirs {
			if d.IsDir() && strings.HasPrefix(d.Name(), "mariadb-") {
				mariaRoot = filepath.Join(mariaRoot, d.Name())
				break
			}
		}
	}
	database.SyncInfrastructureMetadata(coreWorkspace, mariaRoot, "3306", "", "")

	// Check if this is the first launch runtimes initialization
	initFlagPath := filepath.Join(coreWorkspace, "core", "runtimes_initialized.flag")
	if _, errStat := os.Stat(initFlagPath); os.IsNotExist(errStat) {
		e.AddLog("[First Launch] Initializing factory default settings for all runtimes...")
		runtimes, errFetch := database.FetchInstalledRuntimes(mariaRoot, "3306")
		if errFetch == nil {
			for _, rt := range runtimes {
				if strings.HasPrefix(rt.Key, "php-") {
					e.AddLog(fmt.Sprintf("[First Launch] Resetting PHP config to factory defaults for %s", rt.Key))
					phpini.RestoreConfig(rt.Key, coreWorkspace)
				} else if strings.HasPrefix(rt.Key, "node-") {
					e.AddLog(fmt.Sprintf("[First Launch] Resetting Node.js config to factory defaults for %s", rt.Key))
					def := nodeconfig.GetDefaultConfig()
					nodeconfig.SaveConfig(coreWorkspace, rt.Key, def)
				} else if rt.Key == "go-runtime" || strings.HasPrefix(rt.Key, "go-") || rt.Key == "go" {
					e.AddLog(fmt.Sprintf("[First Launch] Resetting Go config to factory defaults for %s", rt.Key))
					def := goconfig.GetDefaultConfig()
					goconfig.SaveConfig(coreWorkspace, rt.Key, def)
				}
			}
		}
		os.WriteFile(initFlagPath, []byte("initialized"), 0644)
		e.AddLog("[First Launch] All runtimes successfully initialized to factory defaults!")
	}

	e.AddLog("All offline requirements and 3-pillar assets provisioned and synced to database!")
	notifier.Success("Precheck Complete", "System reached total stability! All runtimes verified and ready for project management.")

	e.mu.Lock()
	cb := e.OnComplete
	e.mu.Unlock()
	if cb != nil {
		cb()
	}
}

// RunFullProvisioning executes both Stage 1 and Stage 2 sequentially.
func (e *Engine) RunFullProvisioning(assetsDir string, coreWorkspace string) {
	e.RunStage1(assetsDir, coreWorkspace)
	e.RunStage2(assetsDir, coreWorkspace)
}

// EnsureDesktopShortcut creates a Windows shortcut (.lnk) on the user's Desktop pointing to the smart launcher script.
func EnsureDesktopShortcut(launcherPath, workDir string) error {
	if runtime.GOOS != "windows" {
		return nil
	}
	userProfile := os.Getenv("USERPROFILE")
	if userProfile == "" {
		return fmt.Errorf("USERPROFILE environment variable not found")
	}
	desktopDir := filepath.Join(userProfile, "Desktop")
	shortcutPath := filepath.Join(desktopDir, "V-CoPanel Windows.lnk")

	iconPath := filepath.Join(workDir, "internal", "icon.ico")
	if icoData, errRead := os.ReadFile(iconPath); errRead == nil {
		_ = os.WriteFile(filepath.Join(workDir, "pc-assets", "icon.ico"), icoData, 0644)
	} else {
		iconPath = filepath.Join(workDir, "pc-assets", "icon.ico")
	}

	if _, err := os.Stat(iconPath); err != nil {
		// No fallback icon available anymore
		iconPath = ""
	}

	if iconPath != "" {
		psCmd := fmt.Sprintf(`$s=(New-Object -COM WScript.Shell).CreateShortcut('%s');$s.TargetPath='%s';$s.WorkingDirectory='%s';if(Test-Path '%s'){$s.IconLocation='%s'};$s.Save()`,
			shortcutPath, launcherPath, workDir, iconPath, iconPath)
		cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psCmd)
		return cmd.Run()
	}

	psCmd := fmt.Sprintf(`$s=(New-Object -COM WScript.Shell).CreateShortcut('%s');$s.TargetPath='%s';$s.WorkingDirectory='%s';$s.Save()`,
		shortcutPath, launcherPath, workDir)

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psCmd)
	return cmd.Run()
}




