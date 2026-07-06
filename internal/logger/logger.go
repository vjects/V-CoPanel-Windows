package logger

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

var ansiRegex = regexp.MustCompile(`[\x1b\x9b][[()#;?]*(?:[0-9]{1,4}(?:;[0-9]{0,4})*)?[0-9A-ORZcf-nqry=><]`)

type RingBuffer struct {
	mu    sync.Mutex
	lines []string
}

var Console = &RingBuffer{
	lines: make([]string, 0, 1000),
}

func (rb *RingBuffer) Append(line string) {
	cleanLine := ansiRegex.ReplaceAllString(line, "")
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.lines = append(rb.lines, cleanLine)
	if len(rb.lines) > 800 {
		rb.lines = rb.lines[len(rb.lines)-800:]
	}
}

func (rb *RingBuffer) GetAll() string {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return strings.Join(rb.lines, "\n")
}

func (rb *RingBuffer) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.lines = rb.lines[:0]
}

func InitSystemConsoleCapture() {
	r, w, err := os.Pipe()
	if err != nil {
		return
	}

	origStdout := os.Stdout
	_ = os.Stderr

	os.Stdout = w
	os.Stderr = w

	go func() {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Fprintln(origStdout, line)
			Console.Append(line)
		}
	}()

	// Print initial header directly
	Log("Real-time terminal log interception initialized.")
}

// Log prints a formatted message to terminal and ring buffer with [PID - process_name] prefix.
func Log(format string, args ...interface{}) {
	pid := os.Getpid()
	procName := filepath.Base(os.Args[0])
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("[%d - %s] %s\n", pid, procName, msg)
}

// LogProc prints a formatted message for a specific child process PID and name.
func LogProc(pid int, procName string, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("[%d - %s] %s\n", pid, procName, msg)
}
