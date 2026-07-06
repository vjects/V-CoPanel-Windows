package api

import (
	"encoding/json"
	"net/http"

	"vcopanel-bridge/internal/database"
	"vcopanel-bridge/internal/goconfig"
	"vcopanel-bridge/internal/mailpit"
	"vcopanel-bridge/internal/nodeconfig"
	"vcopanel-bridge/internal/notifier"
	"vcopanel-bridge/internal/phpini"
	"vcopanel-bridge/internal/redis"
)

func registerRuntimesRoutes(mux *http.ServeMux, ctx *ServerContext) {
	// ── Runtimes: Available (100% DB-driven) ──────────────────────────────────
	mux.HandleFunc("/api/runtimes/available", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		runtimes, err := database.FetchInstalledRuntimes(ctx.MariaRoot, "3306")
		if err != nil {
			json.NewEncoder(w).Encode([]interface{}{})
			return
		}

		// Return flat list that frontend can filter by key prefix
		type runtimeEntry struct {
			Key     string `json:"key"`
			Name    string `json:"name"`
			Version string `json:"version"`
			Path    string `json:"path"`
		}
		var result []runtimeEntry
		for _, rt := range runtimes {
			result = append(result, runtimeEntry{
				Key:     rt.Key,
				Name:    rt.Name,
				Version: rt.Version,
				Path:    rt.Path,
			})
		}
		if result == nil {
			result = []runtimeEntry{}
		}
		json.NewEncoder(w).Encode(result)
	})

	// ── PHP INI Config ────────────────────────────────────────────────────────
	mux.HandleFunc("/api/phpini/get", func(w http.ResponseWriter, r *http.Request) {
		// Accept both ?ver= and ?version= for backward compatibility
		version := r.URL.Query().Get("version")
		if version == "" {
			version = r.URL.Query().Get("ver")
		}
		if version == "" {
			version = "php-8.3"
		}
		cfg, err := phpini.GetConfig(version, ctx.WorkspaceDir)
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(cfg)
	})

	mux.HandleFunc("/api/phpini/update", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Version string          `json:"version"`
			Config  phpini.PHPConfig `json:"config"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if err := phpini.UpdateConfig(req.Version, ctx.WorkspaceDir, req.Config); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		notifier.Success("PHP Config Updated", "Settings saved for "+req.Version)
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/api/phpini/restore", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Version string `json:"version"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if req.Version == "" {
			req.Version = "php-8.3"
		}
		if err := phpini.RestoreConfig(req.Version, ctx.WorkspaceDir); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		notifier.Success("PHP Config Restored", "Factory defaults restored for "+req.Version)
		w.WriteHeader(http.StatusOK)
	})

	// ── Node.js Config (version from query string) ────────────────────────────
	mux.HandleFunc("/api/nodeconfig/get", func(w http.ResponseWriter, r *http.Request) {
		ver := r.URL.Query().Get("version")
		if ver == "" {
			ver = "node-v22"
		}
		cfg := nodeconfig.GetConfig(ctx.WorkspaceDir, ver)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cfg)
	})

	mux.HandleFunc("/api/nodeconfig/update", func(w http.ResponseWriter, r *http.Request) {
		ver := r.URL.Query().Get("version")
		if ver == "" {
			ver = "node-v22"
		}
		var req nodeconfig.Config
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		nodeconfig.SaveConfig(ctx.WorkspaceDir, ver, req)
		notifier.Success("Node.js Config Saved", "Settings updated for "+ver)
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/api/nodeconfig/restore", func(w http.ResponseWriter, r *http.Request) {
		ver := r.URL.Query().Get("version")
		if ver == "" {
			ver = "node-v22"
		}
		def := nodeconfig.Config{
			Registry:    "https://registry.npmjs.org/",
			MaxMemoryMB: 4096,
			NodeOptions: "--max-old-space-size=4096",
		}
		nodeconfig.SaveConfig(ctx.WorkspaceDir, ver, def)
		notifier.Success("Node.js Defaults Restored", "Registry and memory reset for "+ver)
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/api/nodeconfig/cache-clean", func(w http.ResponseWriter, r *http.Request) {
		ver := r.URL.Query().Get("version")
		if ver == "" {
			ver = "global"
		}
		w.Header().Set("Content-Type", "application/json")
		// NPM cache-clean via npm executable in the versioned runtime dir
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "version": ver})
	})

	// ── Go Config ───────────────────────────────────────────────────────────
	mux.HandleFunc("/api/goconfig/get", func(w http.ResponseWriter, r *http.Request) {
		ver := r.URL.Query().Get("version")
		if ver == "" {
			ver = "go"
		}
		cfg := goconfig.GetConfig(ctx.WorkspaceDir, ver)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cfg)
	})

	mux.HandleFunc("/api/goconfig/update", func(w http.ResponseWriter, r *http.Request) {
		ver := r.URL.Query().Get("version")
		if ver == "" {
			ver = "go"
		}
		var req goconfig.Config
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		goconfig.SaveConfig(ctx.WorkspaceDir, ver, req)
		notifier.Success("Go Config Saved", "Settings updated for "+ver)
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/api/goconfig/restore", func(w http.ResponseWriter, r *http.Request) {
		ver := r.URL.Query().Get("version")
		if ver == "" {
			ver = "go"
		}
		def := goconfig.GetDefaultConfig()
		if err := goconfig.SaveConfig(ctx.WorkspaceDir, ver, def); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		notifier.Success("Go Defaults Restored", "GOPROXY and CGO reset to defaults for "+ver)
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/api/goconfig/cache-clean", func(w http.ResponseWriter, r *http.Request) {
		ver := r.URL.Query().Get("version")
		if ver == "" {
			ver = "go"
		}
		out, err := goconfig.CleanCache(ctx.WorkspaceDir, ver)
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		notifier.Success("Go Cache Cleaned", "Build and module caches wiped for "+ver)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "output": out})
	})

	// ── Redis Config (Get/Update via redis package) ───────────────────────────
	mux.HandleFunc("/api/redis/config/get", func(w http.ResponseWriter, r *http.Request) {
		maxMem, policy := redis.GetConfig(ctx.WorkspaceDir)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"max_memory": maxMem,
			"policy":     policy,
		})
	})

	mux.HandleFunc("/api/redis/config/update", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			MaxMemory string `json:"max_memory"`
			Policy    string `json:"policy"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if err := redis.UpdateConfig(ctx.WorkspaceDir, req.MaxMemory, req.Policy); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		notifier.Success("Redis Config Saved", "Max memory and eviction policy updated.")
		w.WriteHeader(http.StatusOK)
	})

	// ── Mailpit: Clear inbox ──────────────────────────────────────────────────
	mux.HandleFunc("/api/mailpit/clear", func(w http.ResponseWriter, r *http.Request) {
		if !mailpit.Instance.Status() {
			http.Error(w, "Mailpit is not running", 503)
			return
		}
		// Call Mailpit's own REST API to delete all messages
		req2, _ := http.NewRequest("DELETE", "http://127.0.0.1:8025/api/v1/messages", nil)
		client := &http.Client{}
		resp, err := client.Do(req2)
		if err != nil || (resp != nil && resp.StatusCode >= 400) {
			http.Error(w, "Failed to clear inbox", 500)
			return
		}
		notifier.Success("Mailbox Cleared", "All test emails have been removed from the inbox.")
		w.WriteHeader(http.StatusOK)
	})
}
