package database

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"vcopanel-bridge/internal/logger"
)

type AdminCreds struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Host     string `json:"host"`
	Port     string `json:"port"`
}

// ==============================================================================
// ⚙️ RESTRICTED CONFIGURATIONS (DATABASE CREDENTIALS)
// ==============================================================================
var (
	Creds = AdminCreds{
		Username: "admin",
		Password: "Admin_VCoPanel_2026!",
		Host:     "127.0.0.1",
		Port:     "3306",
	}
	pmaMu      sync.Mutex
	pmaProcess *exec.Cmd
)
// ==============================================================================

// ResolveMariaRoot automatically detects the true MariaDB directory (e.g. inside mariadb-11.4.2-winx64 subfolder)
func ResolveMariaRoot(mariaDir string) string {
	if _, err := os.Stat(filepath.Join(mariaDir, "bin", "mysql.exe")); err == nil {
		return mariaDir
	}
	if dirs, err := os.ReadDir(mariaDir); err == nil {
		for _, d := range dirs {
			if d.IsDir() && len(d.Name()) > 8 && d.Name()[:8] == "mariadb-" {
				sub := filepath.Join(mariaDir, d.Name())
				if _, err2 := os.Stat(filepath.Join(sub, "bin", "mysql.exe")); err2 == nil {
					return sub
				}
			}
		}
	}
	return mariaDir
}

// InitSystemDB connects to MariaDB and creates vcopanel-db and the global admin user.
func InitSystemDB(mariaDir string, port string) error {
	mariaDir = ResolveMariaRoot(mariaDir)
	mysqlExe := filepath.Join(mariaDir, "bin", "mysql.exe")

	queries := []string{
		"CREATE DATABASE IF NOT EXISTS `vcopanel-db` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;",
		fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'localhost' IDENTIFIED BY '%s';", Creds.Username, Creds.Password),
		fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'127.0.0.1' IDENTIFIED BY '%s';", Creds.Username, Creds.Password),
		fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED BY '%s';", Creds.Username, Creds.Password),
		fmt.Sprintf("GRANT ALL PRIVILEGES ON *.* TO '%s'@'localhost' WITH GRANT OPTION;", Creds.Username),
		fmt.Sprintf("GRANT ALL PRIVILEGES ON *.* TO '%s'@'127.0.0.1' WITH GRANT OPTION;", Creds.Username),
		fmt.Sprintf("GRANT ALL PRIVILEGES ON *.* TO '%s'@'%%' WITH GRANT OPTION;", Creds.Username),
		"FLUSH PRIVILEGES;",
	}

	fullQuery := ""
	for _, q := range queries {
		fullQuery += q + " "
	}

	logger.Log("Initializing MariaDB System Schema & Admin user privileges...")

	var lastErr error
	for i := 0; i < 10; i++ {
		cmd := exec.Command(mysqlExe, "-u", "root", "-P", port, "-h", "127.0.0.1", "-e", fullQuery)
		output, err := cmd.CombinedOutput()
		if err == nil {
			logger.Log("MariaDB Database Server : Active on port %s (Admin: `%s`)", port, Creds.Username)
			return nil
		}
		cmd2 := exec.Command(mysqlExe, "-u", "root", "-P", port, "-e", fullQuery)
		output2, err2 := cmd2.CombinedOutput()
		if err2 == nil {
			logger.Log("MariaDB Database Server : Active on port %s (Admin: `%s`)", port, Creds.Username)
			return nil
		}
		cmd3 := exec.Command(mysqlExe, "-u", "root", fmt.Sprintf("-p%s", Creds.Password), "-P", port, "-h", "127.0.0.1", "-e", fullQuery)
		output3, err3 := cmd3.CombinedOutput()
		if err3 == nil {
			logger.Log("MariaDB Database Server : Active on port %s (Admin: `%s`)", port, Creds.Username)
			return nil
		}
		lastErr = fmt.Errorf("(-h 127.0.0.1): %s | (localhost): %s | (with pass): %s", strings.TrimSpace(string(output)), strings.TrimSpace(string(output2)), strings.TrimSpace(string(output3)))
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("failed to init system database after 10 attempts: %w", lastErr)
}

// TestAdminConnection verifies that MariaDB can be reached using the new admin credentials and vcopanel-db is accessible.
func TestAdminConnection(mariaDir string, port string) error {
	mariaDir = ResolveMariaRoot(mariaDir)
	mysqlExe := filepath.Join(mariaDir, "bin", "mysql.exe")
	if _, err := os.Stat(mysqlExe); err != nil {
		return fmt.Errorf("mysql binary not found at %s", mysqlExe)
	}

	logger.Log("Testing MariaDB connection with new admin credentials (`%s`)...", Creds.Username)

	var lastErr error
	for i := 0; i < 10; i++ {
		cmd := exec.Command(mysqlExe, "-u", Creds.Username, fmt.Sprintf("-p%s", Creds.Password), "-P", port, "-h", "127.0.0.1", "-e", "USE `vcopanel-db`; SELECT 1;")
		output, err := cmd.CombinedOutput()
		if err == nil {
			logger.Log("MariaDB Admin Connection : Verified successfully (`%s`@`127.0.0.1`)", Creds.Username)
			return nil
		}
		cmd2 := exec.Command(mysqlExe, "-u", Creds.Username, fmt.Sprintf("-p%s", Creds.Password), "-P", port, "-e", "USE `vcopanel-db`; SELECT 1;")
		output2, err2 := cmd2.CombinedOutput()
		if err2 == nil {
			logger.Log("MariaDB Admin Connection : Verified successfully (`%s`@`localhost`)", Creds.Username)
			return nil
		}
		lastErr = fmt.Errorf("(-h 127.0.0.1): %s | (localhost): %s", strings.TrimSpace(string(output)), strings.TrimSpace(string(output2)))
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("MariaDB admin connection failed after 10 attempts: %w", lastErr)
}

// TestServicePort verifies that a TCP service (like Redis, Mailpit, phpMyAdmin) is active and listening on the specified port.
func TestServicePort(serviceName string, port string) error {
	logger.Log("Testing %s TCP service on port %s...", serviceName, port)
	var lastErr error
	for i := 0; i < 10; i++ {
		conn, err := net.DialTimeout("tcp", "127.0.0.1:"+port, 1*time.Second)
		if err == nil {
			conn.Close()
			logger.Log("%s Service : Verified active and listening on port %s", serviceName, port)
			return nil
		}
		lastErr = err
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("%s service failed to answer on port %s after 10 attempts: %w", serviceName, port, lastErr)
}

// StartPHPMyAdmin launches phpMyAdmin built-in server on port 8881.
func StartPHPMyAdmin(workspaceDir string) error {
	pmaMu.Lock()
	defer pmaMu.Unlock()

	if pmaProcess != nil && pmaProcess.Process != nil {
		return nil
	}

	// If it is already running and reachable, do nothing
	conn, err := net.DialTimeout("tcp", "127.0.0.1:8881", 500*time.Millisecond)
	if err == nil {
		conn.Close()
		return nil
	}

	// Attempt to kill existing php.exe processes that might be orphaned
	exec.Command("taskkill", "/F", "/IM", "php.exe").Run()
	time.Sleep(200 * time.Millisecond)
	time.Sleep(200 * time.Millisecond)

	pmaDir := filepath.Join(workspaceDir, "shared-services", "phpmyadmin")
	if _, err := os.Stat(pmaDir); err != nil {
		pmaDir = filepath.Join(workspaceDir, "phpmyadmin")
		if _, err2 := os.Stat(pmaDir); err2 != nil {
			return fmt.Errorf("phpMyAdmin folder not found in workspace. Run Precheck first")
		}
	}

	// If index.php is inside a subfolder (e.g. phpMyAdmin-5.2.1-all-languages), use that folder as root
	if _, errIndex := os.Stat(filepath.Join(pmaDir, "index.php")); os.IsNotExist(errIndex) {
		entries, _ := os.ReadDir(pmaDir)
		for _, entry := range entries {
			if entry.IsDir() {
				subDir := filepath.Join(pmaDir, entry.Name())
				if _, errSub := os.Stat(filepath.Join(subDir, "index.php")); errSub == nil {
					pmaDir = subDir
					break
				}
			}
		}
	}

	// Ensure config.inc.php exists inside the true pmaDir root
	cfgPath := filepath.Join(pmaDir, "config.inc.php")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		cfgContent := `<?php
$cfg['blowfish_secret'] = 'MultiLaravelPortableManagerSecretKey2026!';
$i = 0;
$i++;
$cfg['Servers'][$i]['auth_type'] = 'cookie';
$cfg['Servers'][$i]['host'] = '127.0.0.1';
$cfg['Servers'][$i]['port'] = '3306';
$cfg['Servers'][$i]['compress'] = false;
$cfg['Servers'][$i]['AllowNoPassword'] = true;
$cfg['AllowThirdPartyFraming'] = true;
`
		os.WriteFile(cfgPath, []byte(cfgContent), 0644)
	}

	// Prioritize isolated PHP inside core directory
	var phpExe string
	coreCandidate := filepath.Join(workspaceDir, "core", "php", "php.exe")
	if _, err := os.Stat(coreCandidate); err == nil {
		phpExe = coreCandidate
	} else {
		// Fallback to any runtime PHP
		for _, ver := range []string{"php-8.3", "php-8.4", "php-8.5", "php-8.2"} {
			candidate := filepath.Join(workspaceDir, "runtimes", ver, "php.exe")
			if _, err := os.Stat(candidate); err == nil {
				phpExe = candidate
				break
			}
			candidateLegacy := filepath.Join(workspaceDir, ver, "php.exe")
			if _, err := os.Stat(candidateLegacy); err == nil {
				phpExe = candidateLegacy
				break
			}
		}
	}
	if phpExe == "" {
		return fmt.Errorf("no portable PHP binary found to serve phpMyAdmin")
	}

	extDir := filepath.Join(filepath.Dir(phpExe), "ext")
	iniPath := filepath.Join(filepath.Dir(phpExe), "php.ini")

	// Build a minimal dedicated php.ini for phpMyAdmin to ensure mysqli is always loaded
	pmaIniPath := filepath.Join(filepath.Dir(phpExe), "php-pma.ini")
	pmaIniContent := fmt.Sprintf(`;; Dedicated phpMyAdmin config (auto-generated, do not edit)
extension_dir = "%s"
extension=mysqli
extension=mbstring
extension=curl
extension=openssl
extension=pdo_mysql
extension=gd
extension=zip
date.timezone = "UTC"
display_errors = Off
`, extDir)
	_ = os.WriteFile(pmaIniPath, []byte(pmaIniContent), 0644)

	// Use dedicated pma ini if exists, otherwise fall back to standard php.ini
	activeIni := pmaIniPath
	if _, err := os.Stat(pmaIniPath); err != nil {
		activeIni = iniPath
	}

	cmd := exec.Command(phpExe, "-c", activeIni, "-S", "127.0.0.1:8881", "-t", pmaDir)
	cmd.Dir = pmaDir
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
	
	stdoutPipe, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return err
	}

	pid := cmd.Process.Pid
	go func() {
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			logger.LogProc(pid, "php.exe", "%s", scanner.Text())
		}
	}()
	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			logger.LogProc(pid, "php.exe", "%s", scanner.Text())
		}
	}()

	pmaProcess = cmd
	go func(c *exec.Cmd) {
		c.Wait()
		pmaMu.Lock()
		if pmaProcess == c {
			pmaProcess = nil
		}
		pmaMu.Unlock()
	}(cmd)

	logger.Log("phpMyAdmin Web Interface: Active on http://127.0.0.1:8881 (PID: %d)", cmd.Process.Pid)
	return nil
}

func StopPHPMyAdmin() {
	pmaMu.Lock()
	defer pmaMu.Unlock()
	if pmaProcess != nil && pmaProcess.Process != nil {
		exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", pmaProcess.Process.Pid)).Run()
		pmaProcess = nil
	}
	out, _ := exec.Command("netstat", "-aon").Output()
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, ":8881") && strings.Contains(line, "LISTENING") {
			parts := strings.Fields(line)
			if len(parts) >= 5 {
				exec.Command("taskkill", "/F", "/PID", parts[len(parts)-1]).Run()
			}
		}
	}
}

func IsPHPMyAdminRunning() bool {
	pmaMu.Lock()
	hasProcess := pmaProcess != nil && pmaProcess.Process != nil
	pmaMu.Unlock()
	if hasProcess {
		return true
	}
	conn, err := net.DialTimeout("tcp", "127.0.0.1:8881", 500*time.Millisecond)
	if err == nil {
		conn.Close()
		return true
	}
	return false
}

func IsMariaDBRunning() bool {
	cmd := exec.Command("tasklist", "/FI", "IMAGENAME eq mysqld.exe")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(out)), "mysqld.exe")
}

func StopMariaDB() {
	logger.Log("Terminating MariaDB service...")
	exec.Command("taskkill", "/F", "/IM", "mysqld.exe").Run()
}

// ServiceRecord represents a dynamically detected infrastructure asset stored in MariaDB
type ServiceRecord struct {
	Key      string `json:"service_key"`
	Name     string `json:"service_name"`
	Category string `json:"category"`
	Version  string `json:"version"`
	Path     string `json:"install_path"`
}

// SyncInfrastructureMetadata dynamically discovers installed core/shared/runtime assets
// and stores their version, path, and admin credentials directly into vcopanel-db.
func SyncInfrastructureMetadata(workspaceDir string, mariaRoot string, port string, dbUser string, dbPass string) error {
	mariaRoot = ResolveMariaRoot(mariaRoot)
	if dbUser == "" || dbPass == "" {
		confPath := filepath.Join(workspaceDir, "shared-services", "db-admin.json")
		if data, err := os.ReadFile(confPath); err == nil {
			var parsed map[string]interface{}
			if json.Unmarshal(data, &parsed) == nil {
				if u, ok := parsed["admin_user"].(string); ok { dbUser = u }
				if p, ok := parsed["admin_pass"].(string); ok { dbPass = p }
			}
		}
	}
	if dbUser == "" { dbUser = "vcopanel_admin" }

	mysqlExe := filepath.Join(mariaRoot, "bin", "mysql.exe")
	if _, err := os.Stat(mysqlExe); err != nil {
		return nil
	}

	// 1. Create dynamic metadata tables inside vcopanel-db
	schemaQueries := []string{
		"CREATE DATABASE IF NOT EXISTS `vcopanel-db` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;",
		"USE `vcopanel-db`;",
		"CREATE TABLE IF NOT EXISTS `system_infrastructure` (`service_key` VARCHAR(64) PRIMARY KEY, `service_name` VARCHAR(128) NOT NULL, `category` VARCHAR(64) NOT NULL, `version` VARCHAR(64) NOT NULL, `install_path` VARCHAR(512) NOT NULL, `updated_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;",
		"CREATE TABLE IF NOT EXISTS `database_credentials` (`config_key` VARCHAR(64) PRIMARY KEY, `host` VARCHAR(64) NOT NULL, `port` INT NOT NULL, `username` VARCHAR(64) NOT NULL, `password` VARCHAR(128) NOT NULL, `core_database` VARCHAR(64) NOT NULL, `updated_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;",
		"CREATE TABLE IF NOT EXISTS `offline_catalog` (`asset_key` VARCHAR(64) PRIMARY KEY, `asset_name` VARCHAR(128) NOT NULL, `category` VARCHAR(64) NOT NULL, `zip_path` VARCHAR(512) NOT NULL, `is_extracted` TINYINT(1) DEFAULT 0, `extracted_path` VARCHAR(512) DEFAULT '', `updated_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;",
		"CREATE TABLE IF NOT EXISTS `system_settings` (`setting_key` VARCHAR(64) PRIMARY KEY, `setting_value` VARCHAR(512) NOT NULL, `updated_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;",
		"CREATE TABLE IF NOT EXISTS `system_services` (`service_key` VARCHAR(64) PRIMARY KEY, `service_name` VARCHAR(128) NOT NULL, `port` INT NOT NULL, `url` VARCHAR(256) NOT NULL, `status` VARCHAR(32) NOT NULL, `updated_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;",
		
		// Projects Core Table
		"CREATE TABLE IF NOT EXISTS `projects` (`uuid` VARCHAR(36) UNIQUE, `path` VARCHAR(512) PRIMARY KEY, `name` VARCHAR(128) NOT NULL, `stack` VARCHAR(64) NOT NULL, `framework` VARCHAR(64) NOT NULL, `status` VARCHAR(64) NOT NULL DEFAULT 'Pending', `php_version` VARCHAR(64) DEFAULT '', `node_version` VARCHAR(64) DEFAULT '', `go_version` VARCHAR(64) DEFAULT '', `port` VARCHAR(32) DEFAULT '', `db_name` VARCHAR(128) DEFAULT '', `collation` VARCHAR(64) DEFAULT 'utf8mb4_unicode_ci', `isolation_mode` VARCHAR(32) DEFAULT 'standard', `updated_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;",
		"ALTER TABLE `projects` ADD COLUMN IF NOT EXISTS `go_version` VARCHAR(64) DEFAULT '';",
		"ALTER TABLE `projects` ADD COLUMN IF NOT EXISTS `uuid` VARCHAR(36) UNIQUE;",
		"ALTER TABLE `projects` ADD COLUMN IF NOT EXISTS `isolation_mode` VARCHAR(32) DEFAULT 'standard';",
		
		// Phase 12 Tables
		"CREATE TABLE IF NOT EXISTS `project_runtime_state` (`id` BIGINT AUTO_INCREMENT PRIMARY KEY, `project_uuid` VARCHAR(36) NOT NULL, `process_type` VARCHAR(32) NOT NULL, `pid` INT NOT NULL, `port` INT DEFAULT 0, `entrypoint` VARCHAR(256) DEFAULT '', `cpu_usage` FLOAT DEFAULT 0.0, `memory_bytes` BIGINT DEFAULT 0, `health_status` VARCHAR(32) DEFAULT 'healthy', `started_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP, `last_heartbeat` TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP, UNIQUE KEY `uk_project_process` (`project_uuid`, `process_type`), INDEX `idx_pid` (`pid`), INDEX `idx_port` (`port`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;",
		"CREATE TABLE IF NOT EXISTS `project_env_variables` (`project_uuid` VARCHAR(36) NOT NULL, `env_key` VARCHAR(128) NOT NULL, `env_value` TEXT NOT NULL, `is_secret` TINYINT(1) DEFAULT 0, `description` VARCHAR(256) DEFAULT '', `updated_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP, PRIMARY KEY (`project_uuid`, `env_key`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;",
		"CREATE TABLE IF NOT EXISTS `project_cron_jobs` (`id` VARCHAR(36) PRIMARY KEY, `project_uuid` VARCHAR(36) NOT NULL, `cron_expression` VARCHAR(64) NOT NULL, `command` VARCHAR(512) NOT NULL, `is_active` TINYINT(1) DEFAULT 1, `last_run_at` TIMESTAMP NULL DEFAULT NULL, `last_run_status` VARCHAR(32) DEFAULT 'pending', `last_output` TEXT DEFAULT NULL, `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP, INDEX `idx_active_cron` (`is_active`, `cron_expression`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;",
		"CREATE TABLE IF NOT EXISTS `project_dependencies` (`id` BIGINT AUTO_INCREMENT PRIMARY KEY, `project_uuid` VARCHAR(36) NOT NULL, `package_manager` VARCHAR(32) NOT NULL, `package_name` VARCHAR(128) NOT NULL, `installed_version` VARCHAR(64) NOT NULL, `is_dev_dependency` TINYINT(1) DEFAULT 0, `updated_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP, UNIQUE KEY `uk_project_package` (`project_uuid`, `package_manager`, `package_name`), INDEX `idx_package_name` (`package_name`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;",

		"INSERT IGNORE INTO `system_settings` (`setting_key`, `setting_value`) VALUES ('projects_directory', 'C:/Projects');",
	}

	fullSchemaQuery := strings.Join(schemaQueries, " ")
	exec.Command(mysqlExe, "-u", "root", "-P", port, "-h", "127.0.0.1", "-e", fullSchemaQuery).Run()

	// 2. Scan and build dynamic list of installed services
	var records []ServiceRecord

	cleanPath := func(p string) string {
		return filepath.ToSlash(filepath.Clean(p))
	}

	// Core: Go Compiler
	goDir := filepath.Join(workspaceDir, "core", "go")
	if info, err := os.Stat(goDir); err == nil && info.IsDir() {
		records = append(records, ServiceRecord{Key: "go", Name: "Go Portable Compiler", Category: "core", Version: "1.23.4", Path: cleanPath(goDir)})
	}

	// Core: PHP Engine
	phpDir := filepath.Join(workspaceDir, "core", "php")
	if info, err := os.Stat(phpDir); err == nil && info.IsDir() {
		records = append(records, ServiceRecord{Key: "core-php", Name: "PHP Core Daemon", Category: "core", Version: "8.x Portable", Path: cleanPath(phpDir)})
	}

	// Shared Service: MariaDB
	records = append(records, ServiceRecord{Key: "mariadb", Name: "MariaDB Database Engine", Category: "shared-service", Version: filepath.Base(mariaRoot), Path: cleanPath(mariaRoot)})

	// Shared Service: phpMyAdmin
	pmaDir := filepath.Join(workspaceDir, "shared-services", "phpmyadmin")
	if info, err := os.Stat(pmaDir); err == nil && info.IsDir() {
		records = append(records, ServiceRecord{Key: "phpmyadmin", Name: "phpMyAdmin Universal GUI", Category: "shared-service", Version: "5.2.1", Path: cleanPath(pmaDir)})
	} else if infoLegacy, err2 := os.Stat(filepath.Join(workspaceDir, "phpmyadmin")); err2 == nil && infoLegacy.IsDir() {
		records = append(records, ServiceRecord{Key: "phpmyadmin", Name: "phpMyAdmin Universal GUI", Category: "shared-service", Version: "5.2.1", Path: cleanPath(filepath.Join(workspaceDir, "phpmyadmin"))})
	}

	// Shared Service: Mailpit
	mailpitDir := filepath.Join(workspaceDir, "shared-services", "mailpit")
	if info, err := os.Stat(mailpitDir); err == nil && info.IsDir() {
		records = append(records, ServiceRecord{Key: "mailpit", Name: "Mailpit SMTP & Webmail Server", Category: "shared-service", Version: "v1.22.3", Path: cleanPath(mailpitDir)})
	} else if infoLegacy, err2 := os.Stat(filepath.Join(workspaceDir, "mailpit")); err2 == nil && infoLegacy.IsDir() {
		records = append(records, ServiceRecord{Key: "mailpit", Name: "Mailpit SMTP & Webmail Server", Category: "shared-service", Version: "v1.22.3", Path: cleanPath(filepath.Join(workspaceDir, "mailpit"))})
	}

	// Shared Service: Redis
	redisDir := filepath.Join(workspaceDir, "shared-services", "redis")
	if info, err := os.Stat(redisDir); err == nil && info.IsDir() {
		ver := "5.0.14.1"
		records = append(records, ServiceRecord{Key: "redis", Name: "Redis Portable Cache", Category: "shared-service", Version: ver, Path: cleanPath(redisDir)})
	} else if infoLegacy, err2 := os.Stat(filepath.Join(workspaceDir, "redis")); err2 == nil && infoLegacy.IsDir() {
		records = append(records, ServiceRecord{Key: "redis", Name: "Redis Portable Cache", Category: "shared-service", Version: "5.0.14.1", Path: cleanPath(filepath.Join(workspaceDir, "redis"))})
	}

	// Project Runtimes (scan workspace/runtimes)
	runtimesDir := filepath.Join(workspaceDir, "runtimes")
	if dirs, err := os.ReadDir(runtimesDir); err == nil {
		for _, d := range dirs {
			if !d.IsDir() { continue }
			subPath := filepath.Join(runtimesDir, d.Name())
			lower := strings.ToLower(d.Name())
			if strings.HasPrefix(lower, "php-") {
				ver := strings.TrimPrefix(lower, "php-")
				records = append(records, ServiceRecord{Key: lower, Name: "PHP Runtime " + ver, Category: "runtime", Version: ver, Path: cleanPath(subPath)})
			} else if strings.HasPrefix(lower, "node") {
				ver := strings.TrimPrefix(lower, "node-")
				records = append(records, ServiceRecord{Key: lower, Name: "Node.js Runtime " + ver, Category: "runtime", Version: ver, Path: cleanPath(subPath)})
			} else if strings.HasPrefix(lower, "composer") {
				records = append(records, ServiceRecord{Key: "composer", Name: "Composer Package Manager", Category: "runtime", Version: "2.8", Path: cleanPath(subPath)})
			} else if strings.HasPrefix(lower, "go-") {
				ver := strings.TrimPrefix(lower, "go-")
				records = append(records, ServiceRecord{Key: lower, Name: "Go Portable Runtime " + ver, Category: "runtime", Version: ver, Path: cleanPath(subPath)})
			} else if lower == "go" {
				records = append(records, ServiceRecord{Key: "go-runtime", Name: "Go Portable Runtime", Category: "runtime", Version: "1.26.4", Path: cleanPath(subPath)})
			}
		}
	}

	vcredistFlag := filepath.Join(workspaceDir, "core", "vcredist_installed.flag")
	if _, err := os.Stat(vcredistFlag); err == nil {
		records = append(records, ServiceRecord{Key: "vcredist", Name: "Visual C++ 2015-2022 Redistributable", Category: "core", Version: "14.40.33810", Path: cleanPath(filepath.Join(workspaceDir, "core"))})
	}

	// 3. Execute REPLACE INTO queries to save dynamic records
	var insertQueries []string
	insertQueries = append(insertQueries, "USE `vcopanel-db`;")
	for _, rec := range records {
		q := fmt.Sprintf("REPLACE INTO `system_infrastructure` (`service_key`, `service_name`, `category`, `version`, `install_path`) VALUES ('%s', '%s', '%s', '%s', '%s');",
			rec.Key, rec.Name, rec.Category, rec.Version, rec.Path)
		insertQueries = append(insertQueries, q)
	}

	credQuery := fmt.Sprintf("REPLACE INTO `database_credentials` (`config_key`, `host`, `port`, `username`, `password`, `core_database`) VALUES ('db_admin', '127.0.0.1', %s, '%s', '%s', 'vcopanel-db');",
		port, dbUser, dbPass)
	insertQueries = append(insertQueries, credQuery)

	fullInsertQuery := strings.Join(insertQueries, " ")
	exec.Command(mysqlExe, "-u", "root", "-P", port, "-h", "127.0.0.1", "-e", fullInsertQuery).Run()

	assetsDir := filepath.Join(filepath.Dir(workspaceDir), "pc-assets")
	SyncOfflineCatalog(workspaceDir, assetsDir, mariaRoot, port)

	logger.Log("Dynamic Infrastructure : All service paths and versions synced to database `vcopanel-db`.")
	return nil
}

type SystemServiceRecord struct {
	Key    string `json:"service_key"`
	Name   string `json:"service_name"`
	Port   int    `json:"port"`
	URL    string `json:"url"`
	Status string `json:"status"`
}

func SaveServiceState(mariaRoot, dbPort, key, name string, port int, url, status string) error {
	mariaRoot = ResolveMariaRoot(mariaRoot)
	mysqlExe := filepath.Join(mariaRoot, "bin", "mysql.exe")
	if _, err := os.Stat(mysqlExe); err != nil {
		return err
	}
	query := fmt.Sprintf("USE `vcopanel-db`; REPLACE INTO `system_services` (`service_key`, `service_name`, `port`, `url`, `status`) VALUES ('%s', '%s', %d, '%s', '%s');",
		key, name, port, url, status)
	return exec.Command(mysqlExe, "-u", "root", "-P", dbPort, "-h", "127.0.0.1", "-e", query).Run()
}

func FetchSystemServices(mariaRoot string, port string) ([]SystemServiceRecord, error) {
	mariaRoot = ResolveMariaRoot(mariaRoot)
	mysqlExe := filepath.Join(mariaRoot, "bin", "mysql.exe")
	cmd := exec.Command(mysqlExe, "-u", "root", "-P", port, "-h", "127.0.0.1", "-D", "vcopanel-db", "-sN", "-e", "SELECT service_key, service_name, port, url, status FROM system_services;")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var res []SystemServiceRecord
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, l := range lines {
		if strings.TrimSpace(l) == "" { continue }
		parts := strings.Split(l, "\t")
		if len(parts) >= 5 {
			pVal, _ := strconv.Atoi(parts[2])
			res = append(res, SystemServiceRecord{
				Key:    parts[0],
				Name:   parts[1],
				Port:   pVal,
				URL:    parts[3],
				Status: parts[4],
			})
		}
	}
	return res, nil
}

// SyncOfflineCatalog scans pc-assets/ and syncs all offline archive status to offline_catalog table
func SyncOfflineCatalog(workspaceDir string, assetsDir string, mariaRoot string, port string) {
	mariaRoot = ResolveMariaRoot(mariaRoot)
	mysqlExe := filepath.Join(mariaRoot, "bin", "mysql.exe")
	if _, err := os.Stat(mysqlExe); err != nil {
		return
	}

	cleanPath := func(p string) string { return filepath.ToSlash(filepath.Clean(p)) }
	var queries []string
	queries = append(queries, "USE `vcopanel-db`;")

	processFile := func(path string, name string, cat string) {
		key := strings.TrimSuffix(name, filepath.Ext(name))
		isExtracted := 0
		extractedPath := ""

		lower := strings.ToLower(name)
		if strings.HasPrefix(lower, "mariadb") {
			target := filepath.Join(workspaceDir, "shared-services", "mariadb")
			if info, err := os.Stat(target); err == nil && info.IsDir() { isExtracted = 1; extractedPath = cleanPath(target) }
		} else if strings.HasPrefix(lower, "redis") {
			target := filepath.Join(workspaceDir, "shared-services", "redis")
			if info, err := os.Stat(target); err == nil && info.IsDir() { isExtracted = 1; extractedPath = cleanPath(target) }
		} else if strings.HasPrefix(lower, "phpmyadmin") {
			target := filepath.Join(workspaceDir, "shared-services", "phpmyadmin")
			if info, err := os.Stat(target); err == nil && info.IsDir() { isExtracted = 1; extractedPath = cleanPath(target) }
		} else if strings.HasPrefix(lower, "mailpit") {
			target := filepath.Join(workspaceDir, "shared-services", "mailpit")
			if info, err := os.Stat(target); err == nil && info.IsDir() { isExtracted = 1; extractedPath = cleanPath(target) }
		} else if strings.HasPrefix(lower, "composer") {
			target := filepath.Join(workspaceDir, "runtimes", "composer")
			if info, err := os.Stat(target); err == nil && info.IsDir() { isExtracted = 1; extractedPath = cleanPath(target) }
		} else if strings.HasSuffix(lower, ".exe") && strings.Contains(lower, "redist") {
			target := filepath.Join(workspaceDir, "core", "vcredist_installed.flag")
			if _, err := os.Stat(target); err == nil { isExtracted = 1; extractedPath = cleanPath(filepath.Join(workspaceDir, "core")) }
		} else if lower == "icon.ico" {
			target := filepath.Join(workspaceDir, "pc-assets", "icon.ico")
			if _, err := os.Stat(target); err == nil { isExtracted = 1; extractedPath = cleanPath(target) }
		} else {
			subDir := strings.TrimSuffix(name, ".zip")
			subDir = strings.TrimSuffix(subDir, "-win64")
			
			if strings.HasPrefix(lower, "node-v") {
				parts := strings.Split(subDir, ".")
				if len(parts) > 0 {
					subDir = parts[0]
				}
			}
			if strings.HasPrefix(lower, "php-") {
				parts := strings.Split(subDir, ".")
				if len(parts) >= 2 {
					subDir = parts[0] + "." + parts[1]
				}
			}
			
			target := filepath.Join(workspaceDir, "runtimes", subDir)
			if info, err := os.Stat(target); err == nil && info.IsDir() { isExtracted = 1; extractedPath = cleanPath(target) }
		}

		q := fmt.Sprintf("REPLACE INTO `offline_catalog` (`asset_key`, `asset_name`, `category`, `zip_path`, `is_extracted`, `extracted_path`) VALUES ('%s', '%s', '%s', '%s', %d, '%s');",
			key, name, cat, cleanPath(path), isExtracted, extractedPath)
		queries = append(queries, q)
	}

	if entries, err := os.ReadDir(assetsDir); err == nil {
		for _, e := range entries {
			if e.IsDir() && e.Name() != "Wallpapers" && e.Name() != "Stack-Logo" {
				catDir := filepath.Join(assetsDir, e.Name())
				if sub, errSub := os.ReadDir(catDir); errSub == nil {
					for _, sf := range sub {
						if !sf.IsDir() { processFile(filepath.Join(catDir, sf.Name()), sf.Name(), e.Name()) }
					}
				}
			} else if !e.IsDir() {
				processFile(filepath.Join(assetsDir, e.Name()), e.Name(), "general")
			}
		}
	}

	if len(queries) > 1 {
		fullQuery := strings.Join(queries, " ")
		exec.Command(mysqlExe, "-u", "root", "-P", port, "-h", "127.0.0.1", "-e", fullQuery).Run()
	}
}

type OfflineAsset struct {
	AssetKey    string
	AssetName   string
	IsExtracted bool
}

// FetchOfflineCatalog queries the dynamic offline catalog status from MariaDB
func FetchOfflineCatalog(mariaRoot string, port string) ([]OfflineAsset, error) {
	mariaRoot = ResolveMariaRoot(mariaRoot)
	mysqlExe := filepath.Join(mariaRoot, "bin", "mysql.exe")
	cmd := exec.Command(mysqlExe, "-u", "root", "-P", port, "-h", "127.0.0.1", "-D", "vcopanel-db", "-sN", "-e", "SELECT asset_key, asset_name, is_extracted FROM offline_catalog;")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var res []OfflineAsset
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, l := range lines {
		if strings.TrimSpace(l) == "" { continue }
		parts := strings.Split(l, "\t")
		if len(parts) >= 3 {
			extracted := false
			if parts[2] == "1" { extracted = true }
			res = append(res, OfflineAsset{AssetKey: parts[0], AssetName: parts[1], IsExtracted: extracted})
		}
	}
	return res, nil
}

// FetchInfrastructureRecords queries the dynamic system metadata from MariaDB
func FetchInfrastructureRecords(mariaRoot string, port string) ([]ServiceRecord, error) {
	mariaRoot = ResolveMariaRoot(mariaRoot)
	mysqlExe := filepath.Join(mariaRoot, "bin", "mysql.exe")
	cmd := exec.Command(mysqlExe, "-u", "root", "-P", port, "-h", "127.0.0.1", "-D", "vcopanel-db", "-sN", "-e", "SELECT service_key, service_name, category, version, install_path FROM system_infrastructure;")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var res []ServiceRecord
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, l := range lines {
		if strings.TrimSpace(l) == "" { continue }
		parts := strings.Split(l, "\t")
		if len(parts) >= 5 {
			res = append(res, ServiceRecord{
				Key:      parts[0],
				Name:     parts[1],
				Category: parts[2],
				Version:  parts[3],
				Path:     parts[4],
			})
		}
	}
	return res, nil
}

// GetProjectsDirectory reads the global projects workspace path from vcopanel-db system_settings. Defaults to C:/Projects.
func GetProjectsDirectory(mariaRoot, port string) string {
	mariaRoot = ResolveMariaRoot(mariaRoot)
	mysqlExe := filepath.Join(mariaRoot, "bin", "mysql.exe")
	if _, err := os.Stat(mysqlExe); err != nil {
		return "C:/Projects"
	}
	query := "USE `vcopanel-db`; SELECT `setting_value` FROM `system_settings` WHERE `setting_key`='projects_directory';"
	cmd := exec.Command(mysqlExe, "-u", "root", "-P", port, "-h", "127.0.0.1", "-N", "-s", "-e", query)
	out, err := cmd.Output()
	if err == nil {
		val := strings.TrimSpace(string(out))
		if val != "" {
			return filepath.ToSlash(val)
		}
	}
	return "C:/Projects"
}

// SetProjectsDirectory saves the global projects workspace path into vcopanel-db system_settings.
func SetProjectsDirectory(mariaRoot, port, path string) error {
	mariaRoot = ResolveMariaRoot(mariaRoot)
	mysqlExe := filepath.Join(mariaRoot, "bin", "mysql.exe")
	cleanP := filepath.ToSlash(filepath.Clean(path))
	query := fmt.Sprintf("USE `vcopanel-db`; CREATE TABLE IF NOT EXISTS `system_settings` (`setting_key` VARCHAR(64) PRIMARY KEY, `setting_value` VARCHAR(512) NOT NULL, `updated_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci; REPLACE INTO `system_settings` (`setting_key`, `setting_value`) VALUES ('projects_directory', '%s');", cleanP)
	cmd := exec.Command(mysqlExe, "-u", "root", "-P", port, "-h", "127.0.0.1", "-e", query)
	return cmd.Run()
}

// CreateProjectDatabase creates a MariaDB schema for a project with the chosen collation.
func CreateProjectDatabase(mariaRoot, port, dbName, collation string) error {
	mariaRoot = ResolveMariaRoot(mariaRoot)
	mysqlExe := filepath.Join(mariaRoot, "bin", "mysql.exe")
	if collation == "" {
		collation = "utf8mb4_unicode_ci"
	}
	charSet := "utf8mb4"
	if strings.HasPrefix(collation, "utf8_") {
		charSet = "utf8"
	}
	query := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET %s COLLATE %s;", dbName, charSet, collation)
	cmd := exec.Command(mysqlExe, "-u", "root", "-P", port, "-h", "127.0.0.1", "-e", query)
	return cmd.Run()
}

// DBProjectRecord represents a project row in vcopanel-db.projects table.
type DBProjectRecord struct {
	UUID          string `json:"uuid"`
	Path          string `json:"path"`
	Name          string `json:"name"`
	Stack         string `json:"stack"`
	Framework     string `json:"framework"`
	Status        string `json:"status"`
	PHPVersion    string `json:"php_version"`
	NodeVersion   string `json:"node_version"`
	GoVersion     string `json:"go_version"`
	Port          string `json:"port"`
	DBName        string `json:"db_name"`
	Collation     string `json:"collation"`
	IsolationMode string `json:"isolation_mode"`
}

// SaveProjectToDB inserts or updates a project record in vcopanel-db.projects table.
func SaveProjectToDB(mariaRoot, dbPort, uuid, path, name, stack, framework, status, phpVer, nodeVer, goVer, projPort, dbName, collation, isolationMode string) error {
	mariaRoot = ResolveMariaRoot(mariaRoot)
	mysqlExe := filepath.Join(mariaRoot, "bin", "mysql.exe")
	if _, err := os.Stat(mysqlExe); err != nil {
		return err
	}
	cleanP := filepath.ToSlash(filepath.Clean(path))
	if isolationMode == "" {
		isolationMode = "standard"
	}
	query := fmt.Sprintf("USE `vcopanel-db`; REPLACE INTO `projects` (`uuid`, `path`, `name`, `stack`, `framework`, `status`, `php_version`, `node_version`, `go_version`, `port`, `db_name`, `collation`, `isolation_mode`) VALUES ('%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s');",
		uuid, cleanP, name, stack, framework, status, phpVer, nodeVer, goVer, projPort, dbName, collation, isolationMode)
	return exec.Command(mysqlExe, "-u", "root", "-P", dbPort, "-h", "127.0.0.1", "-e", query).Run()
}

// FetchAllProjectsFromDB retrieves all project records stored in vcopanel-db.projects table.
func FetchAllProjectsFromDB(mariaRoot, dbPort string) ([]DBProjectRecord, error) {
	mariaRoot = ResolveMariaRoot(mariaRoot)
	mysqlExe := filepath.Join(mariaRoot, "bin", "mysql.exe")
	if _, err := os.Stat(mysqlExe); err != nil {
		return nil, err
	}
	query := "USE `vcopanel-db`; SELECT uuid, path, name, stack, framework, status, php_version, node_version, go_version, port, db_name, collation, isolation_mode FROM projects;"
	cmd := exec.Command(mysqlExe, "-u", "root", "-P", dbPort, "-h", "127.0.0.1", "-sN", "-e", query)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var res []DBProjectRecord
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		parts := strings.Split(l, "\t")
		if len(parts) >= 11 {
			rec := DBProjectRecord{
				UUID:          parts[0],
				Path:          parts[1],
				Name:          parts[2],
				Stack:         parts[3],
				Framework:     parts[4],
				Status:        parts[5],
				PHPVersion:    parts[6],
				NodeVersion:   parts[7],
				GoVersion:     parts[8],
				Port:          parts[9],
				DBName:        parts[10],
			}
			if len(parts) >= 12 {
				rec.Collation = parts[11]
			}
			if len(parts) >= 13 {
				rec.IsolationMode = parts[12]
			}
			res = append(res, rec)
		} else if len(parts) >= 10 { // Fallback for old schema
			rec := DBProjectRecord{
				Path:          parts[0],
				Name:          parts[1],
				Stack:         parts[2],
				Framework:     parts[3],
				Status:        parts[4],
				PHPVersion:    parts[5],
				NodeVersion:   parts[6],
				GoVersion:     parts[7],
				Port:          parts[8],
				DBName:        parts[9],
			}
			res = append(res, rec)
		}
	}
	return res, nil
}

func DeleteProjectFromDB(mariaRoot, dbPort, path string) error {
	mariaRoot = ResolveMariaRoot(mariaRoot)
	mysqlExe := filepath.Join(mariaRoot, "bin", "mysql.exe")
	if _, err := os.Stat(mysqlExe); err != nil {
		return err
	}
	cleanP := filepath.ToSlash(filepath.Clean(path))
	winP := filepath.FromSlash(cleanP)
	
	// First extract UUID
	cmd := exec.Command(mysqlExe, "-u", "root", "-P", dbPort, "-h", "127.0.0.1", "-e", fmt.Sprintf("USE `vcopanel-db`; SELECT uuid FROM projects WHERE path='%s' OR path='%s' LIMIT 1;", cleanP, winP), "-sN")
	out, _ := cmd.Output()
	uuid := strings.TrimSpace(string(out))

	var queries []string
	queries = append(queries, "USE `vcopanel-db`;")
	
	if uuid != "" {
		queries = append(queries, fmt.Sprintf("DELETE FROM `projects` WHERE `uuid`='%s' OR `path`='%s' OR `path`='%s';", uuid, cleanP, winP))
		queries = append(queries, fmt.Sprintf("DELETE FROM `project_runtime_state` WHERE `project_uuid`='%s';", uuid))
		queries = append(queries, fmt.Sprintf("DELETE FROM `project_env_variables` WHERE `project_uuid`='%s';", uuid))
		queries = append(queries, fmt.Sprintf("DELETE FROM `project_cron_jobs` WHERE `project_uuid`='%s';", uuid))
		queries = append(queries, fmt.Sprintf("DELETE FROM `project_dependencies` WHERE `project_uuid`='%s';", uuid))
	} else {
		queries = append(queries, fmt.Sprintf("DELETE FROM `projects` WHERE `path`='%s' OR `path`='%s';", cleanP, winP))
	}

	return exec.Command(mysqlExe, "-u", "root", "-P", dbPort, "-h", "127.0.0.1", "-e", strings.Join(queries, " ")).Run()
}

// ClearPendingProjectsFromDB removes all pending or unconfigured projects from the database.
func ClearPendingProjectsFromDB(mariaRoot, dbPort string) error {
	mariaRoot = ResolveMariaRoot(mariaRoot)
	mysqlExe := filepath.Join(mariaRoot, "bin", "mysql.exe")
	if _, err := os.Stat(mysqlExe); err != nil {
		return err
	}
	
	query := "USE `vcopanel-db`; DELETE FROM `projects` WHERE `status` IN ('Pending', 'Unconfigured', 'Ejected');"
	return exec.Command(mysqlExe, "-u", "root", "-P", dbPort, "-h", "127.0.0.1", "-e", query).Run()
}

// ClearAllProjectsFromDB removes all projects and their related metadata from the database.
func ClearAllProjectsFromDB(mariaRoot, dbPort string) error {
	mariaRoot = ResolveMariaRoot(mariaRoot)
	mysqlExe := filepath.Join(mariaRoot, "bin", "mysql.exe")
	if _, err := os.Stat(mysqlExe); err != nil {
		return err
	}
	
	queries := []string{
		"USE `vcopanel-db`;",
		"DELETE FROM `projects`;",
		"DELETE FROM `project_runtime_state`;",
		"DELETE FROM `project_env_variables`;",
		"DELETE FROM `project_cron_jobs`;",
		"DELETE FROM `project_dependencies`;",
	}
	return exec.Command(mysqlExe, "-u", "root", "-P", dbPort, "-h", "127.0.0.1", "-e", strings.Join(queries, " ")).Run()
}

// DeleteProjectAndDatabase completely removes a project record from all tables and drops its database schema.
func DeleteProjectAndDatabase(mariaRoot, dbPort, path, dbName, uuid string) error {
	mariaRoot = ResolveMariaRoot(mariaRoot)
	mysqlExe := filepath.Join(mariaRoot, "bin", "mysql.exe")
	if _, err := os.Stat(mysqlExe); err != nil {
		return err
	}
	cleanP := filepath.ToSlash(filepath.Clean(path))
	winP := filepath.FromSlash(cleanP)

	var queries []string
	queries = append(queries, "USE `vcopanel-db`;")
	if uuid != "" {
		queries = append(queries, fmt.Sprintf("DELETE FROM `projects` WHERE `uuid`='%s' OR `path`='%s' OR `path`='%s';", uuid, cleanP, winP))
		queries = append(queries, fmt.Sprintf("DELETE FROM `project_runtime_state` WHERE `project_uuid`='%s';", uuid))
		queries = append(queries, fmt.Sprintf("DELETE FROM `project_env_variables` WHERE `project_uuid`='%s';", uuid))
		queries = append(queries, fmt.Sprintf("DELETE FROM `project_cron_jobs` WHERE `project_uuid`='%s';", uuid))
		queries = append(queries, fmt.Sprintf("DELETE FROM `project_dependencies` WHERE `project_uuid`='%s';", uuid))
	} else {
		queries = append(queries, fmt.Sprintf("DELETE FROM `projects` WHERE `path`='%s' OR `path`='%s';", cleanP, winP))
	}

	if dbName != "" {
		queries = append(queries, fmt.Sprintf("DROP DATABASE IF EXISTS `%s`;", dbName))
	} else {
		baseName := filepath.Base(cleanP)
		cleanBase := ""
		for _, r := range baseName {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
				cleanBase += string(r)
			} else {
				cleanBase += "_"
			}
		}
		expectedDb := "app_" + strings.ToLower(cleanBase)
		queries = append(queries, fmt.Sprintf("DROP DATABASE IF EXISTS `%s`;", expectedDb))
	}

	fullQuery := strings.Join(queries, " ")
	return exec.Command(mysqlExe, "-u", "root", "-P", dbPort, "-h", "127.0.0.1", "-e", fullQuery).Run()
}

// EjectProjectFromDB drops the database schema and resets the project record to Pending state.
func EjectProjectFromDB(mariaRoot, dbPort, path, dbName, uuid string) error {
	mariaRoot = ResolveMariaRoot(mariaRoot)
	mysqlExe := filepath.Join(mariaRoot, "bin", "mysql.exe")
	if _, err := os.Stat(mysqlExe); err != nil {
		return err
	}
	cleanP := filepath.ToSlash(filepath.Clean(path))
	winP := filepath.FromSlash(cleanP)

	var queries []string
	queries = append(queries, "USE `vcopanel-db`;")
	if uuid != "" {
		queries = append(queries, fmt.Sprintf("DELETE FROM `projects` WHERE `uuid`='%s' OR `path`='%s' OR `path`='%s';", uuid, cleanP, winP))
		queries = append(queries, fmt.Sprintf("DELETE FROM `project_runtime_state` WHERE `project_uuid`='%s';", uuid))
		queries = append(queries, fmt.Sprintf("DELETE FROM `project_env_variables` WHERE `project_uuid`='%s';", uuid))
		queries = append(queries, fmt.Sprintf("DELETE FROM `project_cron_jobs` WHERE `project_uuid`='%s';", uuid))
		queries = append(queries, fmt.Sprintf("DELETE FROM `project_dependencies` WHERE `project_uuid`='%s';", uuid))
	} else {
		queries = append(queries, fmt.Sprintf("DELETE FROM `projects` WHERE `path`='%s' OR `path`='%s';", cleanP, winP))
	}

	if dbName != "" {
		queries = append(queries, fmt.Sprintf("DROP DATABASE IF EXISTS `%s`;", dbName))
	} else {
		baseName := filepath.Base(cleanP)
		cleanBase := ""
		for _, r := range baseName {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
				cleanBase += string(r)
			} else {
				cleanBase += "_"
			}
		}
		expectedDb := "app_" + strings.ToLower(cleanBase)
		queries = append(queries, fmt.Sprintf("DROP DATABASE IF EXISTS `%s`;", expectedDb))
	}

	fullQuery := strings.Join(queries, " ")
	return exec.Command(mysqlExe, "-u", "root", "-P", dbPort, "-h", "127.0.0.1", "-e", fullQuery).Run()
}

// InstalledRuntime represents a verified runtime from system_infrastructure.
type InstalledRuntime struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Version  string `json:"version"`
	Category string `json:"category"`
	Path     string `json:"path"`
}

// FetchInstalledRuntimes returns runtimes and shared-services from system_infrastructure
// that have a verified physical presence on disk.
func FetchInstalledRuntimes(mariaRoot, port string) ([]InstalledRuntime, error) {
	mariaRoot = ResolveMariaRoot(mariaRoot)
	records, err := FetchInfrastructureRecords(mariaRoot, port)
	if err != nil {
		return nil, err
	}
	var result []InstalledRuntime
	for _, r := range records {
		// Only include runtimes (not go core, not shared services) that exist on disk
		if r.Category != "runtime" {
			continue
		}
		if _, err := os.Stat(r.Path); err != nil {
			continue // Physical path missing — skip
		}
		result = append(result, InstalledRuntime{
			Key:      r.Key,
			Name:     r.Name,
			Version:  r.Version,
			Category: r.Category,
			Path:     r.Path,
		})
	}
	return result, nil
}

// UpdateMariaDBConfig updates tuning parameters in my.ini or creates it if missing.
func UpdateMariaDBConfig(workspaceDir, bufferPool, maxConn, charset, collation string) error {
	baseDir := filepath.Join(workspaceDir, "shared-services", "mariadb")
	if _, err := os.Stat(baseDir); err != nil {
		baseDir = filepath.Join(workspaceDir, "mariadb")
	}
	os.MkdirAll(baseDir, 0755)
	iniPath := filepath.Join(baseDir, "my.ini")
	if _, err := os.Stat(iniPath); os.IsNotExist(err) {
		iniPath = filepath.Join(baseDir, "my.cnf")
	}
	var lines []string
	if content, err := os.ReadFile(iniPath); err == nil {
		lines = strings.Split(string(content), "\n")
	} else {
		iniPath = filepath.Join(baseDir, "my.ini")
		lines = []string{
			"[mysqld]",
			"port=3306",
			"bind-address=127.0.0.1",
		}
	}

	foundBuf, foundConn, foundChar, foundColl := false, false, false, false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "innodb_buffer_pool_size") && strings.Contains(trimmed, "=") {
			lines[i] = fmt.Sprintf("innodb_buffer_pool_size=%s", bufferPool)
			foundBuf = true
		} else if strings.HasPrefix(trimmed, "max_connections") && strings.Contains(trimmed, "=") {
			lines[i] = fmt.Sprintf("max_connections=%s", maxConn)
			foundConn = true
		} else if strings.HasPrefix(trimmed, "character-set-server") && strings.Contains(trimmed, "=") {
			lines[i] = fmt.Sprintf("character-set-server=%s", charset)
			foundChar = true
		} else if strings.HasPrefix(trimmed, "collation-server") && strings.Contains(trimmed, "=") {
			lines[i] = fmt.Sprintf("collation-server=%s", collation)
			foundColl = true
		}
	}

	if !foundBuf {
		lines = append(lines, fmt.Sprintf("innodb_buffer_pool_size=%s", bufferPool))
	}
	if !foundConn {
		lines = append(lines, fmt.Sprintf("max_connections=%s", maxConn))
	}
	if !foundChar {
		lines = append(lines, fmt.Sprintf("character-set-server=%s", charset))
	}
	if !foundColl {
		lines = append(lines, fmt.Sprintf("collation-server=%s", collation))
	}

	logger.Log("Updated MariaDB config | Buffer: %s | Max Conn: %s | Charset: %s", bufferPool, maxConn, charset)
	return os.WriteFile(iniPath, []byte(strings.Join(lines, "\n")+"\n"), 0644)
}

func GetMariaDBConfig(workspaceDir string) map[string]string {
	baseDir := filepath.Join(workspaceDir, "shared-services", "mariadb")
	if _, err := os.Stat(baseDir); err != nil {
		baseDir = filepath.Join(workspaceDir, "mariadb")
	}
	iniPath := filepath.Join(baseDir, "my.ini")
	if _, err := os.Stat(iniPath); os.IsNotExist(err) {
		iniPath = filepath.Join(baseDir, "my.cnf")
	}
	res := map[string]string{
		"buffer_pool":     "512M",
		"max_connections": "250",
		"charset":         "utf8mb4",
		"collation":       "utf8mb4_unicode_ci",
	}
	if content, err := os.ReadFile(iniPath); err == nil {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "innodb_buffer_pool_size") && strings.Contains(trimmed, "=") {
				parts := strings.SplitN(trimmed, "=", 2)
				if len(parts) == 2 { res["buffer_pool"] = strings.TrimSpace(parts[1]) }
			} else if strings.HasPrefix(trimmed, "max_connections") && strings.Contains(trimmed, "=") {
				parts := strings.SplitN(trimmed, "=", 2)
				if len(parts) == 2 { res["max_connections"] = strings.TrimSpace(parts[1]) }
			} else if strings.HasPrefix(trimmed, "character-set-server") && strings.Contains(trimmed, "=") {
				parts := strings.SplitN(trimmed, "=", 2)
				if len(parts) == 2 { res["charset"] = strings.TrimSpace(parts[1]) }
			} else if strings.HasPrefix(trimmed, "collation-server") && strings.Contains(trimmed, "=") {
				parts := strings.SplitN(trimmed, "=", 2)
				if len(parts) == 2 { res["collation"] = strings.TrimSpace(parts[1]) }
			}
		}
	}
	return res
}

// ExecuteDynamicQuery runs a generic raw query on vcopanel-db.
func ExecuteDynamicQuery(mariaRoot, dbPort, query string) error {
	mariaRoot = ResolveMariaRoot(mariaRoot)
	mysqlExe := filepath.Join(mariaRoot, "bin", "mysql.exe")
	if _, err := os.Stat(mysqlExe); err != nil {
		return err
	}
	return exec.Command(mysqlExe, "-u", "root", "-P", dbPort, "-h", "127.0.0.1", "-e", "USE `vcopanel-db`; " + query).Run()
}

type DBProjectCron struct {
	ID             string `json:"id"`
	ProjectUUID    string `json:"project_uuid"`
	CronExpression string `json:"cron_expression"`
	Command        string `json:"command"`
	IsActive       int    `json:"is_active"`
	LastRunAt      string `json:"last_run_at"`
	LastRunStatus  string `json:"last_run_status"`
	LastOutput     string `json:"last_output"`
	CreatedAt      string `json:"created_at"`
}

func FetchCronJobs(mariaRoot, dbPort, projectUUID string) ([]DBProjectCron, error) {
	mariaRoot = ResolveMariaRoot(mariaRoot)
	mysqlExe := filepath.Join(mariaRoot, "bin", "mysql.exe")
	var result []DBProjectCron
	if _, err := os.Stat(mysqlExe); err != nil {
		return result, err
	}
	
	query := fmt.Sprintf("SELECT id, project_uuid, cron_expression, command, is_active, IFNULL(last_run_at, ''), last_run_status, IFNULL(last_output, ''), created_at FROM `vcopanel-db`.`project_cron_jobs` WHERE project_uuid='%s'", projectUUID)
	cmd := exec.Command(mysqlExe, "-u", "root", "-P", dbPort, "-h", "127.0.0.1", "-e", query, "-B", "-N")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return result, err
	}
	
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) >= 9 {
			isActive, _ := strconv.Atoi(parts[4])
			result = append(result, DBProjectCron{
				ID:             parts[0],
				ProjectUUID:    parts[1],
				CronExpression: parts[2],
				Command:        parts[3],
				IsActive:       isActive,
				LastRunAt:      parts[5],
				LastRunStatus:  parts[6],
				LastOutput:     parts[7],
				CreatedAt:      parts[8],
			})
		}
	}
	return result, nil
}
