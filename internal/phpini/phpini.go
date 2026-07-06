package phpini

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"vcopanel-bridge/internal/logger"
)

type PHPConfig struct {
	MemoryLimit       string          `json:"memory_limit"`
	PostMaxSize       string          `json:"post_max_size"`
	UploadMaxFilesize string          `json:"upload_max_filesize"`
	MaxExecutionTime  string          `json:"max_execution_time"`
	MaxInputTime      string          `json:"max_input_time"`
	MaxInputVars      string          `json:"max_input_vars"`
	DisplayErrors     string          `json:"display_errors"`
	Timezone          string          `json:"timezone"`
	Extensions        map[string]bool `json:"extensions"`
	HasBackup         bool            `json:"has_backup"`
}

// GetDefaultConfig returns a robust, optimal default configuration for modern Laravel applications.
func GetDefaultConfig() PHPConfig {
	return PHPConfig{
		MemoryLimit:       "512M",
		PostMaxSize:       "128M",
		UploadMaxFilesize: "128M",
		MaxExecutionTime:  "300",
		MaxInputTime:      "60",
		MaxInputVars:      "5000",
		DisplayErrors:     "On",
		Timezone:          "UTC",
		Extensions: map[string]bool{
			"curl":       true,
			"mbstring":   true,
			"openssl":    true,
			"pdo_mysql":  true,
			"mysqli":     true,
			"fileinfo":   true,
			"gd":         true,
			"zip":        true,
			"intl":       true,
			"exif":       true,
			"bcmath":     false,
			"soap":       true,
			"sockets":    true,
			"pdo_sqlite": true,
		},
	}
}

// EnsureDefaultIni checks if php.ini exists. If not (or if uninitialized), creates it with optimal defaults.
func EnsureDefaultIni(phpVersion, workspaceDir string) error {
	phpDir := findBinaryDir(filepath.Join(workspaceDir, "runtimes", phpVersion), "php.exe")
	if phpDir == "" {
		phpDir = findBinaryDir(filepath.Join(workspaceDir, phpVersion), "php.exe")
	}
	if phpDir == "" {
		phpDir = filepath.Join(workspaceDir, "runtimes", phpVersion)
	}
	iniPath := filepath.Join(phpDir, "php.ini")
	if _, err := os.Stat(iniPath); err != nil {
		devIni := filepath.Join(phpDir, "php.ini-development")
		if content, errDev := os.ReadFile(devIni); errDev == nil {
			os.WriteFile(iniPath, content, 0644)
		}
		cfg := GetDefaultConfig()
		return UpdateConfig(phpVersion, workspaceDir, cfg)
	}
	return nil
}

// GetConfig reads php.ini from portable PHP folder.
func GetConfig(phpVersion, workspaceDir string) (*PHPConfig, error) {
	phpDir := findBinaryDir(filepath.Join(workspaceDir, "runtimes", phpVersion), "php.exe")
	if phpDir == "" {
		phpDir = findBinaryDir(filepath.Join(workspaceDir, phpVersion), "php.exe")
	}
	if phpDir == "" {
		phpDir = filepath.Join(workspaceDir, "runtimes", phpVersion)
	}
	iniPath := filepath.Join(phpDir, "php.ini")
	isNew := false
	if _, err := os.Stat(iniPath); err != nil {
		devIni := filepath.Join(phpDir, "php.ini-development")
		if content, errDev := os.ReadFile(devIni); errDev == nil {
			os.WriteFile(iniPath, content, 0644)
			isNew = true
		}
	}

	cfgVal := GetDefaultConfig()
	bakPath := filepath.Join(phpDir, "php.ini.bak")
	if _, err := os.Stat(bakPath); err == nil {
		cfgVal.HasBackup = true
	}
	cfg := &cfgVal

	if isNew {
		UpdateConfig(phpVersion, workspaceDir, *cfg)
	}

	lines, err := readLines(iniPath)
	if err != nil {
		return cfg, nil
	}

	for ext := range cfg.Extensions {
		cfg.Extensions[ext] = false
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "memory_limit") && strings.Contains(trimmed, "=") {
			cfg.MemoryLimit = extractVal(trimmed)
		}
		if strings.HasPrefix(trimmed, "post_max_size") && strings.Contains(trimmed, "=") {
			cfg.PostMaxSize = extractVal(trimmed)
		}
		if strings.HasPrefix(trimmed, "upload_max_filesize") && strings.Contains(trimmed, "=") {
			cfg.UploadMaxFilesize = extractVal(trimmed)
		}
		if strings.HasPrefix(trimmed, "max_execution_time") && strings.Contains(trimmed, "=") {
			cfg.MaxExecutionTime = extractVal(trimmed)
		}
		if strings.HasPrefix(trimmed, "max_input_time") && strings.Contains(trimmed, "=") {
			cfg.MaxInputTime = extractVal(trimmed)
		}
		if strings.HasPrefix(trimmed, "max_input_vars") && strings.Contains(trimmed, "=") {
			cfg.MaxInputVars = extractVal(trimmed)
		}
		if strings.HasPrefix(trimmed, "display_errors") && strings.Contains(trimmed, "=") {
			cfg.DisplayErrors = extractVal(trimmed)
		}
		if strings.HasPrefix(trimmed, "date.timezone") && strings.Contains(trimmed, "=") {
			cfg.Timezone = extractVal(trimmed)
		}

		for ext := range cfg.Extensions {
			if trimmed == "extension="+ext || trimmed == "extension=php_"+ext || trimmed == "extension=php_"+ext+".dll" || strings.HasPrefix(trimmed, "extension="+ext+" ") || strings.HasPrefix(trimmed, "extension="+ext+"\t") {
				cfg.Extensions[ext] = true
			}
		}
	}

	return cfg, nil
}

func extractVal(line string) string {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) == 2 {
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, "\"")
		return strings.Split(val, ";")[0]
	}
	return ""
}

// UpdateConfig edits php.ini memory limits, performance parameters, and extensions.
func UpdateConfig(phpVersion, workspaceDir string, newCfg PHPConfig) error {
	phpDir := findBinaryDir(filepath.Join(workspaceDir, "runtimes", phpVersion), "php.exe")
	if phpDir == "" {
		phpDir = findBinaryDir(filepath.Join(workspaceDir, phpVersion), "php.exe")
	}
	if phpDir == "" {
		phpDir = filepath.Join(workspaceDir, "runtimes", phpVersion)
	}
	os.MkdirAll(phpDir, 0755)
	iniPath := filepath.Join(phpDir, "php.ini")
	if _, errStat := os.Stat(iniPath); errStat != nil {
		EnsureDefaultIni(phpVersion, workspaceDir)
	}
	bakPath := filepath.Join(phpDir, "php.ini.bak")
	if _, errStat := os.Stat(bakPath); errStat != nil {
		if origBytes, errRead := os.ReadFile(iniPath); errRead == nil {
			os.WriteFile(bakPath, origBytes, 0644)
		}
	}

	lines, err := readLines(iniPath)
	if err != nil {
		return err
	}

	seenExt := make(map[string]bool)
	updatedKeys := make(map[string]bool)
	var updated []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if (strings.HasPrefix(trimmed, "memory_limit") || strings.HasPrefix(trimmed, ";memory_limit")) && strings.Contains(trimmed, "=") {
			if !updatedKeys["memory_limit"] {
				updated = append(updated, "memory_limit = "+newCfg.MemoryLimit)
				updatedKeys["memory_limit"] = true
			}
			continue
		}
		if (strings.HasPrefix(trimmed, "post_max_size") || strings.HasPrefix(trimmed, ";post_max_size")) && strings.Contains(trimmed, "=") {
			if !updatedKeys["post_max_size"] {
				updated = append(updated, "post_max_size = "+newCfg.PostMaxSize)
				updatedKeys["post_max_size"] = true
			}
			continue
		}
		if (strings.HasPrefix(trimmed, "upload_max_filesize") || strings.HasPrefix(trimmed, ";upload_max_filesize")) && strings.Contains(trimmed, "=") {
			if !updatedKeys["upload_max_filesize"] {
				updated = append(updated, "upload_max_filesize = "+newCfg.UploadMaxFilesize)
				updatedKeys["upload_max_filesize"] = true
			}
			continue
		}
		if (strings.HasPrefix(trimmed, "max_execution_time") || strings.HasPrefix(trimmed, ";max_execution_time")) && strings.Contains(trimmed, "=") {
			if !updatedKeys["max_execution_time"] {
				updated = append(updated, "max_execution_time = "+newCfg.MaxExecutionTime)
				updatedKeys["max_execution_time"] = true
			}
			continue
		}
		if (strings.HasPrefix(trimmed, "max_input_time") || strings.HasPrefix(trimmed, ";max_input_time")) && strings.Contains(trimmed, "=") {
			if !updatedKeys["max_input_time"] {
				updated = append(updated, "max_input_time = "+newCfg.MaxInputTime)
				updatedKeys["max_input_time"] = true
			}
			continue
		}
		if (strings.HasPrefix(trimmed, "max_input_vars") || strings.HasPrefix(trimmed, ";max_input_vars")) && strings.Contains(trimmed, "=") {
			if !updatedKeys["max_input_vars"] {
				updated = append(updated, "max_input_vars = "+newCfg.MaxInputVars)
				updatedKeys["max_input_vars"] = true
			}
			continue
		}
		if strings.HasPrefix(trimmed, "display_errors") && strings.Contains(trimmed, "=") && !strings.HasPrefix(trimmed, "display_errors_") {
			if !updatedKeys["display_errors"] {
				updated = append(updated, "display_errors = "+newCfg.DisplayErrors)
				updatedKeys["display_errors"] = true
			}
			continue
		}
		if (strings.HasPrefix(trimmed, "date.timezone") || strings.HasPrefix(trimmed, ";date.timezone")) && strings.Contains(trimmed, "=") {
			if !updatedKeys["date.timezone"] {
				updated = append(updated, "date.timezone = \""+newCfg.Timezone+"\"")
				updatedKeys["date.timezone"] = true
			}
			continue
		}
		if strings.HasPrefix(trimmed, ";extension_dir = \"ext\"") || strings.HasPrefix(trimmed, "extension_dir =") {
			extAbsPath := filepath.Join(filepath.Dir(iniPath), "ext")
			updated = append(updated, fmt.Sprintf("extension_dir = \"%s\"", extAbsPath))
			continue
		}

		isExtLine := false
		for ext, enabled := range newCfg.Extensions {
			if strings.Contains(trimmed, "extension="+ext) || strings.Contains(trimmed, "extension=php_"+ext) {
				isExtLine = true
				if enabled && !seenExt[ext] {
					updated = append(updated, "extension="+ext)
					seenExt[ext] = true
				} else {
					updated = append(updated, ";extension="+ext)
				}
				break
			}
		}
		if !isExtLine {
			updated = append(updated, line)
		}
	}

	if !updatedKeys["memory_limit"] && newCfg.MemoryLimit != "" {
		updated = append(updated, "memory_limit = "+newCfg.MemoryLimit)
	}
	if !updatedKeys["post_max_size"] && newCfg.PostMaxSize != "" {
		updated = append(updated, "post_max_size = "+newCfg.PostMaxSize)
	}
	if !updatedKeys["upload_max_filesize"] && newCfg.UploadMaxFilesize != "" {
		updated = append(updated, "upload_max_filesize = "+newCfg.UploadMaxFilesize)
	}
	if !updatedKeys["max_execution_time"] && newCfg.MaxExecutionTime != "" {
		updated = append(updated, "max_execution_time = "+newCfg.MaxExecutionTime)
	}
	if !updatedKeys["max_input_time"] && newCfg.MaxInputTime != "" {
		updated = append(updated, "max_input_time = "+newCfg.MaxInputTime)
	}
	if !updatedKeys["max_input_vars"] && newCfg.MaxInputVars != "" {
		updated = append(updated, "max_input_vars = "+newCfg.MaxInputVars)
	}
	if !updatedKeys["display_errors"] && newCfg.DisplayErrors != "" {
		updated = append(updated, "display_errors = "+newCfg.DisplayErrors)
	}
	if !updatedKeys["date.timezone"] && newCfg.Timezone != "" {
		updated = append(updated, "date.timezone = \""+newCfg.Timezone+"\"")
	}

	// Append any missing extensions that are enabled
	for ext, enabled := range newCfg.Extensions {
		if enabled && !seenExt[ext] {
			updated = append(updated, "extension="+ext)
		}
	}

	logger.Log("[%s] Updated php.ini | Memory: %s | MaxExec: %s | Timezone: %s", phpVersion, newCfg.MemoryLimit, newCfg.MaxExecutionTime, newCfg.Timezone)
	return os.WriteFile(iniPath, []byte(strings.Join(updated, "\n")+"\n"), 0644)
}

// RestoreConfig restores php.ini from php.ini.bak if available, or falls back to EnsureDefaultIni.
func RestoreConfig(phpVersion, workspaceDir string) error {
	phpDir := findBinaryDir(filepath.Join(workspaceDir, "runtimes", phpVersion), "php.exe")
	if phpDir == "" {
		phpDir = findBinaryDir(filepath.Join(workspaceDir, phpVersion), "php.exe")
	}
	if phpDir == "" {
		phpDir = filepath.Join(workspaceDir, "runtimes", phpVersion)
	}
	iniPath := filepath.Join(phpDir, "php.ini")
	bakPath := filepath.Join(phpDir, "php.ini.bak")

	if content, err := os.ReadFile(bakPath); err == nil {
		os.WriteFile(iniPath, content, 0644)
	} else {
		os.Remove(iniPath)
		EnsureDefaultIni(phpVersion, workspaceDir)
	}
	cfg := GetDefaultConfig()
	return UpdateConfig(phpVersion, workspaceDir, cfg)
}

func readLines(path string) ([]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	rawLines := strings.Split(string(content), "\n")
	var lines []string
	for _, l := range rawLines {
		lines = append(lines, strings.TrimRight(l, "\r"))
	}
	return lines, nil
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
