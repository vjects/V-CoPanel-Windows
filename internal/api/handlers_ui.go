package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"vcopanel-bridge/internal/database"
	"vcopanel-bridge/internal/mailpit"
	"vcopanel-bridge/internal/redis"
	"vcopanel-bridge/internal/server"
)

// TopbarContext holds all data that the Topbar UI needs to render dynamically.
type TopbarContext struct {
	// Engine identity (read from system_infrastructure)
	EngineName    string `json:"engine_name"`
	EngineVersion string `json:"engine_version"`

	// Live service status (Check & Run style — real-time from process managers)
	Services []TopbarServiceStatus `json:"services"`

	// Project summary counts
	TotalProjects   int `json:"total_projects"`
	RunningProjects int `json:"running_projects"`
}

type TopbarServiceStatus struct {
	Key     string `json:"key"`
	Name    string `json:"name"`
	Running bool   `json:"running"`
	Port    string `json:"port"`
}

func registerUIRoutes(mux *http.ServeMux, ctx *ServerContext) {
	mux.HandleFunc("/api/ui/topbar-context", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// ── Engine identity from DB ───────────────────────────────────────────
		engineName := "V-CoPanel Bridge"
		engineVer := "v2.0.1"

		recs, _ := database.FetchInfrastructureRecords(ctx.MariaRoot, "3306")
		for _, rec := range recs {
			if rec.Key == "vcopanel_engine" {
				engineVer = rec.Version
				engineName = rec.Name
			}
		}

		// ── Service statuses (Check & Run — live) ─────────────────────────────
		mariaRunning := database.IsMariaDBRunning()
		redisStatus := redis.GetStatus(ctx.WorkspaceDir)
		mailpitRunning := mailpit.Instance.Status()
		pmaRunning := database.IsPHPMyAdminRunning()

		portsMap := map[string]string{
			"mariadb":    "3306",
			"redis":      "6379",
			"mailpit":    "8025",
			"phpmyadmin": "8881",
		}
		if dbServices, err := database.FetchSystemServices(ctx.MariaRoot, "3306"); err == nil {
			for _, srv := range dbServices {
				portsMap[srv.Key] = strconv.Itoa(srv.Port)
			}
		}

		services := []TopbarServiceStatus{
			{Key: "mariadb", Name: "MariaDB", Running: mariaRunning, Port: portsMap["mariadb"]},
			{Key: "redis", Name: "Redis", Running: redisStatus.Running, Port: portsMap["redis"]},
			{Key: "mailpit", Name: "Mailpit", Running: mailpitRunning, Port: portsMap["mailpit"]},
			{Key: "phpmyadmin", Name: "phpMyAdmin", Running: pmaRunning, Port: portsMap["phpmyadmin"]},
		}

		// ── Project statistics from DB ────────────────────────────────────────
		projects, _ := database.FetchAllProjectsFromDB(ctx.MariaRoot, "3306")
		totalProjects := len(projects)
		runningCount := 0
		for _, p := range projects {
			if server.Instance.IsServeRunning(p.Path) {
				runningCount++
			}
		}

		json.NewEncoder(w).Encode(TopbarContext{
			EngineName:      engineName,
			EngineVersion:   engineVer,
			Services:        services,
			TotalProjects:   totalProjects,
			RunningProjects: runningCount,
		})
	})

	mux.HandleFunc("/api/ui/sidebar-context", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		runtimes, _ := database.FetchInstalledRuntimes(ctx.MariaRoot, "3306")
		
		hasPHP := false
		hasNode := false
		hasGo := false

		for _, rt := range runtimes {
			if len(rt.Key) >= 3 && rt.Key[:3] == "php" {
				hasPHP = true
			}
			if len(rt.Key) >= 4 && rt.Key[:4] == "node" {
				hasNode = true
			}
			if rt.Key == "go" || (len(rt.Key) >= 2 && rt.Key[:2] == "go") {
				hasGo = true
			}
		}

		type SidebarItem struct {
			TabID  string `json:"tab_id"`
			Name   string `json:"name"`
			Icon   string `json:"icon"`
			Title  string `json:"title"`
		}

		var runtimeItems []SidebarItem
		if hasPHP {
			runtimeItems = append(runtimeItems, SidebarItem{TabID: "tab-php_config", Name: "PHP", Icon: "data_object", Title: "PHP Runtime Config"})
		}
		if hasNode {
			runtimeItems = append(runtimeItems, SidebarItem{TabID: "tab-node_config", Name: "Node.js", Icon: "javascript", Title: "Node.js Runtime Config"})
		}
		if hasGo {
			runtimeItems = append(runtimeItems, SidebarItem{TabID: "tab-go_config", Name: "Go", Icon: "developer_board", Title: "Go Runtime Config"})
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"runtimes": runtimeItems,
		})
	})
}
