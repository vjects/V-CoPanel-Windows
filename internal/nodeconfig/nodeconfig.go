package nodeconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"vcopanel-bridge/internal/logger"
)


type Config struct {
	Registry       string `json:"registry"`
	MaxMemoryMB    int    `json:"max_memory_mb"`
	NodeOptions    string `json:"node_options"`
}

var mu sync.Mutex

func GetDefaultConfig() Config {
	return Config{
		Registry:    "https://registry.npmjs.org/",
		MaxMemoryMB: 4096,
		NodeOptions: "--max-old-space-size=4096",
	}
}

func GetConfig(workspaceDir string, versionId string) Config {
	mu.Lock()
	defer mu.Unlock()
	
	cfgDir := workspaceDir
	if versionId != "" && versionId != "global" {
		cfgDir = filepath.Join(workspaceDir, "runtimes", versionId)
	}
	cfgPath := filepath.Join(cfgDir, "nodeconfig.json")
	
	if data, err := os.ReadFile(cfgPath); err == nil {
		var c Config
		if errDecode := json.Unmarshal(data, &c); errDecode == nil {
			if c.MaxMemoryMB == 0 {
				c.MaxMemoryMB = 4096
			}
			if c.NodeOptions == "" {
				c.NodeOptions = fmt.Sprintf("--max-old-space-size=%d", c.MaxMemoryMB)
			}
			

			
			return c
		}
	}
	def := GetDefaultConfig()
	// Do not auto-save on Get to prevent deadlocks with mu.Lock inside SaveConfig
	


	return def
}

func SaveConfig(workspaceDir string, versionId string, c Config) error {
	mu.Lock()
	defer mu.Unlock()
	if c.MaxMemoryMB <= 0 {
		c.MaxMemoryMB = 4096
	}
	c.NodeOptions = fmt.Sprintf("--max-old-space-size=%d", c.MaxMemoryMB)
	
	cfgDir := workspaceDir
	if versionId != "" && versionId != "global" {
		cfgDir = filepath.Join(workspaceDir, "runtimes", versionId)
	}
	os.MkdirAll(cfgDir, 0755)
	
	cfgPath := filepath.Join(cfgDir, "nodeconfig.json")
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(cfgPath, data, 0644); err != nil {
		return err
	}

	npmrcContent := fmt.Sprintf("registry=%s\n", c.Registry)
	os.WriteFile(filepath.Join(cfgDir, ".npmrc"), []byte(npmrcContent), 0644)
	if dirs, errDir := os.ReadDir(filepath.Join(workspaceDir, "runtimes")); errDir == nil {
		for _, d := range dirs {
			if d.IsDir() && strings.HasPrefix(strings.ToLower(d.Name()), "node") {
				os.WriteFile(filepath.Join(workspaceDir, "runtimes", d.Name(), ".npmrc"), []byte(npmrcContent), 0644)
			}
		}
	}
	logger.Log("Updated Node.js config (%s) | Max Memory: %d MB | Registry: %s", versionId, c.MaxMemoryMB, c.Registry)
	return nil
}

func CleanCache(workspaceDir string, versionId string) (string, error) {
	nodeDir := filepath.Join(workspaceDir, "runtimes", "node")
	if versionId != "" && versionId != "global" {
		nodeDir = filepath.Join(workspaceDir, "runtimes", versionId)
	}
	
	npmCmd := filepath.Join(nodeDir, "npm.cmd")
	if _, err := os.Stat(npmCmd); err != nil {
		for _, v := range []string{"node-v24", "node-v22", "node-v20", "node"} {
			cand := filepath.Join(workspaceDir, "runtimes", v, "npm.cmd")
			if _, errStat := os.Stat(cand); errStat == nil {
				npmCmd = cand
				break
			}
		}
	}

	cmd := exec.Command(npmCmd, "cache", "clean", "--force")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("npm cache clean failed: %v (%s)", err, string(out))
	}
	return string(out), nil
}
