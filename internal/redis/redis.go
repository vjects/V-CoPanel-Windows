package redis

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"vcopanel-bridge/internal/logger"
)

type RedisStatus struct {
	Installed        bool   `json:"installed"`
	Running          bool   `json:"running"`
	Version          string `json:"version"`
	Port             int    `json:"port"`
	MemoryUse        string `json:"memory_use"`
	ConnectedClients string `json:"connected_clients"`
	TotalKeys        string `json:"total_keys"`
}

var redisCmd *exec.Cmd

func getRedisDir(workspaceDir string) string {
	dir := filepath.Join(workspaceDir, "shared-services", "redis")
	if _, err := os.Stat(dir); err != nil {
		dir = filepath.Join(workspaceDir, "redis")
	}
	return dir
}

func GetStatus(workspaceDir string) RedisStatus {
	baseDir := getRedisDir(workspaceDir)
	srvExe := filepath.Join(baseDir, "redis-server.exe")
	cliExe := filepath.Join(baseDir, "redis-cli.exe")

	installed := false
	if _, err := os.Stat(srvExe); err == nil {
		installed = true
	}

	if !installed {
		return RedisStatus{Installed: false, Port: 6379, MemoryUse: "0 B", ConnectedClients: "0", TotalKeys: "0"}
	}

	running := false
	mem := "0 B"
	clients := "0"
	keys := "0"
	ver := "Redis 7.2 Portable"

	if out, err := exec.Command(cliExe, "-p", "6379", "ping").Output(); err == nil && strings.Contains(string(out), "PONG") {
		running = true
		if infoOut, err := exec.Command(cliExe, "-p", "6379", "info").Output(); err == nil {
			lines := strings.Split(string(infoOut), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "used_memory_human:") {
					mem = strings.TrimSpace(strings.TrimPrefix(line, "used_memory_human:"))
				}
				if strings.HasPrefix(line, "connected_clients:") {
					clients = strings.TrimSpace(strings.TrimPrefix(line, "connected_clients:"))
				}
				if strings.HasPrefix(line, "db0:") {
					parts := strings.Split(line, ",")
					for _, p := range parts {
						if strings.Contains(p, "keys=") {
							sub := strings.Split(p, "keys=")
							if len(sub) > 1 {
								keys = strings.TrimSpace(sub[1])
							}
						}
					}
				}
			}
		}
	}

	return RedisStatus{
		Installed:        true,
		Running:          running,
		Version:          ver,
		Port:             6379,
		MemoryUse:        mem,
		ConnectedClients: clients,
		TotalKeys:        keys,
	}
}

func Start(workspaceDir string) error {
	baseDir := getRedisDir(workspaceDir)
	srvExe := filepath.Join(baseDir, "redis-server.exe")
	confFile := filepath.Join(baseDir, "redis.windows.conf")
	if _, err := os.Stat(confFile); os.IsNotExist(err) {
		confFile = filepath.Join(baseDir, "redis.conf")
	}

	args := []string{}
	if _, err := os.Stat(confFile); err == nil {
		args = append(args, confFile)
	}

	cmd := exec.Command(srvExe, args...)
	cmd.Dir = baseDir
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("redis start error: %w", err)
	}
	redisCmd = cmd
	logger.Log("Redis Cache Server : Active on port 6379 (PID: %d)", cmd.Process.Pid)
	return nil
}

func Stop(workspaceDir string) error {
	logger.Log("Terminating Redis server...")
	baseDir := getRedisDir(workspaceDir)
	cliExe := filepath.Join(baseDir, "redis-cli.exe")
	err := exec.Command(cliExe, "-p", "6379", "shutdown").Run()
	if err == nil {
		logger.Log("Redis server shut down successfully.")
	}
	return err
}

func Flush(workspaceDir string) error {
	baseDir := getRedisDir(workspaceDir)
	cliExe := filepath.Join(baseDir, "redis-cli.exe")
	out, err := exec.Command(cliExe, "-p", "6379", "flushall").CombinedOutput()
	if err != nil {
		return fmt.Errorf("flushall failed: %s (%w)", string(out), err)
	}
	return nil
}

func UpdateConfig(workspaceDir, maxMemory, maxMemoryPolicy string) error {
	baseDir := getRedisDir(workspaceDir)
	os.MkdirAll(baseDir, 0755)
	confFile := filepath.Join(baseDir, "redis.windows.conf")
	if _, err := os.Stat(confFile); os.IsNotExist(err) {
		confFile = filepath.Join(baseDir, "redis.conf")
	}
	var lines []string
	if content, err := os.ReadFile(confFile); err == nil {
		lines = strings.Split(string(content), "\n")
	} else {
		confFile = filepath.Join(baseDir, "redis.windows.conf")
		lines = []string{"# V-CoPanel Redis Configuration"}
	}
	foundMem, foundPol := false, false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "maxmemory ") && !strings.HasPrefix(trimmed, "maxmemory-policy") {
			lines[i] = fmt.Sprintf("maxmemory %s", maxMemory)
			foundMem = true
		} else if strings.HasPrefix(trimmed, "maxmemory-policy ") {
			lines[i] = fmt.Sprintf("maxmemory-policy %s", maxMemoryPolicy)
			foundPol = true
		}
	}
	if !foundMem {
		lines = append(lines, fmt.Sprintf("maxmemory %s", maxMemory))
	}
	if !foundPol {
		lines = append(lines, fmt.Sprintf("maxmemory-policy %s", maxMemoryPolicy))
	}
	logger.Log("Updated Redis config | Max Memory: %s | Policy: %s", maxMemory, maxMemoryPolicy)
	return os.WriteFile(confFile, []byte(strings.Join(lines, "\n")+"\n"), 0644)
}

func GetConfig(workspaceDir string) (string, string) {
	baseDir := getRedisDir(workspaceDir)
	confFile := filepath.Join(baseDir, "redis.windows.conf")
	if _, err := os.Stat(confFile); os.IsNotExist(err) {
		confFile = filepath.Join(baseDir, "redis.conf")
	}
	mem := "256mb"
	pol := "allkeys-lru"
	if content, err := os.ReadFile(confFile); err == nil {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "maxmemory ") && !strings.HasPrefix(trimmed, "maxmemory-policy") {
				parts := strings.Fields(trimmed)
				if len(parts) >= 2 {
					mem = parts[1]
				}
			} else if strings.HasPrefix(trimmed, "maxmemory-policy ") {
				parts := strings.Fields(trimmed)
				if len(parts) >= 2 {
					pol = parts[1]
				}
			}
		}
	}
	return mem, pol
}
