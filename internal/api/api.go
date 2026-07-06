package api

import (
	"crypto/md5"
	"encoding/hex"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"vcopanel-bridge/internal/process"
)

type ServerContext struct {
	WorkspaceDir   string
	AssetsDir      string
	MariaRoot      string
	Cwd            string
	ProcessManager *process.Manager
}

func RegisterAllRoutes(mux *http.ServeMux, ctx *ServerContext) {
	registerSystemRoutes(mux, ctx)
	registerProjectsRoutes(mux, ctx)
	registerServicesRoutes(mux, ctx)
	registerRuntimesRoutes(mux, ctx)
	registerTasksRoutes(mux, ctx)
	registerUIRoutes(mux, ctx)
}

func generateUUID(path string) string {
	hasher := md5.New()
	hasher.Write([]byte(strings.ToLower(filepath.ToSlash(filepath.Clean(path)))))
	return hex.EncodeToString(hasher.Sum(nil))
}

func openSystemBrowser(targetURL string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", targetURL)
	case "darwin":
		cmd = exec.Command("open", targetURL)
	default:
		cmd = exec.Command("xdg-open", targetURL)
	}
	if cmd != nil {
		cmd.Start()
	}
}
