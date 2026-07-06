package api

import (
	"encoding/json"
	"net/http"

	"vcopanel-bridge/internal/database"
	"vcopanel-bridge/internal/mailpit"
	"vcopanel-bridge/internal/notifier"
	"vcopanel-bridge/internal/redis"
)

func registerServicesRoutes(mux *http.ServeMux, ctx *ServerContext) {
	// phpMyAdmin APIs
	mux.HandleFunc("/api/phpmyadmin/start", func(w http.ResponseWriter, r *http.Request) {
		if database.IsPHPMyAdminRunning() {
			notifier.Info("phpMyAdmin Already Running", "Database administration UI is already active at http://127.0.0.1:8881")
			database.SaveServiceState(ctx.MariaRoot, "3306", "phpmyadmin", "phpMyAdmin Universal GUI", 8881, "http://127.0.0.1:8881", "running")
			w.WriteHeader(http.StatusOK)
			return
		}
		if err := database.StartPHPMyAdmin(ctx.WorkspaceDir); err != nil {
			notifier.Error("phpMyAdmin Failed", err.Error())
			http.Error(w, err.Error(), 500)
			return
		}
		database.SaveServiceState(ctx.MariaRoot, "3306", "phpmyadmin", "phpMyAdmin Universal GUI", 8881, "http://127.0.0.1:8881", "running")
		notifier.Success("phpMyAdmin Started", "Database administration UI active at http://127.0.0.1:8881")
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/api/phpmyadmin/stop", func(w http.ResponseWriter, r *http.Request) {
		database.StopPHPMyAdmin()
		database.SaveServiceState(ctx.MariaRoot, "3306", "phpmyadmin", "phpMyAdmin Universal GUI", 8881, "http://127.0.0.1:8881", "stopped")
		notifier.Info("phpMyAdmin Stopped", "Database administration server shut down.")
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/api/phpmyadmin/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"running":         database.IsPHPMyAdminRunning(),
			"mariadb_running": database.IsMariaDBRunning(),
			"creds":           database.Creds,
		})
	})

	// MariaDB APIs
	mux.HandleFunc("/api/system/infrastructure", func(w http.ResponseWriter, r *http.Request) {
		recs, err := database.FetchInfrastructureRecords(ctx.MariaRoot, "3306")
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error(), "records": []interface{}{}})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "records": recs})
	})

	mux.HandleFunc("/api/system/services", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		services, err := database.FetchSystemServices(ctx.MariaRoot, "3306")
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error(), "services": []interface{}{}})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "services": services})
	})

	mux.HandleFunc("/api/mariadb/config/get", func(w http.ResponseWriter, r *http.Request) {
		cfg := database.GetMariaDBConfig(ctx.WorkspaceDir)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cfg)
	})

	mux.HandleFunc("/api/mariadb/config/update", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			BufferPool string `json:"buffer_pool"`
			MaxConn    string `json:"max_connections"`
			Charset    string `json:"charset"`
			Collation  string `json:"collation"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if err := database.UpdateMariaDBConfig(ctx.WorkspaceDir, req.BufferPool, req.MaxConn, req.Charset, req.Collation); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		notifier.Success("MariaDB Config Saved", "Database engine tuning parameters updated successfully.")
		w.WriteHeader(http.StatusOK)
	})

	// Mailpit APIs
	mux.HandleFunc("/api/mailpit/start", func(w http.ResponseWriter, r *http.Request) {
		if mailpit.Instance.Status() {
			notifier.Info("Mailpit Already Running", "Testing email server is already active on port 8025")
			database.SaveServiceState(ctx.MariaRoot, "3306", "mailpit", "Mailpit SMTP & Webmail Server", 8025, "http://127.0.0.1:8025", "running")
			w.WriteHeader(http.StatusOK)
			return
		}
		if err := mailpit.Instance.Start(ctx.AssetsDir, ctx.WorkspaceDir); err != nil {
			notifier.Error("Mailpit Failed", err.Error())
			http.Error(w, err.Error(), 500)
			return
		}
		database.SaveServiceState(ctx.MariaRoot, "3306", "mailpit", "Mailpit SMTP & Webmail Server", 8025, "http://127.0.0.1:8025", "running")
		notifier.Success("Mailpit Started", "Testing email server active on port 8025")
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/api/mailpit/stop", func(w http.ResponseWriter, r *http.Request) {
		mailpit.Instance.Stop()
		database.SaveServiceState(ctx.MariaRoot, "3306", "mailpit", "Mailpit SMTP & Webmail Server", 8025, "http://127.0.0.1:8025", "stopped")
		notifier.Info("Mailpit Stopped", "Testing email mailbox server shut down.")
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/api/mailpit/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"running": mailpit.Instance.Status()})
	})

	// Redis APIs
	mux.HandleFunc("/api/redis/start", func(w http.ResponseWriter, r *http.Request) {
		if redis.GetStatus(ctx.WorkspaceDir).Running {
			notifier.Info("Redis Already Running", "In-memory cache service is already active on port 6379.")
			database.SaveServiceState(ctx.MariaRoot, "3306", "redis", "Redis Portable Cache", 6379, "127.0.0.1:6379", "running")
		} else {
			redis.Start(ctx.WorkspaceDir)
			database.SaveServiceState(ctx.MariaRoot, "3306", "redis", "Redis Portable Cache", 6379, "127.0.0.1:6379", "running")
			notifier.Success("Redis Started", "In-memory cache service is now active on port 6379.")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/api/redis/stop", func(w http.ResponseWriter, r *http.Request) {
		redis.Stop(ctx.WorkspaceDir)
		database.SaveServiceState(ctx.MariaRoot, "3306", "redis", "Redis Portable Cache", 6379, "127.0.0.1:6379", "stopped")
		notifier.Info("Redis Stopped", "Portable Redis cache server stopped.")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/api/redis/flush", func(w http.ResponseWriter, r *http.Request) {
		redis.Flush(ctx.WorkspaceDir)
		notifier.Success("Redis Flushed", "All keys cleared.")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/api/redis/status", func(w http.ResponseWriter, r *http.Request) {
		status := redis.GetStatus(ctx.WorkspaceDir)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	})
}
