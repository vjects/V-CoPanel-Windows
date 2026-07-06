package provision

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"vcopanel-bridge/internal/database"
	"vcopanel-bridge/internal/logger"
)

// ProvisionProject backs up .env, injects local environments/Mailpit/MariaDB settings, and creates project database.
func ProvisionProject(projectPath, phpVersion, port, mariaRoot, collation string) error {
	envPath := filepath.Join(projectPath, ".env")
	backupPath := filepath.Join(projectPath, ".env.backup-vcopanel")

	isGo := false
	if _, err := os.Stat(filepath.Join(projectPath, "go.mod")); err == nil {
		isGo = true
	} else if _, err := os.Stat(filepath.Join(projectPath, "main.go")); err == nil {
		if _, errArt := os.Stat(filepath.Join(projectPath, "artisan")); errArt != nil {
			isGo = true
		}
	}

	// Step 1: Secure backup of existing .env
	if content, err := os.ReadFile(envPath); err == nil {
		if _, errBkp := os.Stat(backupPath); os.IsNotExist(errBkp) {
			os.WriteFile(backupPath, content, 0644)
			logger.Log("Backup created at: %s", backupPath)
		}
	} else {
		examplePath := filepath.Join(projectPath, ".env.example")
		if exampleContent, errEx := os.ReadFile(examplePath); errEx == nil {
			os.WriteFile(envPath, exampleContent, 0644)
		} else if isGo {
			os.WriteFile(envPath, []byte("APP_NAME=GoApp\nPORT="+port+"\n"), 0644)
		} else {
			os.WriteFile(envPath, []byte("APP_NAME=Laravel\nAPP_KEY=\n"), 0644)
		}
	}

	// Step 2: Generate dedicated database name from folder basename
	projectName := filepath.Base(projectPath)
	reg := regexp.MustCompile("[^a-zA-Z0-9_]+")
	cleanName := reg.ReplaceAllString(projectName, "_")
	dbName := strings.ToLower("app_" + cleanName)

	if collation == "" {
		collation = "utf8mb4_unicode_ci"
	}
	charSet := "utf8mb4"
	if strings.HasPrefix(collation, "utf8_") {
		charSet = "utf8"
	}

	// Step 3: Create database & grant full privileges to Admin user
	mariaRoot = database.ResolveMariaRoot(mariaRoot)
	mysqlExe := filepath.Join(mariaRoot, "bin", "mysql.exe")
	if _, err := os.Stat(mysqlExe); err == nil {
		query := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET %s COLLATE %s; GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'localhost'; GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'127.0.0.1'; FLUSH PRIVILEGES;", dbName, charSet, collation, dbName, database.Creds.Username, dbName, database.Creds.Username)
		exec.Command(mysqlExe, "-u", "root", "-P", "3306", "-h", "127.0.0.1", "-e", query).Run()
		logger.Log("Created database `%s` and granted access to `%s`", dbName, database.Creds.Username)
	}

	// Step 4: Parse & Modify .env lines with Admin user credentials inside managed block
	lines, _ := readLines(envPath)
	updates := map[string]string{
		"APP_ENV":          "local",
		"APP_DEBUG":        "true",
		"APP_URL":          "http://127.0.0.1:" + port,
		"PORT":             port,
		"APP_PORT":         port,
		"DB_CONNECTION":    "mysql",
		"DB_HOST":          "127.0.0.1",
		"DB_PORT":          "3306",
		"DB_DATABASE":      dbName,
		"DB_USERNAME":      database.Creds.Username,
		"DB_PASSWORD":      database.Creds.Password,
		"MAIL_MAILER":      "smtp",
		"MAIL_HOST":        "127.0.0.1",
		"MAIL_PORT":        "1025",
		"QUEUE_CONNECTION": "database",
	}

	orderedKeys := []string{
		"APP_ENV", "APP_DEBUG", "APP_URL", "PORT", "APP_PORT",
		"DB_CONNECTION", "DB_HOST", "DB_PORT", "DB_DATABASE", "DB_USERNAME", "DB_PASSWORD",
		"MAIL_MAILER", "MAIL_HOST", "MAIL_PORT", "QUEUE_CONNECTION",
	}

	var customLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "V-COPANEL MANAGED VARIABLES") ||
			strings.Contains(trimmed, "DO NOT EDIT MANUALLY") ||
			strings.Contains(trimmed, "strictly controlled and synced") ||
			strings.Contains(trimmed, "Modify them only through") ||
			strings.Contains(trimmed, "USER CUSTOM VARIABLES") ||
			strings.Contains(trimmed, "===================================================================") {
			continue
		}
		if trimmed == "" {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		key := strings.TrimSpace(parts[0])
		if _, isManaged := updates[key]; isManaged {
			continue
		}
		customLines = append(customLines, line)
	}

	var newLines []string
	newLines = append(newLines, "# ===================================================================")
	newLines = append(newLines, "# 🔒 V-COPANEL MANAGED VARIABLES - DO NOT EDIT MANUALLY!")
	newLines = append(newLines, "# All variables below are strictly controlled and synced by V-CoPanel.")
	newLines = append(newLines, "# Modify them only through the V-CoPanel Studio UI dashboard.")
	newLines = append(newLines, "# ===================================================================")
	for _, k := range orderedKeys {
		newLines = append(newLines, fmt.Sprintf("%s=%s", k, updates[k]))
	}
	newLines = append(newLines, "# ===================================================================")
	newLines = append(newLines, "# 🔓 USER CUSTOM VARIABLES (Safe to edit manually below this line)")
	newLines = append(newLines, "# ===================================================================")
	newLines = append(newLines, customLines...)

	// Step 5: Write back modified .env
	return os.WriteFile(envPath, []byte(strings.Join(newLines, "\n")+"\n"), 0644)
}

func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}
