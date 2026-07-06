package notifier

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type Toast struct {
	ID        string `json:"id"`
	Type      string `json:"type"`       // "success", "error", "warning", "info"
	Title     string `json:"title"`
	Message   string `json:"message"`
	CreatedAt int64  `json:"created_at"` // Unix timestamp in milliseconds
	Duration  int    `json:"duration"`   // Display duration in milliseconds (default 5000)
}

type Manager struct {
	mu     sync.Mutex
	toasts []Toast
}

var Global = &Manager{
	toasts: make([]Toast, 0),
}

func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (m *Manager) Push(toastType, title, message string, durationMs int) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if durationMs <= 0 {
		durationMs = 5000
	}

	t := Toast{
		ID:        generateID(),
		Type:      toastType,
		Title:     title,
		Message:   message,
		CreatedAt: time.Now().UnixNano() / int64(time.Millisecond),
		Duration:  durationMs,
	}

	m.toasts = append(m.toasts, t)
	// Keep only the latest 50 toasts in memory to prevent memory leaks
	if len(m.toasts) > 50 {
		m.toasts = m.toasts[len(m.toasts)-50:]
	}
	return t.ID
}

// FetchRecent returns toasts created after 'sinceMs' timestamp
func (m *Manager) FetchRecent(sinceMs int64) []Toast {
	m.mu.Lock()
	defer m.mu.Unlock()

	var result []Toast
	for _, t := range m.toasts {
		if t.CreatedAt > sinceMs {
			result = append(result, t)
		}
	}
	if result == nil {
		result = []Toast{}
	}
	return result
}

// Clear removes all stored toasts
func (m *Manager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.toasts = make([]Toast, 0)
}

// Global convenience wrappers
func Success(title, message string) string {
	return Global.Push("success", title, message, 5000)
}

func Error(title, message string) string {
	return Global.Push("error", title, message, 7000)
}

func Warning(title, message string) string {
	return Global.Push("warning", title, message, 6000)
}

func Info(title, message string) string {
	return Global.Push("info", title, message, 4500)
}
