package api

import (
	"encoding/json"
	"net/http"

	"vcopanel-bridge/internal/mesh"
	"vcopanel-bridge/internal/projectmanager"
	"vcopanel-bridge/internal/server"
)

func registerTasksRoutes(mux *http.ServeMux, ctx *ServerContext) {
	mux.HandleFunc("/api/serve/start", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Path       string `json:"path"`
			PHPVersion string `json:"php_version"`
			Port       string `json:"port"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		
		server.Instance.StartServe(req.Path, req.Port, req.PHPVersion, ctx.WorkspaceDir, "native", "", "", "serve")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/api/serve/stop", func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Path string }
		json.NewDecoder(r.Body).Decode(&req)
		server.Instance.StopServe(req.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/api/serve/status", func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Path string }
		json.NewDecoder(r.Body).Decode(&req)
		status := server.Instance.IsServeRunning(req.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"running": status})
	})

	mux.HandleFunc("/api/queue/start", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Path       string `json:"path"`
			PHPVersion string `json:"php_version"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		server.Instance.StartQueue(req.Path, req.PHPVersion, ctx.WorkspaceDir)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/api/queue/stop", func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Path string }
		json.NewDecoder(r.Body).Decode(&req)
		server.Instance.StopQueue(req.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/api/queue/status", func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Path string }
		json.NewDecoder(r.Body).Decode(&req)
		status := server.Instance.IsQueueRunning(req.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"running": status})
	})

	mux.HandleFunc("/api/project/go", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Path    string `json:"path"`
			Command string `json:"command"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		out, err := projectmanager.ExecGoCommand(req.Path, req.Command)
		w.Header().Set("Content-Type", "application/json")
		errStr := ""
		if err != nil {
			errStr = err.Error()
		}
		json.NewEncoder(w).Encode(map[string]string{"output": out, "error": errStr})
	})

	mux.HandleFunc("/api/mesh/discover", func(w http.ResponseWriter, r *http.Request) {
		service := r.URL.Query().Get("service")
		port := mesh.GlobalRegistry.Discover(service)
		w.Header().Set("Content-Type", "application/json")
		if port != "" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"found":   true,
				"service": service,
				"port":    port,
				"url":     "http://127.0.0.1:" + port,
			})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"found":   false,
				"service": service,
			})
		}
	})
}
