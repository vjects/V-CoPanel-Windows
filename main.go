package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"os/exec"

	"vcopanel-bridge/internal/api"
	"vcopanel-bridge/internal/database"
	"vcopanel-bridge/internal/logger"
	"vcopanel-bridge/internal/mailpit"
	"vcopanel-bridge/internal/mesh"
	"vcopanel-bridge/internal/precheck"
	"vcopanel-bridge/internal/process"
	"vcopanel-bridge/internal/redis"
	"vcopanel-bridge/internal/server"
	"vcopanel-bridge/internal/wallpaper"
)

var (
	assetsDir    string
	workspaceDir string
	startTime    = time.Now()
)

func ensureWorkspaceDirs(wsDir string) {
	os.MkdirAll(filepath.Join(wsDir, "core"), 0755)
	os.MkdirAll(filepath.Join(wsDir, "runtimes"), 0755)
	os.MkdirAll(filepath.Join(wsDir, "shared-services"), 0755)
	os.MkdirAll(filepath.Join(wsDir, "projects"), 0755)
}

func syncWallpapers(assetsDir, workspaceDir string) {
	wpDir := filepath.Join(workspaceDir, "wallpapers")
	os.MkdirAll(wpDir, 0755)
	if _, err := os.Stat(filepath.Join(wpDir, "default.png")); os.IsNotExist(err) {
		wallpaper.SyncBingDaily(workspaceDir)
	}
}

func main() {
	cwd, _ := os.Getwd()
	logger.InitSystemConsoleCapture()

	defer func() {
		if r := recover(); r != nil {
			errStr := fmt.Sprintf("PANIC: %v\n", r)
			os.WriteFile(filepath.Join(cwd, "crash.log"), []byte(errStr), 0644)
			fmt.Println(errStr)
			time.Sleep(10 * time.Second)
		}
	}()

	assetsDir = filepath.Join(cwd, "pc-assets")
	workspaceDir = filepath.Join(cwd, "workspace")
	if envWp := os.Getenv("VCOPANEL_WORKSPACE"); envWp != "" {
		workspaceDir = envWp
	}

	ensureWorkspaceDirs(workspaceDir)
	syncWallpapers(assetsDir, workspaceDir)

	pm := process.NewManager()

	mariaRoot := database.ResolveMariaRoot(filepath.Join(workspaceDir, "shared-services", "mariadb"))
	ctx := &api.ServerContext{
		WorkspaceDir:   workspaceDir,
		AssetsDir:      assetsDir,
		MariaRoot:      mariaRoot,
		Cwd:            cwd,
		ProcessManager: pm,
	}

	// Initial bootstrap
	go mesh.StartProxy()

	stage1Done := make(chan struct{})
	go func() {
		precheck.Instance.RunStage1(assetsDir, workspaceDir)
		close(stage1Done)
	}()

	// Wait for Stage 1 (Core & Shared Services) to finish before starting services
	go func() {
		<-stage1Done
		
		// Re-resolve mariaRoot after Stage 1 extraction
		mariaRoot = database.ResolveMariaRoot(filepath.Join(workspaceDir, "shared-services", "mariadb"))
		ctx.MariaRoot = mariaRoot

		// 1. Check & verify Core PHP is available for internal servers
		corePhpExe := filepath.Join(workspaceDir, "core", "php", "php.exe")
		if _, err := os.Stat(corePhpExe); err == nil {
			logger.Log("Core PHP verified in workspace/core/php")
		}
		time.Sleep(500 * time.Millisecond)

		// 2. MariaDB check & start

		mysqldExe := filepath.Join(mariaRoot, "bin", "mysqld.exe")
		if _, err := os.Stat(mysqldExe); err == nil {
			if !database.IsMariaDBRunning() {
				dataDir := filepath.Join(mariaRoot, "data")
				if _, err := os.Stat(filepath.Join(dataDir, "mysql")); os.IsNotExist(err) {
					logger.Log("Initializing MariaDB system database schema (first run)...")
					os.RemoveAll(dataDir)
					os.MkdirAll(dataDir, 0755)
					installDbExe := filepath.Join(mariaRoot, "bin", "mariadb-install-db.exe")
					if _, err := os.Stat(installDbExe); err != nil {
						installDbExe = filepath.Join(mariaRoot, "bin", "mysql_install_db.exe")
					}
					cmd := exec.Command(installDbExe, "--datadir="+dataDir)
					if out, err := cmd.CombinedOutput(); err != nil {
						logger.Log("ERROR: mariadb-install-db failed: %v | %s", err, string(out))
					} else {
						logger.Log("MariaDB data directory initialized successfully.")
					}
				}
				pm.StartBackground("mariadb", mysqldExe, "--datadir="+dataDir, "--port=3306", "--bind-address=127.0.0.1")
				time.Sleep(3000 * time.Millisecond) // Ensure MariaDB process is up before DB init
			} else {
				logger.Log("MariaDB is already running. Skipping start.")
			}

			// 3. MariaDB Admin in database (System DB & Admin user init)
			if err := database.InitSystemDB(mariaRoot, "3306"); err != nil {
				logger.Log("FATAL ERROR: Failed to initialize MariaDB system schema! %v", err)
				time.Sleep(30 * time.Second)
				os.Exit(1)
			}
			database.SyncInfrastructureMetadata(workspaceDir, mariaRoot, "3306", "", "")
			time.Sleep(500 * time.Millisecond)

			// Record core database and bridge engine states
			database.SaveServiceState(mariaRoot, "3306", "bridge", "V-CoPanel Bridge Engine", 8880, "http://localhost:8880", "running")
			database.SaveServiceState(mariaRoot, "3306", "mariadb", "MariaDB Database Engine", 3306, "127.0.0.1:3306", "running")
			
			// 4. phpMyAdmin check & start
			pmaStatus := "stopped"
			if !database.IsPHPMyAdminRunning() {
				database.StartPHPMyAdmin(workspaceDir)
				pmaStatus = "running"
				time.Sleep(500 * time.Millisecond)
			} else {
				logger.Log("phpMyAdmin is already running. Skipping start.")
				pmaStatus = "running"
			}
			database.SaveServiceState(mariaRoot, "3306", "phpmyadmin", "phpMyAdmin Universal GUI", 8881, "http://127.0.0.1:8881", pmaStatus)
			
			// 5. Mailpit check & start
			mailpitStatus := "stopped"
			if !mailpit.Instance.Status() {
				mailpit.Instance.Start(assetsDir, workspaceDir)
				mailpitStatus = "running"
				time.Sleep(500 * time.Millisecond)
			} else {
				logger.Log("Mailpit is already running. Skipping start.")
				mailpitStatus = "running"
			}
			database.SaveServiceState(mariaRoot, "3306", "mailpit", "Mailpit SMTP & Webmail Server", 8025, "http://127.0.0.1:8025", mailpitStatus)
		}

		// 6. Redis check & start
		redisExe := filepath.Join(workspaceDir, "shared-services", "redis", "redis-server.exe")
		if _, errR := os.Stat(redisExe); errR == nil {
			redisStatus := "stopped"
			if !redis.GetStatus(workspaceDir).Running {
				pm.StartBackground("redis", redisExe, "--port", "6379")
				redisStatus = "running"
				time.Sleep(500 * time.Millisecond)
			} else {
				logger.Log("Redis is already running. Skipping start.")
				redisStatus = "running"
			}
			database.SaveServiceState(mariaRoot, "3306", "redis", "Redis Portable Cache", 6379, "127.0.0.1:6379", redisStatus)
		}

		// 7. Verify all server connections before opening browser
		logger.Log("Verifying all database & server connections before opening browser...")
		if err := database.TestAdminConnection(mariaRoot, "3306"); err != nil {
			logger.Log("FATAL ERROR: MariaDB Admin Connection Failed! %v", err)
			fmt.Fprintf(os.Stderr, "\n===================================================================\n")
			fmt.Fprintf(os.Stderr, " CRITICAL LAUNCH ERROR: MARIADB ADMIN VERIFICATION FAILED\n")
			fmt.Fprintf(os.Stderr, " %v\n", err)
			fmt.Fprintf(os.Stderr, " Please check MariaDB logs and verify admin user privileges.\n")
			fmt.Fprintf(os.Stderr, "===================================================================\n\n")
			time.Sleep(30 * time.Second)
			os.Exit(1)
		}
		if err := database.TestServicePort("phpMyAdmin", "8881"); err != nil {
			logger.Log("ERROR: phpMyAdmin verification failed: %v", err)
		}
		if err := database.TestServicePort("Mailpit", "8025"); err != nil {
			logger.Log("ERROR: Mailpit verification failed: %v", err)
		}
		if err := database.TestServicePort("Redis", "6379"); err != nil {
			logger.Log("ERROR: Redis verification failed: %v", err)
		}

		// Launch System Default Browser ONLY AFTER Stage 1 and all Shared Services are verified live
		logger.Log("All Core & Shared Services are verified live! Launching dashboard in browser...")
		switch runtime.GOOS {
		case "windows":
			exec.Command("cmd", "/c", "start", "", "http://localhost:8880").Start()
		case "darwin":
			exec.Command("open", "http://localhost:8880").Start()
		default:
			exec.Command("xdg-open", "http://localhost:8880").Start()
		}

		// Launch Pre-check Stage 2 (Runtimes & Wallpapers) in background after shared services are live
		time.Sleep(500 * time.Millisecond)
		go precheck.Instance.RunStage2(assetsDir, workspaceDir)
	}()

	mux := http.NewServeMux()
	
	// Setup static files
	webDir := filepath.Join(cwd, "web")
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/panel" {
			http.ServeFile(w, r, filepath.Join(webDir, "index.html"))
			return
		}
		if r.URL.Path == "/favicon.ico" || r.URL.Path == "/logo.ico" || r.URL.Path == "/icon.ico" {
			http.ServeFile(w, r, filepath.Join(cwd, "internal", "icon.ico"))
			return
		}
		if r.URL.Path == "/logo.png" || r.URL.Path == "/img/logo.png" || r.URL.Path == "/icon.png" {
			http.ServeFile(w, r, filepath.Join(cwd, "internal", "logo.png"))
			return
		}
		if strings.HasPrefix(r.URL.Path, "/wallpapers/") {
			wpPath := filepath.Join(workspaceDir, "wallpapers", strings.TrimPrefix(r.URL.Path, "/wallpapers/"))
			logger.Log("[HTTP] Serve wallpaper request: %s -> local path: %s", r.URL.Path, wpPath)
			if stat, err := os.Stat(wpPath); err == nil && !stat.IsDir() {
				http.ServeFile(w, r, wpPath)
				return
			} else {
				logger.Log("[HTTP] Wallpaper file not found or stats error: %v", err)
			}
		}
		target := filepath.Join(webDir, filepath.FromSlash(r.URL.Path))
		if stat, err := os.Stat(target); err == nil && !stat.IsDir() {
			http.ServeFile(w, r, target)
			return
		}
		http.ServeFile(w, r, filepath.Join(webDir, "index.html"))
	})

	// Setup API Routes
	api.RegisterAllRoutes(mux, ctx)

	// Graceful Shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		logger.Log("Shutting down V-CoPanel Bridge...")
		database.StopMariaDB()
		database.StopPHPMyAdmin()
		mailpit.Instance.Stop()
		redis.Stop(workspaceDir)
		server.Instance.StopAll()
		os.Exit(0)
	}()

	port := "8880"
	logger.Log("==================================================================")
	logger.Log("🚀 V-CoPanel Windows Bridge Engine v2.0.1")
	logger.Log("   Copyright (c) 2026 VJECTS (vjects.com). All rights reserved.")
	logger.Log("   Telegram Support: @vjects (https://t.me/vjects)")
	logger.Log("   Repository: https://github.com/vjects/V-CoPanel-Windows.git")
	logger.Log("==================================================================")
	logger.Log("CORE DASHBOARD READY & LISTENING ON http://localhost:%s", port)

	err := http.ListenAndServe("127.0.0.1:"+port, mux)
	if err != nil {
		logger.Log("FATAL ERROR: Server crashed or port 8880 is already in use: %v", err)
		time.Sleep(30 * time.Second)
	}
}
