package mailpit

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"vcopanel-bridge/internal/extractor"
	"vcopanel-bridge/internal/logger"
)

type Manager struct {
	mu        sync.Mutex
	isRunning bool
	cmd       *exec.Cmd
}

var Instance = &Manager{}

// Start extracts Mailpit (if needed) and launches it with elevated/admin request or directly.
func (m *Manager) Start(assetsDir, workspaceDir string) error {
	m.mu.Lock()
	if m.isRunning {
		m.mu.Unlock()
		return nil
	}
	m.mu.Unlock()

	mailpitTargetDir := filepath.Join(workspaceDir, "shared-services", "mailpit")
	mailpitDir := filepath.Join(assetsDir, "mailpit")
	var mailpitZip string
	if entries, err := os.ReadDir(mailpitDir); err == nil {
		for _, e := range entries {
			if strings.HasSuffix(strings.ToLower(e.Name()), ".zip") {
				mailpitZip = filepath.Join(mailpitDir, e.Name())
				break
			}
		}
	}
	if mailpitZip == "" {
		mailpitZip = filepath.Join(assetsDir, "mailpit-windows-amd64.zip")
	}

	if _, err := extractor.EnsureAsset(mailpitZip, mailpitTargetDir); err != nil {
		return fmt.Errorf("failed to extract mailpit: %w", err)
	}

	exePath := filepath.Join(mailpitTargetDir, "mailpit.exe")
	if _, err := os.Stat(exePath); err != nil {
		return fmt.Errorf("mailpit.exe not found at %s", exePath)
	}

	// Ensure no orphaned Mailpit process is blocking ports 8025/3025 before starting
	exec.Command("taskkill", "/F", "/IM", "mailpit.exe").Run()

	cmd := exec.Command(exePath, "--smtp", "0.0.0.0:3025", "--listen", "0.0.0.0:8025")
	stdoutPipe, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		logger.Log("Mailpit start failed, requesting UAC elevated prompt...")
		psCmd := exec.Command("powershell", "-Command", fmt.Sprintf("Start-Process -FilePath '%s' -ArgumentList '--smtp 0.0.0.0:3025 --listen 0.0.0.0:8025' -Verb RunAs", exePath))
		if errPs := psCmd.Run(); errPs != nil {
			return fmt.Errorf("failed to launch Mailpit via UAC prompt: %w", errPs)
		}
	} else {
		m.mu.Lock()
		m.cmd = cmd
		m.mu.Unlock()

		pid := cmd.Process.Pid
		go func() {
			scanner := bufio.NewScanner(stdoutPipe)
			for scanner.Scan() {
				logger.LogProc(pid, "mailpit.exe", "%s", scanner.Text())
			}
		}()
		go func() {
			scanner := bufio.NewScanner(stderrPipe)
			for scanner.Scan() {
				logger.LogProc(pid, "mailpit.exe", "%s", scanner.Text())
			}
		}()

		go func(c *exec.Cmd) {
			c.Wait()
			m.mu.Lock()
			if m.cmd == c {
				m.isRunning = false
				m.cmd = nil
			}
			m.mu.Unlock()
		}(cmd)
	}

	m.mu.Lock()
	m.isRunning = true
	m.mu.Unlock()

	logger.Log("Mailpit Mailbox Server : Active on http://127.0.0.1:8025 (SMTP: 3025, PID: %d)", cmd.Process.Pid)
	return nil
}

// Stop terminates Mailpit process.
func (m *Manager) Stop() error {
	logger.Log("Terminating Mailpit server...")
	m.mu.Lock()
	if m.cmd != nil && m.cmd.Process != nil {
		exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", m.cmd.Process.Pid)).Run()
	}
	m.cmd = nil
	m.isRunning = false
	m.mu.Unlock()

	exec.Command("taskkill", "/F", "/IM", "mailpit.exe").Run()
	logger.Log("Mailpit server shut down successfully.")
	return nil
}

// Status returns whether Mailpit is running.
func (m *Manager) Status() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.isRunning
}
