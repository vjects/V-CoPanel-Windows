package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"vcopanel-bridge/internal/database"
	"vcopanel-bridge/internal/discovery"
	"vcopanel-bridge/internal/envmanager"
	"vcopanel-bridge/internal/logger"
	"vcopanel-bridge/internal/server"
	"vcopanel-bridge/internal/shims"
)

func registerProjectsRoutes(mux *http.ServeMux, ctx *ServerContext) {
	type ExtendedProject struct {
		database.DBProjectRecord
		ServeRunning bool `json:"serve_running"`
		QueueRunning bool `json:"queue_running"`
	}

	mux.HandleFunc("/api/projects/list", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		dbProjects, _ := database.FetchAllProjectsFromDB(ctx.MariaRoot, "3306")
		var projects []ExtendedProject
		for _, p := range dbProjects {
			projects = append(projects, ExtendedProject{
				DBProjectRecord: p,
				ServeRunning:    server.Instance.IsServeRunning(p.Path),
				QueueRunning:    server.Instance.IsQueueRunning(p.Path),
			})
		}
		if projects == nil {
			projects = []ExtendedProject{}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "projects": projects})
	})

	mux.HandleFunc("/api/projects/list-full", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		dbProjects, _ := database.FetchAllProjectsFromDB(ctx.MariaRoot, "3306")
		var projects []ExtendedProject
		for _, p := range dbProjects {
			projects = append(projects, ExtendedProject{
				DBProjectRecord: p,
				ServeRunning:    server.Instance.IsServeRunning(p.Path),
				QueueRunning:    server.Instance.IsQueueRunning(p.Path),
			})
		}
		if projects == nil {
			projects = []ExtendedProject{}
		}
		json.NewEncoder(w).Encode(projects)
	})

	mux.HandleFunc("/api/projects/clear-pending", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		err := database.ClearPendingProjectsFromDB(ctx.MariaRoot, "3306")
		if err != nil {
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/api/projects/scan", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		pDir := database.GetProjectsDirectory(ctx.MariaRoot, "3306")
		if pDir == "" {
			json.NewEncoder(w).Encode(map[string]string{"error": "workspace_not_set"})
			return
		}

		database.ClearPendingProjectsFromDB(ctx.MariaRoot, "3306")
		
		existing, _ := database.FetchAllProjectsFromDB(ctx.MariaRoot, "3306")
		configuredPaths := make(map[string]discovery.ProjectInfo)
		for _, ex := range existing {
			configuredPaths[ex.Path] = discovery.ProjectInfo{
				UUID: ex.UUID,
				Name: ex.Name,
				Path: ex.Path,
				Stack: ex.Stack,
				Framework: ex.Framework,
				Status: ex.Status,
				PHPVersion: ex.PHPVersion,
				NodeVersion: ex.NodeVersion,
				GoVersion: ex.GoVersion,
				Port: ex.Port,
				DBName: ex.DBName,
			}
		}

		scanned, err := discovery.ScanProjects(pDir, configuredPaths)
		if err == nil {
			for _, p := range scanned {
				database.SaveProjectToDB(ctx.MariaRoot, "3306", p.UUID, p.Path, p.Name, p.Stack, p.Framework, p.Status, p.PHPVersion, p.NodeVersion, p.GoVersion, p.Port, p.DBName, "utf8mb4_unicode_ci", "standard")
			}
		}
		
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/api/projects/provision", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Path          string `json:"path"`
			PHPVersion    string `json:"php_version"`
			NodeVersion   string `json:"node_version"`
			GoVersion     string `json:"go_version"`
			Port          string `json:"port"`
			Collation     string `json:"collation"`
			Stack         string `json:"stack"`
			Framework     string `json:"framework"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		
		stack, framework := req.Stack, req.Framework
		if stack == "" {
			stack, framework = discovery.DetectStackAndFramework(req.Path)
		}
		
		uuid := generateUUID(req.Path)
		dbname := "db_" + uuid[:8]
		
		shims.GenerateShims(req.Path, stack, req.PHPVersion, req.NodeVersion, req.GoVersion, ctx.WorkspaceDir, ctx.AssetsDir)
		
		envVars := map[string]string{
			"APP_NAME":         filepath.Base(req.Path),
			"APP_URL":          "http://127.0.0.1:" + req.Port,
			"PORT":             req.Port,
			"DB_CONNECTION":    "mysql",
			"DB_HOST":          "127.0.0.1",
			"DB_PORT":          "3306",
			"DB_DATABASE":      dbname,
			"DB_USERNAME":      "root",
			"DB_PASSWORD":      "",
			"QUEUE_CONNECTION": "database",
			"REDIS_HOST":       "127.0.0.1",
			"REDIS_PASSWORD":   "null",
			"REDIS_PORT":       "6379",
			"MAIL_MAILER":      "smtp",
			"MAIL_HOST":        "127.0.0.1",
			"MAIL_PORT":        "3025",
		}
		envmanager.SyncSandboxBlock(filepath.Join(req.Path, ".env"), envVars)
		
		rtContent := fmt.Sprintf(`{"stack":"%s","framework":"%s","port":"%s","db_name":"%s"}`, stack, framework, req.Port, dbname)
		os.WriteFile(filepath.Join(req.Path, ".vcopanel-runtime.json"), []byte(rtContent), 0644)
		
		// Inject Architecture Documentation
		if docContent, err := os.ReadFile(filepath.Join(ctx.WorkspaceDir, "..", "internal", "V-CoPanel-Architecture.md")); err == nil {
			os.WriteFile(filepath.Join(req.Path, "V-CoPanel-Architecture.md"), docContent, 0644)
		}

		database.SaveProjectToDB(ctx.MariaRoot, "3306", uuid, req.Path, filepath.Base(req.Path), stack, framework, "Configured", req.PHPVersion, req.NodeVersion, req.GoVersion, req.Port, dbname, req.Collation, "shared")
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})





	mux.HandleFunc("/api/projects/delete", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Path   string `json:"path"`
			DBName string `json:"db_name"`
			UUID   string `json:"uuid"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		
		cleanPath := filepath.ToSlash(filepath.Clean(req.Path))
		if server.Instance.IsServeRunning(cleanPath) {
			server.Instance.StopServe(cleanPath)
		}
		if server.Instance.IsQueueRunning(cleanPath) {
			server.Instance.StopQueue(cleanPath)
		}
		
		if err := database.DeleteProjectAndDatabase(ctx.MariaRoot, "3306", cleanPath, req.DBName, req.UUID); err != nil {
			logger.Log("Failed to delete project from DB: %v", err)
		}
		
		// Ensure safety before deleting from disk
		if cleanPath != "" && len(cleanPath) > 5 {
			time.Sleep(1 * time.Second) // wait for server taskkill to release locks
			winPath := filepath.FromSlash(cleanPath)
			psCmd := fmt.Sprintf(`Remove-Item -Recurse -Force -LiteralPath "%s"`, winPath)
			if err := exec.Command("powershell", "-NoProfile", "-Command", psCmd).Run(); err != nil {
				logger.Log("Failed to delete project directory %s: %v", cleanPath, err)
				http.Error(w, fmt.Sprintf("Failed to delete directory: %v", err), 500)
				return
			}
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/api/projects/eject", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Path   string `json:"path"`
			DBName string `json:"db_name"`
			UUID   string `json:"uuid"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		
		cleanPath := filepath.ToSlash(filepath.Clean(req.Path))
		if server.Instance.IsServeRunning(cleanPath) {
			server.Instance.StopServe(cleanPath)
		}
		if server.Instance.IsQueueRunning(cleanPath) {
			server.Instance.StopQueue(cleanPath)
		}
		
		// Drop DB and update panel's database to Pending state
		if err := database.EjectProjectFromDB(ctx.MariaRoot, "3306", cleanPath, req.DBName, req.UUID); err != nil {
			logger.Log("Failed to eject project from DB: %v", err)
		}
		
		// Revert .env to original state
		envPath := filepath.Join(cleanPath, ".env")
		if err := envmanager.EjectSandboxBlock(envPath); err != nil {
			logger.Log("Warning: Failed to eject sandbox block from .env: %v", err)
		}
		
		// Remove runtime config and doc
		os.Remove(filepath.Join(cleanPath, ".vcopanel-runtime.json"))
		os.Remove(filepath.Join(cleanPath, "V-CoPanel-Architecture.md"))
		
		// Clean up shims
		shims.EjectToolsVersionBlock(filepath.Join(cleanPath, ".tools-version"))
		os.Remove(filepath.Join(cleanPath, "php.bat"))
		os.Remove(filepath.Join(cleanPath, "composer.bat"))
		os.Remove(filepath.Join(cleanPath, "queue.bat"))
		os.Remove(filepath.Join(cleanPath, "schedule.bat"))
		os.Remove(filepath.Join(cleanPath, "npm.cmd"))
		os.Remove(filepath.Join(cleanPath, "npm.bat"))
		os.Remove(filepath.Join(cleanPath, "node.cmd"))
		os.Remove(filepath.Join(cleanPath, "node.bat"))
		os.Remove(filepath.Join(cleanPath, "go.bat"))
		os.Remove(filepath.Join(cleanPath, "run.bat"))
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/api/project/code", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Query().Get("path")
		if path == "" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"error": "Path is required"})
			return
		}
		
		cmd := exec.Command("cmd", "/c", "code", ".")
		cmd.Dir = path
		err := cmd.Start()
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to start VS Code: " + err.Error()})
			return
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/api/project/open", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Query().Get("path")
		
		cmd := exec.Command("cmd", "/c", "explorer", filepath.FromSlash(path))
		cmd.Start()
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/api/projects/terminal", func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Path string }
		json.NewDecoder(r.Body).Decode(&req)
		
		shims.OpenTerminal(req.Path)
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/api/projects/scaffold", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name         string `json:"name"`
			Stack        string `json:"stack"`
			Port         string `json:"port"`
			Collation    string `json:"collation"`
			PHPVersion   string `json:"php_version"`
			NodeVersion  string `json:"node_version"`
			GoVersion    string `json:"go_version"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		
		pDir := database.GetProjectsDirectory(ctx.MariaRoot, "3306")
		targetPath := filepath.Join(pDir, req.Name)
		if _, err := os.Stat(targetPath); err == nil {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]string{"error": "Directory already exists"})
			return
		}
		
		os.MkdirAll(targetPath, 0755)
		
		if req.Stack == "laravel" || req.Stack == "php" {
			os.WriteFile(filepath.Join(targetPath, "index.php"), []byte("<?php\necho 'Hello World';\n"), 0644)
		} else if req.Stack == "node" {
			os.WriteFile(filepath.Join(targetPath, "index.js"), []byte("console.log('Hello Node');\n"), 0644)
		} else if req.Stack == "go" {
			os.WriteFile(filepath.Join(targetPath, "main.go"), []byte("package main\n\nfunc main() {\n}\n"), 0644)
		}
		
		uuid := generateUUID(targetPath)
		dbname := "db_" + uuid[:8]
		tvContent := "# V-CoPanel Portable Tool Versions\n"
		if req.PHPVersion != "" { tvContent += fmt.Sprintf("php %s\n", req.PHPVersion) }
		if req.NodeVersion != "" { tvContent += fmt.Sprintf("node %s\n", req.NodeVersion) }
		if req.GoVersion != "" { tvContent += fmt.Sprintf("go %s\n", req.GoVersion) }
		os.WriteFile(filepath.Join(targetPath, ".tools-version"), []byte(tvContent), 0644)
		
		envVars := map[string]string{
			"APP_NAME":         req.Name,
			"APP_URL":          "http://127.0.0.1:" + req.Port,
			"PORT":             req.Port,
			"DB_CONNECTION":    "mysql",
			"DB_HOST":          "127.0.0.1",
			"DB_PORT":          "3306",
			"DB_DATABASE":      dbname,
			"DB_USERNAME":      "root",
			"DB_PASSWORD":      "",
			"QUEUE_CONNECTION": "database",
			"REDIS_HOST":       "127.0.0.1",
			"REDIS_PASSWORD":   "null",
			"REDIS_PORT":       "6379",
			"MAIL_MAILER":      "smtp",
			"MAIL_HOST":        "127.0.0.1",
			"MAIL_PORT":        "3025",
		}
		envmanager.SyncSandboxBlock(filepath.Join(targetPath, ".env"), envVars)
		
		fw := req.Stack
		if req.Stack == "empty" { fw = "Empty" }
		rtContent := fmt.Sprintf(`{"stack":"%s","framework":"%s","port":"%s","db_name":"%s"}`, req.Stack, fw, req.Port, dbname)
		os.WriteFile(filepath.Join(targetPath, ".vcopanel-runtime.json"), []byte(rtContent), 0644)
		
		database.SaveProjectToDB(ctx.MariaRoot, "3306", uuid, targetPath, req.Name, req.Stack, fw, "Configured", req.PHPVersion, req.NodeVersion, req.GoVersion, req.Port, dbname, req.Collation, "shared")
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
}

