package scaffolder

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"vcopanel-bridge/internal/envmanager"
	"vcopanel-bridge/internal/shims"
)

// CreateProject scaffolds a basic starter application for backward compatibility.
func CreateProject(baseDir, name, stack string) (string, error) {
	return CreateAdvancedProject(baseDir, name, stack, "8000", "app_"+stringsReplace(strings.ToLower(name), "-", "_"), "", "", "utf8mb4_unicode_ci")
}

// CreateAdvancedProject scaffolds a new project with specific port, runtime versions, and database settings.
func CreateAdvancedProject(baseDir, name, stack, port, dbName, phpVer, nodeVer, collation string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("project name cannot be empty")
	}
	if port == "" {
		port = "8000"
	}
	if dbName == "" {
		dbName = "app_" + stringsReplace(strings.ToLower(name), "-", "_")
	}

	projectsDir := baseDir
	if filepath.Base(baseDir) == "workspace" {
		projectsDir = filepath.Join(baseDir, "projects")
	}
	if err := os.MkdirAll(projectsDir, 0755); err != nil {
		return "", err
	}

	targetDir := filepath.Join(projectsDir, name)
	if _, err := os.Stat(targetDir); err == nil {
		return "", fmt.Errorf("project directory already exists: %s", name)
	}
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", err
	}

	var err error
	switch strings.ToLower(stack) {
	case "go":
		err = scaffoldGoAdvanced(targetDir, name, port)
	case "laravel":
		err = scaffoldLaravelAdvanced(targetDir, name, port, dbName)

	case "nextjs", "next":
		err = scaffoldNextJSAdvanced(targetDir, name, port)
	case "vue":
		err = scaffoldVueAdvanced(targetDir, name, port)
	case "simple-php", "php":
		err = scaffoldSimplePHPAdvanced(targetDir, name, port, dbName)
	case "nodejs", "node", "express":
		err = scaffoldNodeAdvanced(targetDir, name, port)
	case "sandbox":
		err = scaffoldSandboxAdvanced(targetDir, name, port)
	case "generic":
		err = scaffoldGenericAdvanced(targetDir, name, port)
	default:
		return "", fmt.Errorf("unsupported stack template: %s", stack)
	}

	if err != nil {
		return "", err
	}

	// Write runtime configuration file (.vcopanel-runtime.json and .tools-version)
	runtimeMeta := fmt.Sprintf(`{
  "name": "%s",
  "stack": "%s",
  "port": "%s",
  "php_version": "%s",
  "node_version": "%s",  "db_name": "%s",
  "collation": "%s"
}`, name, stack, port, phpVer, nodeVer, dbName, collation)
	os.WriteFile(filepath.Join(targetDir, ".vcopanel-runtime.json"), []byte(runtimeMeta), 0644)

	if phpVer != "" || nodeVer != "" || stack == "go" {
		shims.SyncToolsVersionBlock(filepath.Join(targetDir, ".tools-version"), phpVer, nodeVer, "", strings.ToLower(stack) == "go")
	}

	return targetDir, nil
}

func stringsReplace(s, old, newStr string) string {
	return strings.ReplaceAll(s, old, newStr)
}

func scaffoldGoAdvanced(dir, name, port string) error {
	goMod := fmt.Sprintf("module %s\n\ngo 1.22\n", name)
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644); err != nil {
		return err
	}

	mainGo := `package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func main() {
	port := "` + port + `"
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"message": "Welcome to Go Starter Project on V-CoPanel Engine!",
			"app":     "` + name + `",
		})
	})

	fmt.Printf("🚀 Go Starter [%s] listening on http://localhost:%s\n", "` + name + `", port)
	http.ListenAndServe(":"+port, nil)
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(mainGo), 0644); err != nil {
		return err
	}

	envVars := map[string]string{
		"APP_NAME": name,
		"PORT":     port,
		"APP_ENV":  "local",
	}
	return envmanager.SyncSandboxBlock(filepath.Join(dir, ".env"), envVars)
}

func scaffoldLaravelAdvanced(dir, name, port, dbName string) error {
	dirs := []string{
		"app/Http/Controllers",
		"bootstrap",
		"config",
		"public",
		"resources/views",
		"routes",
		"storage/logs",
		"storage/app/public",
		"database/migrations",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(dir, d), 0755); err != nil {
			return err
		}
	}

	// composer.json — required for composer install
	composerJson := fmt.Sprintf(`{
  "name": "%s/%s",
  "description": "Laravel 11 Starter scaffolded by V-CoPanel",
  "type": "project",
  "require": {
    "php": "^8.2",
    "laravel/framework": "^11.0"
  },
  "autoload": {
    "psr-4": {
      "App\\\\": "app/",
      "Database\\\\": "database/"
    }
  },
  "scripts": {
    "post-autoload-dump": ["Illuminate\\\\Foundation\\\\ComposerScripts::postAutoloadDump"]
  },
  "minimum-stability": "stable",
  "prefer-stable": true
}`, strings.ToLower(name), strings.ToLower(name))
	if err := os.WriteFile(filepath.Join(dir, "composer.json"), []byte(composerJson), 0644); err != nil {
		return err
	}

	artisan := `#!/usr/bin/env php
<?php
if (isset($argv[1]) && $argv[1] === 'serve') {
    $port = ` + port + `;
    foreach ($argv as $arg) {
        if (strpos($arg, '--port=') === 0) {
            $port = substr($arg, 7);
        }
    }
    echo "Starting Laravel development server: http://127.0.0.1:{$port}\n";
    passthru("php -S 127.0.0.1:{$port} -t public");
} else {
    echo "V-CoPanel Laravel Starter Engine (v11.x)\n";
}
`
	if err := os.WriteFile(filepath.Join(dir, "artisan"), []byte(artisan), 0755); err != nil {
		return err
	}

	indexPhp := `<?php
header('Content-Type: application/json');
echo json_encode([
    'status' => 'success',
    'framework' => 'Laravel 11 Starter',
    'project' => '` + name + `',
    'message' => 'Scaffolded automatically by V-CoPanel Multi-Stack Engine'
]);
`
	if err := os.WriteFile(filepath.Join(dir, "public", "index.php"), []byte(indexPhp), 0644); err != nil {
		return err
	}

	envVars := map[string]string{
		"APP_NAME":         name,
		"APP_ENV":          "local",
		"APP_KEY":          "",
		"APP_DEBUG":        "true",
		"APP_URL":          "http://127.0.0.1:" + port,
		"PORT":             port,
		"DB_CONNECTION":    "mysql",
		"DB_HOST":          "127.0.0.1",
		"DB_PORT":          "3306",
		"DB_DATABASE":      dbName,
		"DB_USERNAME":      "root",
		"DB_PASSWORD":      "",
		"MAIL_MAILER":      "smtp",
		"MAIL_HOST":        "127.0.0.1",
		"MAIL_PORT":        "1025",
		"QUEUE_CONNECTION": "database",
	}
	return envmanager.SyncSandboxBlock(filepath.Join(dir, ".env"), envVars)
}


func scaffoldNextJSAdvanced(dir, name, port string) error {
	pkgJson := fmt.Sprintf(`{
  "name": "%s",
  "version": "0.1.0",
  "private": true,
  "scripts": {
    "dev": "next dev -p %s",
    "build": "next build",
    "start": "next start -p %s"
  },
  "dependencies": {
    "next": "^14.2.0",
    "react": "^18.3.0",
    "react-dom": "^18.3.0"
  }
}`, name, port, port)
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgJson), 0644); err != nil {
		return err
	}

	nextCfg := `/** @type {import('next').NextConfig} */
const nextConfig = {};
module.exports = nextConfig;
`
	os.WriteFile(filepath.Join(dir, "next.config.js"), []byte(nextCfg), 0644)

	appDir := filepath.Join(dir, "app")
	os.MkdirAll(appDir, 0755)
	pageJs := `export default function Home() {
  return (
    <div style={{ fontFamily: 'sans-serif', padding: 40, textAlign: 'center' }}>
      <h1>Welcome to Next.js Starter on V-CoPanel</h1>
      <p>Project: ` + name + `</p>
    </div>
  );
}`
	os.WriteFile(filepath.Join(appDir, "page.js"), []byte(pageJs), 0644)

	envVars := map[string]string{
		"APP_NAME": name,
		"PORT":     port,
	}
	return envmanager.SyncSandboxBlock(filepath.Join(dir, ".env"), envVars)
}

func scaffoldVueAdvanced(dir, name, port string) error {
	pkgJson := fmt.Sprintf(`{
  "name": "%s",
  "version": "0.0.0",
  "scripts": {
    "dev": "vite --port %s",
    "build": "vite build",
    "preview": "vite preview --port %s"
  },
  "dependencies": {
    "vue": "^3.4.0"
  },
  "devDependencies": {
    "@vitejs/plugin-vue": "^5.0.0",
    "vite": "^5.2.0"
  }
}`, name, port, port)
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgJson), 0644); err != nil {
		return err
	}

	srcDir := filepath.Join(dir, "src")
	os.MkdirAll(srcDir, 0755)

	appVue := `<template>
  <div style="font-family: sans-serif; text-align: center; padding: 40px;">
    <h1>Welcome to Vue.js Starter on V-CoPanel</h1>
    <p>Project: ` + name + `</p>
  </div>
</template>`
	os.WriteFile(filepath.Join(srcDir, "App.vue"), []byte(appVue), 0644)

	viteCfg := `import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
})`
	os.WriteFile(filepath.Join(dir, "vite.config.js"), []byte(viteCfg), 0644)

	indexHtml := `<!DOCTYPE html>
<html>
<head><title>` + name + `</title></head>
<body><div id="app"></div></body>
</html>`
	os.WriteFile(filepath.Join(dir, "index.html"), []byte(indexHtml), 0644)

	envVars := map[string]string{
		"VITE_APP_NAME": name,
		"PORT":          port,
	}
	return envmanager.SyncSandboxBlock(filepath.Join(dir, ".env"), envVars)
}

func scaffoldSimplePHPAdvanced(dir, name, port, dbName string) error {
	indexPhp := `<?php
header('Content-Type: application/json');
echo json_encode([
    'status' => 'success',
    'framework' => 'Simple PHP Starter',
    'project' => '` + name + `',
    'port' => ` + port + `,
    'database' => '` + dbName + `'
]);
`
	if err := os.WriteFile(filepath.Join(dir, "index.php"), []byte(indexPhp), 0644); err != nil {
		return err
	}

	envVars := map[string]string{
		"APP_NAME":    name,
		"APP_ENV":     "local",
		"PORT":        port,
		"DB_HOST":     "127.0.0.1",
		"DB_PORT":     "3306",
		"DB_DATABASE": dbName,
		"DB_USERNAME": "root",
		"DB_PASSWORD": "",
	}
	return envmanager.SyncSandboxBlock(filepath.Join(dir, ".env"), envVars)
}

func scaffoldNodeAdvanced(dir, name, port string) error {
	pkgJson := fmt.Sprintf(`{
  "name": "%s",
  "version": "1.0.0",
  "main": "index.js",
  "scripts": {
    "start": "node index.js"
  },
  "dependencies": {
    "express": "^4.19.0"
  }
}`, name)
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgJson), 0644); err != nil {
		return err
	}

	idxJs := `const http = require('http');
const port = process.env.PORT || ` + port + `;
const server = http.createServer((req, res) => {
  res.writeHead(200, { 'Content-Type': 'application/json' });
  res.end(JSON.stringify({ status: 'ok', framework: 'Node.js Starter', project: '` + name + `' }));
});
server.listen(port, () => console.log('🚀 Node.js Starter listening on port ' + port));
`
	os.WriteFile(filepath.Join(dir, "index.js"), []byte(idxJs), 0644)

	envVars := map[string]string{
		"APP_NAME": name,
		"PORT":     port,
	}
	return envmanager.SyncSandboxBlock(filepath.Join(dir, ".env"), envVars)
}

func scaffoldSandboxAdvanced(dir, name, port string) error {
	readme := fmt.Sprintf("# Sandbox Workspace: %s\n\nThis is an empty Sandbox workspace initialized by V-CoPanel.\nYou can use the Quick Starter buttons in the V-CoPanel Sandbox Studio to generate initial files, or copy/clone your code here and click **Scan Directory & Upgrade Stack**.\n", name)
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0644); err != nil {
		return err
	}
	envVars := map[string]string{
		"APP_NAME": name,
		"APP_ENV":  "local",
		"PORT":     port,
	}
	return envmanager.SyncSandboxBlock(filepath.Join(dir, ".env"), envVars)
}

func scaffoldGenericAdvanced(dir, name, port string) error {
	readme := fmt.Sprintf("# Universal Generic Studio: %s\n\nThis custom workspace is managed via the V-CoPanel Universal Studio.\nYou can configure server ports, manage environment variables, and open terminal or code editors directly from the studio dashboard.\n", name)
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0644); err != nil {
		return err
	}
	envVars := map[string]string{
		"APP_NAME": name,
		"APP_ENV":  "local",
		"PORT":     port,
	}
	return envmanager.SyncSandboxBlock(filepath.Join(dir, ".env"), envVars)
}


