package process

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"unsafe"

	"vcopanel-bridge/internal/logger"
)

var (
	kernel32                     = syscall.NewLazyDLL("kernel32.dll")
	procCreateJobObjectW         = kernel32.NewProc("CreateJobObjectW")
	procSetInformationJobObject  = kernel32.NewProc("SetInformationJobObject")
	procAssignProcessToJobObject = kernel32.NewProc("AssignProcessToJobObject")

	globalJobHandle syscall.Handle
	once          sync.Once
)

type JOBOBJECT_EXTENDED_LIMIT_INFORMATION struct {
	BasicLimitInformation JOBOBJECT_BASIC_LIMIT_INFORMATION
	IoInfo                IO_COUNTERS
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

type JOBOBJECT_BASIC_LIMIT_INFORMATION struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

type IO_COUNTERS struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

const (
	JobObjectExtendedLimitInformation = 9
	JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE = 0x2000
)

func initJobObject() {
	once.Do(func() {
		r1, _, err := procCreateJobObjectW.Call(0, 0)
		if r1 == 0 {
			logger.Log("Warning: Failed to create Job Object: %v", err)
			return
		}
		globalJobHandle = syscall.Handle(r1)

		info := JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
		info.BasicLimitInformation.LimitFlags = JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE

		r1, _, err = procSetInformationJobObject.Call(
			uintptr(globalJobHandle),
			uintptr(JobObjectExtendedLimitInformation),
			uintptr(unsafe.Pointer(&info)),
			uintptr(unsafe.Sizeof(info)),
		)
		if r1 == 0 {
			logger.Log("Warning: Failed to set Job Object info: %v", err)
		}
	})
}

// BindProcess binds an existing OS process to the global Win32 Job Object.
// This guarantees that the process and its children are killed if the Bridge exits.
func BindProcess(pid int) error {
	initJobObject()
	if globalJobHandle == 0 {
		return fmt.Errorf("job object not initialized")
	}

	const PROCESS_SET_QUOTA = 0x0100
	const PROCESS_TERMINATE = 0x0001
	hProcess, err := syscall.OpenProcess(PROCESS_SET_QUOTA|PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return fmt.Errorf("OpenProcess failed for PID %d: %v", pid, err)
	}
	defer syscall.CloseHandle(hProcess)

	r1, _, err := procAssignProcessToJobObject.Call(
		uintptr(globalJobHandle),
		uintptr(hProcess),
	)
	if r1 == 0 {
		return fmt.Errorf("AssignProcessToJobObject failed for PID %d: %v", pid, err)
	}
	return nil
}

type Manager struct{}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) StopAll() {
	// Handled automatically by Win32 Job Object (JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE)
}

func streamWithPrefix(r io.Reader, pid int, procName string) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		logger.LogProc(pid, procName, "%s", scanner.Text())
	}
}

func (m *Manager) StartBackground(name string, exe string, args ...string) error {
	cmd := exec.Command(exe, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	err := cmd.Start()
	if err != nil {
		logger.Log("Failed to start %s: %v", name, err)
		return err
	}
	
	pid := cmd.Process.Pid
	procName := filepath.Base(exe)
	
	go streamWithPrefix(stdout, pid, procName)
	go streamWithPrefix(stderr, pid, procName)

	if cmd.Process != nil {
		BindProcess(pid)
	}
	return nil
}
