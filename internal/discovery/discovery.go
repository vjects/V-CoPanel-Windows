package discovery

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type ProjectInfo struct {
	UUID          string `json:"uuid"`
	Name          string `json:"name"`
	Path          string `json:"path"`
	Stack         string `json:"stack"`         // "laravel", "go", "node", "vue", "nextjs", "php"
	Framework     string `json:"framework"`     // "Laravel", "Go", "Next.js", "Vue.js", "React", "Node.js"
	Status        string `json:"status"`        // "Pending" or "Configured"
	PHPVersion    string `json:"php_version"`
	NodeVersion   string `json:"node_version"`
	GoVersion     string `json:"go_version"`
	Port          string `json:"port"`
	DBName        string `json:"db_name"`
}

// DetectStackAndFramework inspects files inside projectPath to identify application stack & framework.
func DetectStackAndFramework(projectPath string) (string, string) {
	if data, err := os.ReadFile(filepath.Join(projectPath, ".vcopanel-runtime.json")); err == nil {
		var rt struct {
			Stack     string `json:"stack"`
			Framework string `json:"framework"`
		}
		if err := json.Unmarshal(data, &rt); err == nil && rt.Stack != "" {
			fw := rt.Framework
			if fw == "" {
				// Title case the stack as fallback
				fw = strings.ToUpper(rt.Stack[:1]) + rt.Stack[1:]
			}
			return rt.Stack, fw
		}
	}

	if _, err := os.Stat(filepath.Join(projectPath, "artisan")); err == nil {
		return "laravel", "Laravel"
	}
	if _, err := os.Stat(filepath.Join(projectPath, "go.mod")); err == nil {
		return "go", "Go"
	}
	if _, err := os.Stat(filepath.Join(projectPath, "main.go")); err == nil {
		return "go", "Go"
	}
	if _, err := os.Stat(filepath.Join(projectPath, "package.json")); err == nil {
		return "node", "Node.js"
	}
	
	return "other", "Other"
}

// detectStackAndFramework is the internal alias kept for backward compatibility.
func detectStackAndFramework(projectPath string) (string, string) {
	return DetectStackAndFramework(projectPath)
}

// detectProjectStack inspects files inside projectPath to identify the application stack.
func detectProjectStack(projectPath string) string {
	s, _ := DetectStackAndFramework(projectPath)
	return s
}

// ScanProjects discovers multi-stack applications in workspaceDir and workspaceDir/projects.
func ScanProjects(workspaceDir string, configuredPaths map[string]ProjectInfo) ([]ProjectInfo, error) {
	var results []ProjectInfo

	if workspaceDir == "" {
		return results, nil
	}

	dirsToScan := []string{workspaceDir}
	projectsSub := filepath.Join(workspaceDir, "projects")
	if info, err := os.Stat(projectsSub); err == nil && info.IsDir() {
		dirsToScan = append(dirsToScan, projectsSub)
	}

	for _, scanDir := range dirsToScan {
		entries, err := os.ReadDir(scanDir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			if scanDir == workspaceDir && (entry.Name() == "projects" || entry.Name() == "core" || entry.Name() == "runtimes" || entry.Name() == "shared-services" || entry.Name() == "wallpapers" || strings.HasPrefix(entry.Name(), ".")) {
				continue
			}
			if strings.HasPrefix(entry.Name(), ".") {
				continue
			}

			projectPath := filepath.Join(scanDir, entry.Name())
			stack, framework := detectStackAndFramework(projectPath)

			if stack != "" {
				dbName := GetProjectDBName(projectPath)

				if cfg, exists := configuredPaths[projectPath]; exists {
					cfg.DBName = dbName
					if cfg.Stack == "" {
						cfg.Stack = stack
					}
					if cfg.Framework == "" {
						cfg.Framework = framework
					}
					cfg.Status = checkDesyncStatus(projectPath, cfg.Status)
					results = append(results, cfg)
					continue
				}

				// Check if project was already configured on disk (.tools-version or php.bat)
				toolsVerPath := filepath.Join(projectPath, ".tools-version")
				phpBatPath := filepath.Join(projectPath, "php.bat")
				
				// Generate deterministic UUID
				hasher := md5.New()
				hasher.Write([]byte(strings.ToLower(filepath.ToSlash(filepath.Clean(projectPath)))))
				uuid := hex.EncodeToString(hasher.Sum(nil))
				
				if _, errTV := os.Stat(toolsVerPath); errTV == nil {
					phpVer, nodeVer, goVer := parseToolsVersion(toolsVerPath)
					port := parseEnvPort(filepath.Join(projectPath, ".env"))
					if port == "" {
						port = fmt.Sprintf("%d", 8001+len(results))
					}
					info := ProjectInfo{
						UUID:          uuid,
						Name:          entry.Name(),
						Path:          projectPath,
						Stack:         stack,
						Framework:     framework,
						Status:        checkDesyncStatus(projectPath, "Configured"),
						PHPVersion:    phpVer,
						NodeVersion:   nodeVer,
						GoVersion:     goVer,
						Port:          port,
						DBName:        dbName,
					}
					configuredPaths[projectPath] = info
					results = append(results, info)
				} else if _, errPB := os.Stat(phpBatPath); errPB == nil {
					info := ProjectInfo{
						UUID:        uuid,
						Name:        entry.Name(),
						Path:        projectPath,
						Stack:       stack,
						Framework:   framework,
						Status:      checkDesyncStatus(projectPath, "Configured"),
						PHPVersion:  "php-8.3",
						NodeVersion: "node-v22",
						Port:        fmt.Sprintf("%d", 8001+len(results)),
						DBName:      dbName,
					}
					configuredPaths[projectPath] = info
					results = append(results, info)
				} else {
					results = append(results, ProjectInfo{
						UUID:      uuid,
						Name:      entry.Name(),
						Path:      projectPath,
						Stack:     stack,
						Framework: framework,
						Status:    "Pending",
						Port:      fmt.Sprintf("%d", 8001+len(results)),
						DBName:    dbName,
					})
				}
			}
		}
	}

	return results, nil
}

func checkDesyncStatus(projectPath string, currentStatus string) string {
	if currentStatus == "Pending" || currentStatus == "Unconfigured" || currentStatus == "Ejected" {
		return currentStatus
	}
	toolsVerPath := filepath.Join(projectPath, ".tools-version")
	envPath := filepath.Join(projectPath, ".env")
	
	if tvData, err := os.ReadFile(toolsVerPath); err == nil {
		if !strings.Contains(string(tvData), "V-COPANEL MANAGED RUNTIME LOCK") && !strings.Contains(string(tvData), "V-CoPanel Portable Tool Versions") {
			return "Desynced"
		}
	} else {
		return "Desynced"
	}

	if envData, err := os.ReadFile(envPath); err == nil {
		if !strings.Contains(string(envData), "V-COPANEL MANAGED") {
			return "Desynced"
		}
	}

	if manifestData, err := os.ReadFile(filepath.Join(projectPath, ".vcopanel-runtime.json")); err == nil {
		if !strings.Contains(string(manifestData), `"port"`) {
			return "Desynced"
		}
	}
	return "Configured"
}

// parseToolsVersion reads .tools-version file and returns php, node, go versions.
// Supports both whitespace-separated ("php php-8.3") and colon-separated ("php: php-8.3") formats.
func parseToolsVersion(filePath string) (phpVer, nodeVer, goVer string) {
	phpVer = "php-8.3"
	nodeVer = "node-v22"
	goVer = ""
	data, err := os.ReadFile(filePath)
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Normalize: remove colon after key ("php: php-8.3" → "php php-8.3")
		normalized := strings.ReplaceAll(line, ":", "")
		parts := strings.Fields(normalized)
		if len(parts) < 2 {
			continue
		}
		switch parts[0] {
		case "php":
			phpVer = parts[1]
		case "nodejs", "node":
			nodeVer = parts[1]

		case "go":
			goVer = parts[1]
		}
	}
	return
}

func parseEnvPort(envPath string) string {
	data, err := os.ReadFile(envPath)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Laravel / all stacks: APP_URL=http://127.0.0.1:8001
		if strings.HasPrefix(line, "APP_URL=") {
			val := strings.TrimPrefix(line, "APP_URL=")
			val = strings.Trim(val, "\"'")
			parts := strings.Split(val, ":")
			if len(parts) >= 3 {
				return strings.TrimRight(parts[len(parts)-1], "/")
			}
		}
		// Go / Node.js: PORT=8002
		if strings.HasPrefix(line, "PORT=") {
			v := strings.TrimSpace(strings.TrimPrefix(line, "PORT="))
			if v != "" {
				return v
			}
		}
		// FastAPI / alternate: APP_PORT=8003
		if strings.HasPrefix(line, "APP_PORT=") {
			v := strings.TrimSpace(strings.TrimPrefix(line, "APP_PORT="))
			if v != "" {
				return v
			}
		}
	}
	return ""
}

func GetProjectDBName(projectPath string) string {
	envPath := filepath.Join(projectPath, ".env")
	data, err := os.ReadFile(envPath)
	if err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "DB_DATABASE=") {
				val := strings.TrimPrefix(line, "DB_DATABASE=")
				val = strings.TrimSpace(strings.Trim(val, "\"'"))
				if val != "" {
					return val
				}
			}
		}
	}
	reg := regexp.MustCompile("[^a-zA-Z0-9_]+")
	cleanName := reg.ReplaceAllString(filepath.Base(projectPath), "_")
	return strings.ToLower("app_" + cleanName)
}

func GenerateUUID(path string) string {
	hash := md5.Sum([]byte(filepath.ToSlash(filepath.Clean(path))))
	return hex.EncodeToString(hash[:])
}


