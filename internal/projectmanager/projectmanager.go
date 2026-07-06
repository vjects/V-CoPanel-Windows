package projectmanager

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"vcopanel-bridge/internal/database"
	"vcopanel-bridge/internal/notifier"
)

type BackupInfo struct {
	Filename  string `json:"filename"`
	SizeBytes int64  `json:"size_bytes"`
	CreatedAt string `json:"created_at"`
}

// GetEnv reads the .env file and returns its raw content and key-value pairs.
func GetEnv(projectPath string) (string, map[string]string, error) {
	envPath := filepath.Join(projectPath, ".env")
	content, err := os.ReadFile(envPath)
	if err != nil {
		return "", nil, err
	}
	raw := string(content)
	kv := make(map[string]string)
	lines := strings.Split(raw, "\n")
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) == 2 {
			kv[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return raw, kv, nil
}

// SaveEnv writes raw content to .env file.
func SaveEnv(projectPath string, content string) error {
	envPath := filepath.Join(projectPath, ".env")
	return os.WriteFile(envPath, []byte(content), 0644)
}

// GetEnvDiff returns keys present in .env.example but missing in .env.
func GetEnvDiff(projectPath string) ([]string, error) {
	_, envKV, err := GetEnv(projectPath)
	if err != nil {
		return nil, err
	}
	exPath := filepath.Join(projectPath, ".env.example")
	exContent, err := os.ReadFile(exPath)
	if err != nil {
		return []string{}, nil
	}
	var missing []string
	lines := strings.Split(string(exContent), "\n")
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) > 0 {
			key := strings.TrimSpace(parts[0])
			if key != "" {
				if _, ok := envKV[key]; !ok {
					missing = append(missing, key)
				}
			}
		}
	}
	return missing, nil
}

// extractDBName parses .env to find DB_DATABASE.
func extractDBName(projectPath string) string {
	_, kv, err := GetEnv(projectPath)
	if err == nil {
		if db, ok := kv["DB_DATABASE"]; ok && db != "" {
			return strings.Trim(db, "\"'/")
		}
	}
	return ""
}

// CreateBackup creates an instant SQL dump of the project's database.
func CreateBackup(projectPath, mariaRoot string) (string, error) {
	dbName := extractDBName(projectPath)
	if dbName == "" {
		return "", fmt.Errorf("DB_DATABASE is not defined in .env file")
	}

	mariaRoot = database.ResolveMariaRoot(mariaRoot)
	mysqlExe := filepath.Join(mariaRoot, "bin", "mysql.exe")
	mysqldumpExe := filepath.Join(mariaRoot, "bin", "mysqldump.exe")
	if _, err := os.Stat(mysqldumpExe); err != nil {
		return "", fmt.Errorf("mysqldump binary not found at %s", mysqldumpExe)
	}

	// Pre-check if database exists
	checkCmd := exec.Command(mysqlExe, "-u", "root", "-P", "3306", "-h", "127.0.0.1", "-e", fmt.Sprintf("USE `%s`;", dbName))
	if err := checkCmd.Run(); err != nil {
		return "", fmt.Errorf("Database `%s` does not exist or is inaccessible in MariaDB. Run `migrate` or provision first.", dbName)
	}

	backupDir := filepath.Join(projectPath, "storage", "app", "vcopanel_backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", err
	}

	filename := fmt.Sprintf("%s_%s.sql", dbName, time.Now().Format("20060102_150405"))
	outPath := filepath.Join(backupDir, filename)

	// Use direct file redirect instead of cmd.exe /c to avoid Windows path quoting issues
	outFile, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("failed to create snapshot file: %v", err)
	}
	defer outFile.Close()

	dumpCmd := exec.Command(mysqldumpExe, "-u", "root", "-P", "3306", "-h", "127.0.0.1", "--databases", dbName)
	dumpCmd.Stdout = outFile
	dumpCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if out, err := dumpCmd.CombinedOutput(); err != nil {
		os.Remove(outPath) // clean up empty file on failure
		return "", fmt.Errorf("mysqldump failed: %s (%v)", string(out), err)
	}

	return filename, nil
}

// ListBackups lists all created SQL snapshot files.
func ListBackups(projectPath string) ([]BackupInfo, error) {
	backupDir := filepath.Join(projectPath, "storage", "app", "vcopanel_backups")
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return []BackupInfo{}, nil
	}
	var res []BackupInfo
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			info, _ := e.Info()
			res = append(res, BackupInfo{
				Filename:  e.Name(),
				SizeBytes: info.Size(),
				CreatedAt: info.ModTime().Format("2006-01-02 15:04:05"),
			})
		}
	}
	return res, nil
}

// RestoreBackup restores a specific SQL snapshot file.
func RestoreBackup(projectPath, filename, mariaRoot string) error {
	dbName := extractDBName(projectPath)
	if dbName == "" {
		return fmt.Errorf("DB_DATABASE is not defined in .env file")
	}
	backupPath := filepath.Join(projectPath, "storage", "app", "vcopanel_backups", filename)
	if _, err := os.Stat(backupPath); err != nil {
		return fmt.Errorf("Backup file %s not found", filename)
	}
	mariaRoot = database.ResolveMariaRoot(mariaRoot)
	mysqlExe := filepath.Join(mariaRoot, "bin", "mysql.exe")

	// Use direct Stdin file redirect instead of cmd.exe /c to avoid Windows path quoting issues
	inFile, err := os.Open(backupPath)
	if err != nil {
		return fmt.Errorf("failed to open backup file: %v", err)
	}
	defer inFile.Close()

	restoreCmd := exec.Command(mysqlExe, "-u", "root", "-P", "3306", "-h", "127.0.0.1", dbName)
	restoreCmd.Stdin = inFile
	restoreCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if out, err := restoreCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Restore failed: %s (%v)", string(out), err)
	}
	return nil
}

// DeleteBackup deletes a specific SQL snapshot file.
func DeleteBackup(projectPath, filename string) error {
	backupPath := filepath.Join(projectPath, "storage", "app", "vcopanel_backups", filename)
	return os.Remove(backupPath)
}

// ExecArtisanCommand executes an arbitrary artisan command synchronously and returns the output.
func ExecArtisanCommand(projectPath, cmdStr string) (string, error) {
	phpBat := filepath.Join(projectPath, "php.bat")
	projName := filepath.Base(projectPath)
	if _, err := os.Stat(phpBat); err != nil {
		errMsg := "php.bat shim not found in project directory. Please provision project first."
		notifier.Error("Artisan Failed", fmt.Sprintf("[%s] %s", projName, errMsg))
		return "", fmt.Errorf(errMsg)
	}
	args := strings.Fields(cmdStr)
	fullArgs := append([]string{"artisan"}, args...)
	cmd := exec.Command(phpBat, fullArgs...)
	cmd.Dir = projectPath
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		notifier.Error("Artisan Error", fmt.Sprintf("[%s] artisan %s failed: %v", projName, cmdStr, err))
	} else {
		notifier.Success("Artisan Executed", fmt.Sprintf("[%s] artisan %s completed successfully", projName, cmdStr))
	}
	return string(out), err
}

// ExecGoCommand executes an arbitrary go command synchronously using go.bat.
func ExecGoCommand(projectPath, cmdStr string) (string, error) {
	goBat := filepath.Join(projectPath, "go.bat")
	projName := filepath.Base(projectPath)
	if _, err := os.Stat(goBat); err != nil {
		errMsg := "go.bat shim not found in project directory. Please provision project first."
		notifier.Error("Go Command Failed", fmt.Sprintf("[%s] %s", projName, errMsg))
		return "", fmt.Errorf(errMsg)
	}
	args := strings.Fields(cmdStr)
	cmd := exec.Command(goBat, args...)
	cmd.Dir = projectPath
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		notifier.Error("Go Command Error", fmt.Sprintf("[%s] go %s failed: %v", projName, cmdStr, err))
	} else {
		notifier.Success("Go Executed", fmt.Sprintf("[%s] go %s completed successfully", projName, cmdStr))
	}
	return string(out), err
}

type CronJobInfo struct {
	ID        string `json:"id"`
	Line      string `json:"line"`
	CreatedAt string `json:"created_at"`
}

func ListCronJobs(mariaRoot, dbPort, projectUUID string) ([]CronJobInfo, error) {
	dbJobs, err := database.FetchCronJobs(mariaRoot, dbPort, projectUUID)
	if err != nil {
		return []CronJobInfo{}, err
	}
	var list []CronJobInfo
	for _, j := range dbJobs {
		list = append(list, CronJobInfo{
			ID:        j.ID,
			Line:      j.Command,
			CreatedAt: j.CreatedAt,
		})
	}
	return list, nil
}

func AddCronJob(mariaRoot, dbPort, projectUUID, line string) (*CronJobInfo, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, fmt.Errorf("cron job syntax cannot be empty")
	}
	id := fmt.Sprintf("%d", time.Now().UnixNano())
	createdAt := time.Now().Format("2006-01-02 15:04:05")

	safeLine := strings.ReplaceAll(line, "'", "''")

	q := fmt.Sprintf("INSERT INTO `project_cron_jobs` (id, project_uuid, cron_expression, command, created_at) VALUES ('%s', '%s', 'legacy', '%s', '%s');", id, projectUUID, safeLine, createdAt)
	if err := database.ExecuteDynamicQuery(mariaRoot, dbPort, q); err != nil {
		return nil, err
	}

	return &CronJobInfo{
		ID:        id,
		Line:      line,
		CreatedAt: createdAt,
	}, nil
}

func DeleteCronJob(mariaRoot, dbPort, projectUUID, id string) error {
	q := fmt.Sprintf("DELETE FROM `project_cron_jobs` WHERE id='%s' AND project_uuid='%s';", id, projectUUID)
	return database.ExecuteDynamicQuery(mariaRoot, dbPort, q)
}

func RunCronJob(mariaRoot, dbPort, projectUUID, projectPath, id string) (string, error) {
	list, err := ListCronJobs(mariaRoot, dbPort, projectUUID)
	if err != nil {
		return "", err
	}
	var target *CronJobInfo
	for _, item := range list {
		if item.ID == id {
			target = &item
			break
		}
	}
	if target == nil {
		return "", fmt.Errorf("cron job not found")
	}

	cmdStr := strings.TrimSpace(target.Line)
	parts := strings.Fields(cmdStr)
	if len(parts) > 5 && isCronToken(parts[0]) && isCronToken(parts[1]) && isCronToken(parts[2]) && isCronToken(parts[3]) && isCronToken(parts[4]) {
		cmdStr = strings.Join(parts[5:], " ")
	} else if len(parts) > 1 && strings.HasPrefix(parts[0], "@") {
		cmdStr = strings.Join(parts[1:], " ")
	}

	if idx := strings.Index(cmdStr, ">>"); idx != -1 {
		cmdStr = strings.TrimSpace(cmdStr[:idx])
	} else if idx := strings.Index(cmdStr, ">"); idx != -1 {
		cmdStr = strings.TrimSpace(cmdStr[:idx])
	}

	if idx := strings.Index(cmdStr, "artisan "); idx != -1 {
		artisanArgs := strings.TrimSpace(cmdStr[idx+len("artisan "):])
		return ExecArtisanCommand(projectPath, artisanArgs)
	}

	cmd := exec.Command("cmd.exe", "/c", cmdStr)
	cmd.Dir = projectPath
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func isCronToken(s string) bool {
	return s == "*" || strings.HasPrefix(s, "*/") || strings.ContainsAny(s, "0123456789,-")
}
