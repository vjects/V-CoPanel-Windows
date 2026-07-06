package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"vcopanel-bridge/internal/database"
	"vcopanel-bridge/internal/discovery"
	"vcopanel-bridge/internal/logger"
	"vcopanel-bridge/internal/mailpit"
	"vcopanel-bridge/internal/notifier"
	"vcopanel-bridge/internal/precheck"
	"vcopanel-bridge/internal/redis"
	"vcopanel-bridge/internal/server"
	"vcopanel-bridge/internal/wallpaper"
)

func registerSystemRoutes(mux *http.ServeMux, ctx *ServerContext) {
	mux.HandleFunc("/api/system/select-folder", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		
		script := `
$code = @"
using System;
using System.Runtime.InteropServices;
public class FolderPicker {
    [DllImport("shell32.dll", CharSet=CharSet.Unicode)] static extern int SHCreateItemFromParsingName(string pszPath, IntPtr pbc, ref Guid riid, out IntPtr ppv);
    [ComImport, Guid("DC1C5A9C-E88A-4dde-A5A1-60F82A20AEF7")] class FileOpenDialog {}
    [ComImport, Guid("D57C7288-D4AD-4768-BE02-9D969532D960"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
    interface IFileOpenDialog {
        [PreserveSig] int Show(IntPtr parent);
        void SetFileTypes(uint cFileTypes, IntPtr rgFilterSpec);
        void SetFileTypeIndex(uint iFileType);
        void GetFileTypeIndex(out uint piFileType);
        void Advise(IntPtr pfde, out uint pdwCookie);
        void Unadvise(uint dwCookie);
        void SetOptions(uint fos);
        void GetOptions(out uint pfos);
        void SetDefaultFolder(IntPtr psi);
        void SetFolder(IntPtr psi);
        void GetFolder(out IntPtr ppsi);
        void GetCurrentSelection(out IntPtr ppsi);
        void SetFileName([MarshalAs(UnmanagedType.LPWStr)] string pszName);
        void GetFileName([MarshalAs(UnmanagedType.LPWStr)] out string pszName);
        void SetTitle([MarshalAs(UnmanagedType.LPWStr)] string pszTitle);
        void SetOkButtonLabel([MarshalAs(UnmanagedType.LPWStr)] string pszText);
        void SetFileNameLabel([MarshalAs(UnmanagedType.LPWStr)] string pszLabel);
        void GetResult(out IntPtr ppsi);
        void AddPlace(IntPtr psi, int fdap);
        void SetDefaultExtension([MarshalAs(UnmanagedType.LPWStr)] string pszDefaultExtension);
        void Close(int hr);
        void SetClientGuid(ref Guid guid);
        void ClearClientData();
        void SetFilter(IntPtr pFilter);
        void GetResults(out IntPtr ppenum);
        void GetSelectedItems(out IntPtr ppsai);
    }
    [ComImport, Guid("43826D1E-E718-42EE-BC55-A1E261C37BFE"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
    interface IShellItem {
        void BindToHandler(IntPtr pbc, ref Guid bhid, ref Guid riid, out IntPtr ppv);
        void GetParent(out IShellItem ppsi);
        void GetDisplayName(uint sigdnName, [MarshalAs(UnmanagedType.LPWStr)] out string ppszName);
        void GetAttributes(uint sfgaoMask, out uint psfgaoAttribs);
        void Compare(IShellItem psi, uint hint, out int piOrder);
    }
    public static string Pick() {
        var dlg = (IFileOpenDialog)new FileOpenDialog();
        dlg.SetOptions(0x20 | 0x40); // FOS_PICKFOLDERS | FOS_FORCEFILESYSTEM
        dlg.SetTitle("Select Workspace Directory");
        int hr = dlg.Show(IntPtr.Zero);
        if (hr != 0) return "";
        IntPtr ppsi;
        dlg.GetResult(out ppsi);
        var si = (IShellItem)Marshal.GetTypedObjectForIUnknown(ppsi, typeof(IShellItem));
        string path;
        si.GetDisplayName(0x80058000, out path);
        return path;
    }
}
"@
Add-Type -TypeDefinition $code -Language CSharp
[FolderPicker]::Pick()
`
		cmd := exec.Command("powershell", "-NoProfile", "-STA", "-Command", script)
		out, err := cmd.Output()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		
		selectedPath := strings.TrimSpace(string(out))
		if selectedPath == "" {
			json.NewEncoder(w).Encode(map[string]string{"status": "cancelled"})
			return
		}
		
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "path": selectedPath})
	})

	mux.HandleFunc("/api/workspace/get", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		pDir := database.GetProjectsDirectory(ctx.MariaRoot, "3306")
		json.NewEncoder(w).Encode(map[string]string{"workspace": filepath.FromSlash(pDir)})
	})

	mux.HandleFunc("/api/workspace/set", func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Workspace string }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		cleanDir := filepath.ToSlash(filepath.Clean(req.Workspace))
		os.MkdirAll(cleanDir, 0755)
		database.SetProjectsDirectory(ctx.MariaRoot, "3306", cleanDir)
		database.ClearAllProjectsFromDB(ctx.MariaRoot, "3306")

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

		scanned, err := discovery.ScanProjects(cleanDir, configuredPaths)
		if err == nil {
			for _, p := range scanned {
				database.SaveProjectToDB(ctx.MariaRoot, "3306", p.UUID, p.Path, p.Name, p.Stack, p.Framework, p.Status, p.PHPVersion, p.NodeVersion, p.GoVersion, p.Port, p.DBName, "utf8mb4_unicode_ci", "standard")
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "workspace": cleanDir})
	})

	mux.HandleFunc("/api/system/console/logs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte(logger.Console.GetAll()))
	})

	mux.HandleFunc("/api/system/console/clear", func(w http.ResponseWriter, r *http.Request) {
		logger.Console.Clear()
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/api/precheck/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		assetStatus := make(map[string]bool)
		allReady := true
		
		assets, err := database.FetchOfflineCatalog(ctx.MariaRoot, "3306")
		if err == nil && len(assets) > 0 {
			for _, a := range assets {
				assetStatus[a.AssetName] = a.IsExtracted
				if !a.IsExtracted {
					allReady = false
				}
			}
		} else {
			// Fallback to scanning pc-assets if DB is not ready
			checkExtractionTarget := func(name string) bool {
				lower := strings.ToLower(name)
				if strings.HasPrefix(lower, "mariadb") {
					if info, err := os.Stat(filepath.Join(ctx.WorkspaceDir, "shared-services", "mariadb")); err == nil && info.IsDir() { return true }
				} else if strings.HasPrefix(lower, "redis") {
					if info, err := os.Stat(filepath.Join(ctx.WorkspaceDir, "shared-services", "redis")); err == nil && info.IsDir() { return true }
				} else if strings.HasPrefix(lower, "phpmyadmin") {
					if info, err := os.Stat(filepath.Join(ctx.WorkspaceDir, "shared-services", "phpmyadmin")); err == nil && info.IsDir() { return true }
				} else if strings.HasPrefix(lower, "mailpit") {
					if info, err := os.Stat(filepath.Join(ctx.WorkspaceDir, "shared-services", "mailpit")); err == nil && info.IsDir() { return true }
				} else if strings.HasPrefix(lower, "composer") {
					if info, err := os.Stat(filepath.Join(ctx.WorkspaceDir, "runtimes", "composer")); err == nil && info.IsDir() { return true }
				} else if strings.HasSuffix(lower, ".exe") && strings.Contains(lower, "redist") {
					if _, err := os.Stat(filepath.Join(ctx.WorkspaceDir, "core", "vcredist_installed.flag")); err == nil { return true }
				} else if lower == "icon.ico" {
					if _, err := os.Stat(filepath.Join(ctx.WorkspaceDir, "pc-assets", "icon.ico")); err == nil { return true }
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
					
					if info, err := os.Stat(filepath.Join(ctx.WorkspaceDir, "runtimes", subDir)); err == nil && info.IsDir() { return true }
				}
				return false
			}

			if entries, err := os.ReadDir(ctx.AssetsDir); err == nil {
				for _, e := range entries {
					if e.IsDir() && e.Name() != "Wallpapers" && e.Name() != "Stack-Logo" {
						catDir := filepath.Join(ctx.AssetsDir, e.Name())
						if sub, errSub := os.ReadDir(catDir); errSub == nil {
							for _, sf := range sub {
								if !sf.IsDir() {
									name := sf.Name()
									ext := checkExtractionTarget(name)
									assetStatus[name] = ext
									if !ext { allReady = false }
								}
							}
						}
					} else if !e.IsDir() {
						name := e.Name()
						ext := checkExtractionTarget(name)
						assetStatus[name] = ext
						if !ext { allReady = false }
					}
				}
			}
			if len(assetStatus) == 0 { allReady = false }
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"running": precheck.Instance.Status(),
			"ready":   allReady,
			"assets":  assetStatus,
			"logs":    precheck.Instance.GetLogs(),
		})
	})

	mux.HandleFunc("/api/precheck/run", func(w http.ResponseWriter, r *http.Request) {
		go precheck.Instance.RunFullProvisioning(ctx.AssetsDir, ctx.WorkspaceDir)
		w.WriteHeader(http.StatusOK)
	})

var (
	resetActive   bool
	resetProgress float64
	resetStatus   string
	resetMu       sync.Mutex
)

	mux.HandleFunc("/api/engine/reset-status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resetMu.Lock()
		defer resetMu.Unlock()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"active":   resetActive,
			"progress": resetProgress,
			"status":   resetStatus,
		})
	})

	mux.HandleFunc("/api/engine/reset", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		
		resetMu.Lock()
		if resetActive {
			resetMu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "already_active"})
			return
		}
		resetActive = true
		resetProgress = 0
		resetStatus = "Initializing factory reset..."
		resetMu.Unlock()

		// Run the reset sequence in a background goroutine to report progress to the browser
		go func() {
			// 1. Terminate all active processes & services
			resetMu.Lock()
			resetStatus = "Terminating active service daemons (PHP, DB, Redis)..."
			resetMu.Unlock()
			exec.Command("taskkill", "/f", "/im", "php.exe", "/im", "mysqld.exe", "/im", "mariadbd.exe", "/im", "redis-server.exe", "/im", "mailpit.exe", "/im", "node.exe").Run()
			time.Sleep(2500 * time.Millisecond) // Wait 2.5 seconds for Windows to release file handles

			// 2. Scan the workspace directory
			resetMu.Lock()
			resetStatus = "Scanning workspace files..."
			resetMu.Unlock()

			var files []string
			filepath.Walk(ctx.WorkspaceDir, func(path string, info os.FileInfo, err error) error {
				if err == nil && path != ctx.WorkspaceDir {
					files = append(files, path)
				}
				return nil
			})
			total := len(files)

			// Sort files descending by length so children are deleted before directories
			sort.Slice(files, func(i, j int) bool {
				return len(files[i]) > len(files[j])
			})

			// 3. Delete files sequentially with progress reporting
			for i, path := range files {
				var err error
				for retry := 0; retry < 3; retry++ {
					err = os.Remove(path)
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond) // Wait briefly for lock release on failure
				}
				
				// Smooth progress bar update
				time.Sleep(1 * time.Millisecond)

				if i%100 == 0 && total > 0 {
					resetMu.Lock()
					resetProgress = (float64(i) / float64(total)) * 95.0
					resetStatus = fmt.Sprintf("Wiping workspace: cleaned %d/%d entries", i, total)
					resetMu.Unlock()
				}
			}

			// 4. Wipe root workspace folder
			resetMu.Lock()
			resetStatus = "Performing final directory cleanup..."
			resetMu.Unlock()
			os.RemoveAll(ctx.WorkspaceDir)

			resetMu.Lock()
			resetProgress = 100
			resetStatus = "System reset completed. Restarting bridge..."
			resetMu.Unlock()

			// Let the browser fetch the 100% state
			time.Sleep(2 * time.Second)

			// 5. Generate restart batch script
			projectRoot := filepath.Join(ctx.WorkspaceDir, "..")
			resetScriptPath := filepath.Join(projectRoot, "reset_vco.bat")

			resetScript := `
@echo off
title V-CoPanel Reset Engine
echo Terminating bridge daemon...
taskkill /f /im bridge.exe >nul 2>&1

echo Terminating start.bat console...
taskkill /f /fi "WINDOWTITLE eq V-CoPanel Windows*" /im cmd.exe >nul 2>&1

echo Deleting bridge engine binary...
del /f /q "bridge.exe" >nul 2>&1

echo Restarting platform launcher...
start "" "start.bat"

echo Done. Self-destructing.
(goto) 2>nul & del "%~f0"
`
			os.WriteFile(resetScriptPath, []byte(strings.TrimSpace(resetScript)), 0755)

			// Trigger restart script
			cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command",
				fmt.Sprintf("Start-Process -FilePath '%s' -WorkingDirectory '%s'", resetScriptPath, projectRoot))
			cmd.Dir = projectRoot
			cmd.Start()
		}()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/api/system/profile", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		reqChecks := []string{
			filepath.Join(ctx.WorkspaceDir, "shared-services", "mariadb"),
			filepath.Join(ctx.WorkspaceDir, "shared-services", "redis"),
			filepath.Join(ctx.WorkspaceDir, "runtimes", "php-8.3"),
			filepath.Join(ctx.WorkspaceDir, "runtimes", "node-v22"),
			filepath.Join(ctx.WorkspaceDir, "core", "go"),
		}
		allReady := true
		for _, target := range reqChecks {
			if info, err := os.Stat(target); err != nil || !info.IsDir() {
				allReady = false
				break
			}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"app_name":   "V-CoPanel",
			"edition":    "Multi-Stack Universal Engine",
			"version":    "v2.5.0-portable",
			"first_boot": !allReady,
		})
	})

	mux.HandleFunc("/api/engine/info", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		
		recs, _ := database.FetchInfrastructureRecords(ctx.MariaRoot, "3306")
		engineVer := "v1.0.0"
		corePhp := "php-8.3"
		
		for _, rec := range recs {
			if rec.Key == "vcopanel_engine" {
				engineVer = rec.Version
			}
			if rec.Category == "core" && rec.Key == "php" {
				corePhp = rec.Version
			}
		}
		
		json.NewEncoder(w).Encode(map[string]interface{}{
			"app_name":     "V-CoPanel",
			"edition":      "Enterprise / Pro Edition",
			"version":      engineVer,
			"core_php":     corePhp,
			"architecture": "3-Pillar Workspace",
			"daemon":       "Go Bridge Daemon (Active)",
		})
	})

	mux.HandleFunc("/api/engine/status_matrix", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"mariadb":    database.IsMariaDBRunning(),
			"redis":      redis.GetStatus(ctx.WorkspaceDir).Running,
			"mailpit":    mailpit.Instance.Status(),
			"phpmyadmin": database.IsPHPMyAdminRunning(),
		})
	})

	mux.HandleFunc("/api/system/wallpapers", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Kick off background sync (non-blocking)
		go wallpaper.SyncBingDaily(ctx.WorkspaceDir)
		// Return list of already-downloaded wallpaper files
		wpDir := filepath.Join(ctx.WorkspaceDir, "wallpapers")
		entries, err := os.ReadDir(wpDir)
		if err != nil || len(entries) == 0 {
			// No wallpapers yet — return empty array (valid JSON)
			w.Write([]byte("[]"))
			return
		}
		var names []string
		for _, e := range entries {
			if !e.IsDir() {
				names = append(names, e.Name())
			}
		}
		json.NewEncoder(w).Encode(names)
	})

	// SSE endpoint — keeps a persistent connection open for push events
	mux.HandleFunc("/api/events/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}
		// Send a heartbeat every 30s to keep the connection alive
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		// Send initial connected event
		fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"ok\"}\n\n")
		flusher.Flush()
		for {
			select {
			case <-r.Context().Done():
				return
			case t := <-ticker.C:
				fmt.Fprintf(w, "event: heartbeat\ndata: {\"ts\":%d}\n\n", t.Unix())
				flusher.Flush()
			}
		}
	})

	mux.HandleFunc("/api/system/reset-engine", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		go func() {
			notifier.Info("Reset Engine", "Shutting down...")
			time.Sleep(500 * time.Millisecond)
			resetBat := filepath.Join(ctx.Cwd, "internal", "reset_system.bat")
			cmd := exec.Command("cmd.exe", "/c", "start", "", resetBat)
			cmd.Dir = ctx.Cwd
			cmd.Start()
			os.Exit(0)
		}()
	})

	mux.HandleFunc("/api/system/shutdown", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		go func() {
			time.Sleep(500 * time.Millisecond)
			database.StopMariaDB()
			database.StopPHPMyAdmin()
			mailpit.Instance.Stop()
			server.Instance.StopAll()
			os.Exit(0)
		}()
	})

	mux.HandleFunc("/api/engine/performance", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		
		coreMB := int64(m.Alloc / 1024 / 1024)
		if coreMB < 1 {
			coreMB = 5
		}

		osTotal, osUsed := getSystemMemory()
		cpuPercent := getCPULoad()

		mariaMB := getProcessMemoryByName("mariadbd.exe") + getProcessMemoryByName("mysqld.exe")
		redisMB := getProcessMemoryByName("redis-server.exe")
		mailpitMB := getProcessMemoryByName("mailpit.exe")
		
		activeProcs := server.Instance.GetActiveProcesses()
		var projectList []map[string]interface{}
		var projectsTotalRAM int64 = 0

		for _, p := range activeProcs {
			pRAM := getProcessMemoryUsage(p.PID)
			projectsTotalRAM += pRAM
			projectList = append(projectList, map[string]interface{}{
				"name":   p.ProjectName,
				"type":   p.Type,
				"pid":    p.PID,
				"port":   p.Port,
				"ram_mb": pRAM,
			})
		}

		vcoTotalRAM := coreMB + mariaMB + redisMB + mailpitMB + projectsTotalRAM

		json.NewEncoder(w).Encode(map[string]interface{}{
			"os_total_ram_mb":       osTotal,
			"os_used_ram_mb":        osUsed,
			"vcopanel_core_ram_mb":  coreMB,
			"vcopanel_total_ram_mb": vcoTotalRAM,
			"cpu_load_percent":      cpuPercent,
			"status":                "Optimal",
			"maria_ram_mb":          mariaMB,
			"redis_ram_mb":          redisMB,
			"mailpit_ram_mb":        mailpitMB,
			"projects":              projectList,
		})
	})
}

func getSystemMemory() (totalMB int64, usedMB int64) {
	totalMB = 16384
	usedMB = 8192

	cmdTotal := exec.Command("wmic", "computersystem", "get", "TotalPhysicalMemory")
	outTotal, err := cmdTotal.Output()
	if err == nil {
		lines := strings.Split(string(outTotal), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.Contains(line, "TotalPhysicalMemory") {
				continue
			}
			var bytes int64
			if _, errScan := fmt.Sscanf(line, "%d", &bytes); errScan == nil {
				totalMB = bytes / 1024 / 1024
				break
			}
		}
	}

	cmdFree := exec.Command("wmic", "OS", "get", "FreePhysicalMemory")
	outFree, err := cmdFree.Output()
	if err == nil {
		lines := strings.Split(string(outFree), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.Contains(line, "FreePhysicalMemory") {
				continue
			}
			var kb int64
			if _, errScan := fmt.Sscanf(line, "%d", &kb); errScan == nil {
				freeMB := kb / 1024
				usedMB = totalMB - freeMB
				break
			}
		}
	}
	return totalMB, usedMB
}

func getCPULoad() int {
	cmd := exec.Command("wmic", "cpu", "get", "LoadPercentage")
	out, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.Contains(line, "LoadPercentage") {
				continue
			}
			var pct int
			if _, errScan := fmt.Sscanf(line, "%d", &pct); errScan == nil {
				return pct
			}
		}
	}
	return 5
}

func getProcessMemoryUsage(pid int) int64 {
	cmd := exec.Command("wmic", "process", "where", fmt.Sprintf("processid=%d", pid), "get", "WorkingSetSize")
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "WorkingSetSize") {
			continue
		}
		var bytes int64
		if _, errScan := fmt.Sscanf(line, "%d", &bytes); errScan == nil {
			return bytes / 1024 / 1024
		}
	}
	return 0
}

func getProcessMemoryByName(procName string) int64 {
	cmd := exec.Command("wmic", "process", "where", "name='"+procName+"'", "get", "WorkingSetSize")
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	lines := strings.Split(string(out), "\n")
	var total int64
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "WorkingSetSize") {
			continue
		}
		var bytes int64
		if _, errScan := fmt.Sscanf(line, "%d", &bytes); errScan == nil {
			total += bytes / 1024 / 1024
		}
	}
	return total
}
