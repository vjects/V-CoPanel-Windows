package server

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"vcopanel-bridge/internal/database"
	"vcopanel-bridge/internal/discovery"
	"vcopanel-bridge/internal/logger"
	"vcopanel-bridge/internal/mesh"
	"vcopanel-bridge/internal/notifier"
	"vcopanel-bridge/internal/process"
)

type ProjectManager struct {
	mu            sync.Mutex
	serveProcs    map[string]*exec.Cmd
	servePorts    map[string]string
	queueProcs    map[string]*exec.Cmd
	scheduleProcs map[string]*exec.Cmd
	commandOutput map[string]string
	MariaRoot     string
	DBPort        string
}

var Instance = &ProjectManager{
	serveProcs:    make(map[string]*exec.Cmd),
	servePorts:    make(map[string]string),
	queueProcs:    make(map[string]*exec.Cmd),
	scheduleProcs: make(map[string]*exec.Cmd),
	commandOutput: make(map[string]string),
}

func (pm *ProjectManager) StartServe(projectPath, port, phpVersion, workspaceDir, stackType, script, entrypoint, mode string) error {
	pm.mu.Lock()
	if _, running := pm.serveProcs[projectPath]; running {
		pm.mu.Unlock()
		return nil
	}
	pm.mu.Unlock()

	conn, err := net.DialTimeout("tcp", "127.0.0.1:"+port, 150*time.Millisecond)
	if err == nil {
		conn.Close()
		errStr := fmt.Sprintf("Port %s is already in use by another server or project. Please select a different port", port)
		notifier.Error("Port Busy", fmt.Sprintf("[%s] %s", filepath.Base(projectPath), errStr))
		return fmt.Errorf(errStr)
	}

	if stackType == "" {
		if _, err := os.Stat(filepath.Join(projectPath, "artisan")); err == nil {
			stackType = "laravel"
		} else if _, err := os.Stat(filepath.Join(projectPath, "go.mod")); err == nil {
			stackType = "go"
		} else if _, err := os.Stat(filepath.Join(projectPath, "main.go")); err == nil {
			stackType = "go"
		} else if _, err := os.Stat(filepath.Join(projectPath, "package.json")); err == nil {
			if data, _ := os.ReadFile(filepath.Join(projectPath, "package.json")); strings.Contains(strings.ToLower(string(data)), "next") {
				stackType = "nextjs"
			} else if data != nil && strings.Contains(strings.ToLower(string(data)), "vue") {
				stackType = "vue"
			} else {
				stackType = "nodejs"
			}
		} else if _, err := os.Stat(filepath.Join(projectPath, "index.php")); err == nil {
			stackType = "simple-php"
		} else {
			stackType = "laravel"
		}
	}

	var cmd *exec.Cmd
	var desc string

	switch stackType {
	case "go":
		goBat := filepath.Join(projectPath, "go.bat")
		if _, err := os.Stat(goBat); err != nil {
			errStr := "go.bat shim not found. Run provision first"
			notifier.Error("Provision Required", fmt.Sprintf("[%s] %s", filepath.Base(projectPath), errStr))
			return fmt.Errorf(errStr)
		}
		cmd = exec.Command("cmd.exe", "/c", goBat, "run", ".")
		cmd.Env = append(os.Environ(), "PORT="+port, "APP_PORT="+port)
		desc = "Go server"
	case "nextjs", "vue", "node", "nodejs":
		npmCmd := filepath.Join(projectPath, "npm.cmd")
		if _, err := os.Stat(npmCmd); err != nil {
			npmCmd = "npm.cmd"
		}
		if script != "" && script != "start" && script != "dev" {
			cmd = exec.Command("cmd.exe", "/c", npmCmd, "run", script)
		} else if stackType == "nextjs" {
			if script == "start" {
				cmd = exec.Command("cmd.exe", "/c", npmCmd, "start", "--", "-p", port)
			} else {
				cmd = exec.Command("cmd.exe", "/c", npmCmd, "run", "dev", "--", "-p", port)
			}
		} else if stackType == "vue" {
			cmd = exec.Command("cmd.exe", "/c", npmCmd, "run", "dev", "--", "--port", port)
		} else {
			if entrypoint == "" { entrypoint = "index.js" }
			nodeBat := filepath.Join(projectPath, "node.bat")
			if _, err := os.Stat(nodeBat); err != nil {
				nodeBat = "node"
			}
			cmd = exec.Command("cmd.exe", "/c", nodeBat, entrypoint)
		}
		cmd.Env = append(os.Environ(), "PORT="+port)
		desc = stackType + " server"
	case "php", "simple-php", "simple_php":
		phpBat := filepath.Join(projectPath, "php.bat")
		if _, err := os.Stat(phpBat); err != nil {
			errStr := "php.bat shim not found. Run provision first"
			notifier.Error("Provision Required", fmt.Sprintf("[%s] %s", filepath.Base(projectPath), errStr))
			return fmt.Errorf(errStr)
		}
		docRoot := "."
		if info, err := os.Stat(filepath.Join(projectPath, "public")); err == nil && info.IsDir() {
			docRoot = "public"
		}
		cmd = exec.Command("cmd.exe", "/c", phpBat, "-S", "127.0.0.1:"+port, "-t", docRoot)
		desc = "PHP built-in server"
	case "generic", "sandbox":
		if _, err := os.Stat(filepath.Join(projectPath, "package.json")); err == nil {
			npmCmd := filepath.Join(projectPath, "npm.cmd")
			if _, err := os.Stat(npmCmd); err != nil { npmCmd = "npm.cmd" }
			if script == "" { script = "start" }
			cmd = exec.Command("cmd.exe", "/c", npmCmd, "run", script)
		} else if _, err := os.Stat(filepath.Join(projectPath, "index.php")); err == nil {
			phpBat := filepath.Join(projectPath, "php.bat")
			if _, err := os.Stat(phpBat); err != nil { phpBat = "php" }
			cmd = exec.Command("cmd.exe", "/c", phpBat, "-S", "127.0.0.1:"+port, "-t", ".")
		} else {
			phpBat := filepath.Join(projectPath, "php.bat")
			if _, err := os.Stat(phpBat); err != nil { phpBat = "php" }
			cmd = exec.Command("cmd.exe", "/c", phpBat, "-S", "127.0.0.1:"+port, "-t", ".")
		}
		cmd.Env = append(os.Environ(), "PORT="+port)
		desc = stackType + " server"
	default:
		phpBat := filepath.Join(projectPath, "php.bat")
		if _, err := os.Stat(phpBat); err != nil {
			errStr := "php.bat shim not found. Run provision first"
			notifier.Error("Provision Required", fmt.Sprintf("[%s] %s", filepath.Base(projectPath), errStr))
			return fmt.Errorf(errStr)
		}
		cmd = exec.Command("cmd.exe", "/c", phpBat, "artisan", "serve", "--port="+port)
		desc = "Laravel development server"
	}

	cmd.Dir = projectPath
	logDir := filepath.Join(projectPath, "storage", "logs")
	os.MkdirAll(logDir, 0755)
	logFile := filepath.Join(logDir, "app.log")
	if stackType == "laravel" {
		logFile = filepath.Join(logDir, "laravel.log")
	}
	stdoutPipe, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
	if err := cmd.Start(); err != nil {
		notifier.Error("Server Failed", fmt.Sprintf("[%s] Failed to start application server: %v", filepath.Base(projectPath), err))
		return err
	}

	pid := cmd.Process.Pid
	procName := "server.exe"
	if stackType == "go" {
		procName = "go.exe"
	} else if stackType == "laravel" || stackType == "php" || stackType == "simple-php" {
		procName = "php.exe"
	} else {
		procName = "node.exe"
	}

	f, _ := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	go processPipe(stdoutPipe, f, pid, procName)
	go processPipe(stderrPipe, f, pid, procName)

	process.BindProcess(pid)

	pm.mu.Lock()
	pm.serveProcs[projectPath] = cmd
	pm.servePorts[projectPath] = port
	
	if pm.MariaRoot != "" {
		uuid := discovery.GenerateUUID(projectPath)
		q := fmt.Sprintf("INSERT INTO project_runtime_state (project_uuid, state, pid, port, last_started_at) VALUES ('%s', 'running', %d, '%s', NOW()) ON DUPLICATE KEY UPDATE state='running', pid=%d, port='%s', last_started_at=NOW()", uuid, cmd.Process.Pid, port, cmd.Process.Pid, port)
		database.ExecuteDynamicQuery(pm.MariaRoot, pm.DBPort, q)
	}
	mesh.GlobalRegistry.Register(filepath.Base(projectPath), port)

	pm.mu.Unlock()

	go func(p string, c *exec.Cmd, logFilePtr *os.File) {
		c.Wait()
		if logFilePtr != nil {
			logFilePtr.Close()
		}
		pm.mu.Lock()
		if cur, ok := pm.serveProcs[p]; ok && cur == c {
			delete(pm.serveProcs, p)
			if pm.MariaRoot != "" {
				uuid := discovery.GenerateUUID(p)
				q := fmt.Sprintf("UPDATE project_runtime_state SET state='stopped', last_stopped_at=NOW() WHERE project_uuid='%s'", uuid)
				database.ExecuteDynamicQuery(pm.MariaRoot, pm.DBPort, q)
			}
			mesh.GlobalRegistry.Deregister(filepath.Base(p))
		}
		pm.mu.Unlock()
	}(projectPath, cmd, f)

	projName := filepath.Base(projectPath)
	logger.Log("Started %s for %s on port %s (PID: %d)", desc, projName, port, cmd.Process.Pid)
	notifier.Success("Server Started", fmt.Sprintf("[%s] %s active on port %s", projName, desc, port))
	return nil
}

func (pm *ProjectManager) StopServe(projectPath string) error {
	pm.mu.Lock()
	cmd, exists := pm.serveProcs[projectPath]
	if exists {
		delete(pm.serveProcs, projectPath)
		delete(pm.servePorts, projectPath)
		if pm.MariaRoot != "" {
			uuid := discovery.GenerateUUID(projectPath)
			q := fmt.Sprintf("UPDATE project_runtime_state SET state='stopped', last_stopped_at=NOW() WHERE project_uuid='%s'", uuid)
			database.ExecuteDynamicQuery(pm.MariaRoot, pm.DBPort, q)
		}
		mesh.GlobalRegistry.Deregister(filepath.Base(projectPath))
	}
	pm.mu.Unlock()

	if !exists || cmd == nil || cmd.Process == nil {
		return nil
	}

	projName := filepath.Base(projectPath)
	logger.Log("Stopping application server for %s...", projName)
	exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", cmd.Process.Pid)).Run()
	notifier.Info("Server Stopped", fmt.Sprintf("[%s] Application development server shut down", projName))
	return nil
}

func (pm *ProjectManager) IsServeRunning(projectPath string) bool {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	_, exists := pm.serveProcs[projectPath]
	return exists
}

func (pm *ProjectManager) GetServePort(projectPath string) string {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return pm.servePorts[projectPath]
}

func (pm *ProjectManager) StartQueue(projectPath, phpVersion, workspaceDir string) error {
	pm.mu.Lock()
	if _, running := pm.queueProcs[projectPath]; running {
		pm.mu.Unlock()
		return nil
	}
	pm.mu.Unlock()

	queueBat := filepath.Join(projectPath, "queue.bat")
	if _, err := os.Stat(queueBat); os.IsNotExist(err) {
		queueContent := `@echo off
chcp 65001 >nul 2>&1
echo [+] Starting Laravel Queue Listener...
"%~dp0php.bat" artisan queue:listen --tries=3 --timeout=90 --sleep=1
`
		os.WriteFile(queueBat, []byte(queueContent), 0755)
	}

	cmd := exec.Command("cmd.exe", "/c", queueBat)
	cmd.Dir = projectPath
	stdoutPipe, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
	if err := cmd.Start(); err != nil {
		pm.mu.Lock()
		delete(pm.queueProcs, projectPath)
		pm.mu.Unlock()
		return fmt.Errorf("failed starting queue worker: %v", err)
	}

	pid := cmd.Process.Pid
	go processPipe(stdoutPipe, nil, pid, "php.exe")
	go processPipe(stderrPipe, nil, pid, "php.exe")

	process.BindProcess(pid)

	pm.mu.Lock()
	pm.queueProcs[projectPath] = cmd
	pm.mu.Unlock()

	projName := filepath.Base(projectPath)
	logger.Log("Started queue intermediary script (queue.bat) for %s (PID: %d)", projName, cmd.Process.Pid)
	notifier.Info("Queue Listener", fmt.Sprintf("[%s] Queue worker started successfully", projName))

	go func(p string, c *exec.Cmd) {
		c.Wait()
		pm.mu.Lock()
		delete(pm.queueProcs, p)
		pm.mu.Unlock()
		logger.Log("Queue worker process ended for %s", filepath.Base(p))
		notifier.Warning("Queue Listener Stopped", fmt.Sprintf("[%s] Queue worker process terminated", filepath.Base(p)))
	}(projectPath, cmd)

	return nil
}

func (pm *ProjectManager) StopQueue(projectPath string) error {
	pm.mu.Lock()
	cmd, exists := pm.queueProcs[projectPath]
	if !exists {
		pm.mu.Unlock()
		return nil
	}
	delete(pm.queueProcs, projectPath)
	pm.mu.Unlock()

	if cmd != nil && cmd.Process != nil {
		exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", cmd.Process.Pid)).Run()
	}
	projName := filepath.Base(projectPath)
	logger.Log("Stopped queue worker for %s (PID: %d)", projName, cmd.Process.Pid)
	notifier.Info("Queue Listener Stopped", fmt.Sprintf("[%s] Queue worker stopped", projName))
	return nil
}

func (pm *ProjectManager) IsQueueRunning(projectPath string) bool {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	_, exists := pm.queueProcs[projectPath]
	return exists
}

func (pm *ProjectManager) StartSchedule(projectPath, phpVersion, workspaceDir string) error {
	pm.mu.Lock()
	if _, running := pm.scheduleProcs[projectPath]; running {
		pm.mu.Unlock()
		return nil
	}
	pm.mu.Unlock()

	scheduleBat := filepath.Join(projectPath, "schedule.bat")
	if _, err := os.Stat(scheduleBat); os.IsNotExist(err) {
		scheduleContent := `@echo off
chcp 65001 >nul 2>&1
echo [+] Starting Laravel Schedule Worker...
"%~dp0php.bat" artisan schedule:work
`
		os.WriteFile(scheduleBat, []byte(scheduleContent), 0755)
	}

	cmd := exec.Command("cmd.exe", "/c", scheduleBat)
	cmd.Dir = projectPath
	stdoutPipe, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
	if err := cmd.Start(); err != nil {
		notifier.Error("Schedule Failed", fmt.Sprintf("[%s] Failed starting schedule worker: %v", filepath.Base(projectPath), err))
		return err
	}

	pid := cmd.Process.Pid
	go processPipe(stdoutPipe, nil, pid, "php.exe")
	go processPipe(stderrPipe, nil, pid, "php.exe")

	process.BindProcess(pid)

	pm.mu.Lock()
	pm.scheduleProcs[projectPath] = cmd
	pm.mu.Unlock()

	go func(p string, c *exec.Cmd) {
		c.Wait()
		pm.mu.Lock()
		if cur, ok := pm.scheduleProcs[p]; ok && cur == c {
			delete(pm.scheduleProcs, p)
		}
		pm.mu.Unlock()
	}(projectPath, cmd)

	projName := filepath.Base(projectPath)
	logger.Log("Started schedule worker (schedule.bat) for %s (PID: %d)", projName, cmd.Process.Pid)
	notifier.Success("Schedule Active", fmt.Sprintf("[%s] Laravel cron schedule worker started", projName))
	return nil
}

func (pm *ProjectManager) StopSchedule(projectPath string) error {
	pm.mu.Lock()
	cmd, exists := pm.scheduleProcs[projectPath]
	if exists {
		delete(pm.scheduleProcs, projectPath)
	}
	pm.mu.Unlock()

	if !exists || cmd == nil || cmd.Process == nil {
		return nil
	}

	projName := filepath.Base(projectPath)
	logger.Log("Stopping schedule worker for %s...", projName)
	exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", cmd.Process.Pid)).Run()
	notifier.Info("Schedule Stopped", fmt.Sprintf("[%s] Terminated schedule worker", projName))
	return nil
}

func (pm *ProjectManager) IsScheduleRunning(projectPath string) bool {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	_, exists := pm.scheduleProcs[projectPath]
	return exists
}

// RunCommand executes quick artisan or composer/npm commands asynchronously with real-time log streaming.
func (pm *ProjectManager) RunCommand(projectPath, cmdType string) error {
	var cmd *exec.Cmd
	switch cmdType {
	case "migrate:fresh":
		cmd = exec.Command("cmd.exe", "/c", "php.bat", "artisan", "migrate:fresh", "--seed")
	case "optimize:clear":
		cmd = exec.Command("cmd.exe", "/c", "php.bat", "artisan", "optimize:clear")
	case "storage:link":
		// On Windows, artisan storage:link requires admin for symlinks.
		// Junction Points work without admin and are equivalent for web serving.
		publicStoragePath := filepath.Join(projectPath, "public", "storage")
		storagePublicPath := filepath.Join(projectPath, "storage", "app", "public")
		os.MkdirAll(storagePublicPath, 0755)
		os.RemoveAll(publicStoragePath)
		cmd = exec.Command("cmd.exe", "/c", "mklink", "/J", publicStoragePath, storagePublicPath)
	case "key:generate":
		cmd = exec.Command("cmd.exe", "/c", "php.bat", "artisan", "key:generate")
	case "composer-install":
		cmd = exec.Command("cmd.exe", "/c", "composer.bat", "install", "--no-interaction")
	case "npm-install":
		cmd = exec.Command("cmd.exe", "/c", "npm.cmd", "install")
	case "npm-build":
		cmd = exec.Command("cmd.exe", "/c", "npm.cmd", "run", "build")
	default:
		return fmt.Errorf("unknown command type: %s", cmdType)
	}

	cmd.Dir = projectPath

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	pm.mu.Lock()
	pm.commandOutput[projectPath] = fmt.Sprintf("[START] [%s] Starting command: %s...\n", filepath.Base(projectPath), cmdType)
	pm.mu.Unlock()

	if err := cmd.Start(); err != nil {
		notifier.Error("Command Failed", fmt.Sprintf("[%s] Failed starting %s: %v", filepath.Base(projectPath), cmdType, err))
		return err
	}

	projName := filepath.Base(projectPath)
	logger.Log(">>> [Command Started] %s on %s <<<", cmdType, projName)
	notifier.Info("Command Running", fmt.Sprintf("[%s] Executing %s...", projName, cmdType))

	var wg sync.WaitGroup
	wg.Add(2)

	pid := cmd.Process.Pid
	procNameCmd := filepath.Base(cmd.Path)
	readPipe := func(r io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := scanner.Text()
			logger.LogProc(pid, procNameCmd, "[%s | %s] %s", projName, cmdType, line)
			pm.mu.Lock()
			pm.commandOutput[projectPath] += line + "\n"
			pm.mu.Unlock()
		}
	}

	go readPipe(stdoutPipe)
	go readPipe(stderrPipe)

	go func() {
		wg.Wait()
		errWait := cmd.Wait()
		logger.Log("<<< [Command Finished] %s on %s >>>", cmdType, projName)
		pm.mu.Lock()
		if errWait != nil {
			pm.commandOutput[projectPath] += fmt.Sprintf("\n[ERROR] [%s] Command finished with error: %v\n", cmdType, errWait)
			notifier.Error("Command Error", fmt.Sprintf("[%s] %s failed: %v", projName, cmdType, errWait))
		} else {
			pm.commandOutput[projectPath] += fmt.Sprintf("\n[SUCCESS] [%s] Command finished successfully!\n", cmdType)
			notifier.Success("Command Complete", fmt.Sprintf("[%s] Successfully executed %s", projName, cmdType))
		}
		pm.mu.Unlock()
	}()

	return nil
}

func (pm *ProjectManager) GetCommandOutput(projectPath string) string {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return pm.commandOutput[projectPath]
}

func (pm *ProjectManager) ClearCommandOutput(projectPath string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if projectPath == "" {
		for k := range pm.commandOutput {
			pm.commandOutput[k] = ""
		}
	} else {
		pm.commandOutput[projectPath] = ""
	}
}

func (pm *ProjectManager) StopAll() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	for pPath, cmd := range pm.serveProcs {
		if cmd != nil && cmd.Process != nil {
			logger.Log("Stopping serve for %s...", filepath.Base(pPath))
			exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", cmd.Process.Pid)).Run()
		}
	}
	for pPath, cmd := range pm.queueProcs {
		if cmd != nil && cmd.Process != nil {
			logger.Log("Stopping queue for %s...", filepath.Base(pPath))
			exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", cmd.Process.Pid)).Run()
		}
	}
	for pPath, cmd := range pm.scheduleProcs {
		if cmd != nil && cmd.Process != nil {
			logger.Log("Stopping schedule worker for %s...", filepath.Base(pPath))
			exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", cmd.Process.Pid)).Run()
		}
	}
}

func processPipe(pipe io.Reader, logFile *os.File, pid int, procName string) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		line := scanner.Text()
		if logFile != nil {
			logFile.WriteString(line + "\n")
		}
		logger.LogProc(pid, procName, "%s", line)
	}
}

type ActiveProcessInfo struct {
	ProjectPath string `json:"project_path"`
	ProjectName string `json:"project_name"`
	Type        string `json:"type"` // "serve" or "queue"
	PID         int    `json:"pid"`
	Port        string `json:"port"`
}

func (pm *ProjectManager) GetActiveProcesses() []ActiveProcessInfo {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	var list []ActiveProcessInfo
	for path, cmd := range pm.serveProcs {
		if cmd != nil && cmd.Process != nil {
			list = append(list, ActiveProcessInfo{
				ProjectPath: path,
				ProjectName: filepath.Base(path),
				Type:        "serve",
				PID:         cmd.Process.Pid,
				Port:        pm.servePorts[path],
			})
		}
	}
	for path, cmd := range pm.queueProcs {
		if cmd != nil && cmd.Process != nil {
			list = append(list, ActiveProcessInfo{
				ProjectPath: path,
				ProjectName: filepath.Base(path),
				Type:        "queue",
				PID:         cmd.Process.Pid,
				Port:        "",
			})
		}
	}
	return list
}
