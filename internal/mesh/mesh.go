package mesh

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"strings"
	"sync"

	"vcopanel-bridge/internal/logger"
	"vcopanel-bridge/internal/notifier"
)

type Registry struct {
	mu       sync.RWMutex
	services map[string]string // map[projectName]port
}

var GlobalRegistry = &Registry{
	services: make(map[string]string),
}

func (r *Registry) Register(name, port string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cleanName := strings.ToLower(name)
	r.services[cleanName] = port
}

func (r *Registry) Deregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cleanName := strings.ToLower(name)
	delete(r.services, cleanName)
}

func (r *Registry) Discover(name string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cleanName := strings.ToLower(name)
	return r.services[cleanName]
}

// StartProxy starts the internal reverse proxy on port 38000
func StartProxy() {
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			host := req.Host
			if idx := strings.Index(host, ":"); idx != -1 {
				host = host[:idx]
			}
			
			if strings.HasSuffix(host, ".vcopanel.internal") {
				name := strings.TrimSuffix(host, ".vcopanel.internal")
				port := GlobalRegistry.Discover(name)
				if port != "" {
					req.URL.Scheme = "http"
					req.URL.Host = "127.0.0.1:" + port
				} else {
					req.URL.Scheme = "http"
					req.URL.Host = "127.0.0.1:65535"
				}
			} else {
				req.URL.Scheme = "http"
				req.URL.Host = "127.0.0.1:65535"
			}
		},
	}

	server := &http.Server{
		Addr:    "127.0.0.1:38000",
		Handler: proxy,
	}

	go func() {
		logger.Log("Starting internal reverse proxy on 127.0.0.1:38000")
		if err := server.ListenAndServe(); err != nil {
			notifier.Error("Mesh Gateway Error", fmt.Sprintf("Failed to start reverse proxy: %v", err))
		}
	}()
}
