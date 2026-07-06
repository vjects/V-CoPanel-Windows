package goconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"vcopanel-bridge/internal/logger"
)

type Config struct {
	GoProxy    string `json:"goproxy"`
	CgoEnabled string `json:"cgo_enabled"`
}

var mu sync.Mutex

func GetDefaultConfig() Config {
	return Config{
		GoProxy:    "https://proxy.golang.org,direct",
		CgoEnabled: "0",
	}
}

func GetConfig(workspaceDir string, versionId string) Config {
	mu.Lock()
	defer mu.Unlock()
	cfgDir := workspaceDir
	if versionId != "" && versionId != "global" {
		cfgDir = filepath.Join(workspaceDir, "runtimes", versionId)
	}
	cfgPath := filepath.Join(cfgDir, "goconfig.json")
	if data, err := os.ReadFile(cfgPath); err == nil {
		var c Config
		if errDecode := json.Unmarshal(data, &c); errDecode == nil {
			if c.GoProxy == "" {
				c.GoProxy = "https://proxy.golang.org,direct"
			}
			if c.CgoEnabled == "" {
				c.CgoEnabled = "0"
			}
			return c
		}
	}
	def := GetDefaultConfig()
	return def
}

func SaveConfig(workspaceDir string, versionId string, c Config) error {
	mu.Lock()
	defer mu.Unlock()
	if c.GoProxy == "" {
		c.GoProxy = "https://proxy.golang.org,direct"
	}
	if c.CgoEnabled == "" {
		c.CgoEnabled = "0"
	}
	
	cfgDir := workspaceDir
	if versionId != "" && versionId != "global" {
		cfgDir = filepath.Join(workspaceDir, "runtimes", versionId)
	}
	os.MkdirAll(cfgDir, 0755)
	
	cfgPath := filepath.Join(cfgDir, "goconfig.json")
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(cfgPath, data, 0644); err != nil {
		return err
	}

	goEnvContent := fmt.Sprintf("GOPROXY=%s\nCGO_ENABLED=%s\n", c.GoProxy, c.CgoEnabled)
	os.WriteFile(filepath.Join(cfgDir, "go.env"), []byte(goEnvContent), 0644)
	
	goExe := filepath.Join(cfgDir, "bin", "go.exe")
	if _, errExe := os.Stat(goExe); errExe == nil {
		exec.Command(goExe, "env", "-w", "GOPROXY="+c.GoProxy).Run()
		exec.Command(goExe, "env", "-w", "CGO_ENABLED="+c.CgoEnabled).Run()
	}
	
	// Fallback for global
	if versionId == "global" {
		for _, sub := range []string{"runtimes/go", "core/go", "go"} {
			dir := filepath.Join(workspaceDir, sub)
			if _, errDir := os.Stat(dir); errDir == nil {
				os.WriteFile(filepath.Join(dir, "go.env"), []byte(goEnvContent), 0644)
				gExe := filepath.Join(dir, "bin", "go.exe")
				if _, errExe := os.Stat(gExe); errExe == nil {
					exec.Command(gExe, "env", "-w", "GOPROXY="+c.GoProxy).Run()
					exec.Command(gExe, "env", "-w", "CGO_ENABLED="+c.CgoEnabled).Run()
				}
			}
		}
	}
	
	logger.Log("Updated Go config (%s) | Proxy: %s | CGO: %s", versionId, c.GoProxy, c.CgoEnabled)
	return nil
}

func CleanCache(workspaceDir string, versionId string) (string, error) {
	cfgDir := workspaceDir
	if versionId != "" && versionId != "global" {
		cfgDir = filepath.Join(workspaceDir, "runtimes", versionId)
	}
	
	goExe := filepath.Join(cfgDir, "bin", "go.exe")
	if _, err := os.Stat(goExe); err != nil {
		goExe = filepath.Join(cfgDir, "go.exe")
	}
	
	// Fallbacks
	if _, err := os.Stat(goExe); err != nil {
		goExe = filepath.Join(workspaceDir, "runtimes", "go", "bin", "go.exe")
	}
	if _, err := os.Stat(goExe); err != nil {
		goExe = filepath.Join(workspaceDir, "core", "go", "bin", "go.exe")
	}
	if _, err := os.Stat(goExe); err != nil {
		goExe = filepath.Join(workspaceDir, "go", "bin", "go.exe")
	}

	cmd := exec.Command(goExe, "clean", "-cache", "-modcache")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("go cache clean failed: %v (%s)", err, string(out))
	}
	return "Go build cache and module cache cleaned successfully.", nil
}
