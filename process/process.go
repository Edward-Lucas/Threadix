package process

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32                      = windows.NewLazySystemDLL("kernel32.dll")
	procCreateToolhelp32Snapshot  = kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32First            = kernel32.NewProc("Process32FirstW")
	procProcess32Next             = kernel32.NewProc("Process32NextW")
	procOpenProcess               = kernel32.NewProc("OpenProcess")
	procCloseHandle               = kernel32.NewProc("CloseHandle")
	procSetPriorityClass          = kernel32.NewProc("SetPriorityClass")
	procGetPriorityClass          = kernel32.NewProc("GetPriorityClass")
	procSetProcessAffinityMask    = kernel32.NewProc("SetProcessAffinityMask")
	procGetProcessAffinityMask    = kernel32.NewProc("GetProcessAffinityMask")
	procQueryFullProcessImageName = kernel32.NewProc("QueryFullProcessImageNameW")
)

const (
	TH32CS_SNAPPROCESS = 0x00000002
	PROCESS_ALL_ACCESS = 0x1F0FFF

	IDLE_PRIORITY_CLASS         = 0x00000040
	BELOW_NORMAL_PRIORITY_CLASS = 0x00004000
	NORMAL_PRIORITY_CLASS       = 0x00000020
	ABOVE_NORMAL_PRIORITY_CLASS = 0x00008000
	HIGH_PRIORITY_CLASS         = 0x00000080
	REALTIME_PRIORITY_CLASS     = 0x00000100
)

type PROCESSENTRY32 struct {
	Size            uint32
	Usage           uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	Threads         uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [260]uint16
}

// Process represents a running process.
type Process struct {
	PID         uint32 `json:"pid"`
	Name        string `json:"name"`
	ExePath     string `json:"exePath"`
	ServiceName string `json:"serviceName,omitempty"`
	Priority    string `json:"priority"`
	AffinityStr string `json:"affinity"`
}

// Snapshot holds process enumeration results.
type Snapshot struct {
	Processes []Process
}

// GetRunningProcesses enumerates all running processes using Windows Toolhelp32 API.
func GetRunningProcesses() (*Snapshot, error) {
	handle, _, err := procCreateToolhelp32Snapshot.Call(TH32CS_SNAPPROCESS, 0)
	if handle == uintptr(syscall.InvalidHandle) {
		return nil, fmt.Errorf("CreateToolhelp32Snapshot failed: %v", err)
	}
	defer procCloseHandle.Call(handle)

	var pe32 PROCESSENTRY32
	pe32.Size = uint32(unsafe.Sizeof(pe32))

	ret, _, _ := procProcess32First.Call(handle, uintptr(unsafe.Pointer(&pe32)))
	if ret == 0 {
		return nil, fmt.Errorf("Process32First failed")
	}

	snapshot := &Snapshot{}
	for {
		pid := pe32.ProcessID
		if pid > 0 {
			name := utf16ToString(pe32.ExeFile[:])
			exePath := getProcessExePath(pid)

			snapshot.Processes = append(snapshot.Processes, Process{
				PID:         pid,
				Name:        name,
				ExePath:     exePath,
				Priority:    GetCurrentPriorityClass(pid),
				AffinityStr: GetCurrentAffinity(pid),
			})
		}

		ret, _, _ = procProcess32Next.Call(handle, uintptr(unsafe.Pointer(&pe32)))
		if ret == 0 {
			break
		}
	}

	return snapshot, nil
}

func utf16ToString(buf []uint16) string {
	var sb strings.Builder
	for _, v := range buf {
		if v == 0 {
			break
		}
		sb.WriteRune(rune(v))
	}
	return sb.String()
}

// getProcessExePath retrieves the full executable path for a process.
func getProcessExePath(pid uint32) string {
	handle, _, _ := procOpenProcess.Call(PROCESS_ALL_ACCESS, 0, uintptr(pid))
	if handle == 0 {
		return ""
	}
	defer procCloseHandle.Call(handle)

	var buf [260]uint16
	bufSize := uint32(260)
	ret, _, _ := procQueryFullProcessImageName.Call(
		handle,
		0,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&bufSize)),
	)
	if ret == 0 {
		return ""
	}
	return utf16ToString(buf[:])
}

// GetCurrentPriorityClass returns the priority class string for a process.
func GetCurrentPriorityClass(pid uint32) string {
	handle, _, _ := procOpenProcess.Call(PROCESS_ALL_ACCESS, 0, uintptr(pid))
	if handle == 0 {
		return ""
	}
	defer procCloseHandle.Call(handle)

	ret, _, _ := procGetPriorityClass.Call(handle)
	if ret == 0 {
		return ""
	}
	return priorityClassToString(uint32(ret))
}

// SetProcessPriority sets the priority class of a process.
func SetProcessPriority(pid uint32, priority string) error {
	priorityClass, err := parsePriority(priority)
	if err != nil {
		return err
	}

	handle, _, _ := procOpenProcess.Call(PROCESS_ALL_ACCESS, 0, uintptr(pid))
	if handle == 0 {
		return fmt.Errorf("OpenProcess failed for PID %d", pid)
	}
	defer procCloseHandle.Call(handle)

	ret, _, err := procSetPriorityClass.Call(handle, uintptr(priorityClass))
	if ret == 0 {
		return fmt.Errorf("SetPriorityClass failed for PID %d: %v", pid, err)
	}
	return nil
}

// GetProcessAffinityMask returns the current CPU affinity mask as a formatted string.
func GetProcessAffinityMask(pid uint32) string {
	handle, _, _ := procOpenProcess.Call(PROCESS_ALL_ACCESS, 0, uintptr(pid))
	if handle == 0 {
		return ""
	}
	defer procCloseHandle.Call(handle)

	var processMask, systemMask uintptr
	ret, _, _ := procGetProcessAffinityMask.Call(
		handle,
		uintptr(unsafe.Pointer(&processMask)),
		uintptr(unsafe.Pointer(&systemMask)),
	)
	if ret == 0 {
		return ""
	}
	return affinityMaskToString(processMask)
}

// GetProcessAffinityMaskValues returns the raw process and system affinity mask values.
func GetProcessAffinityMaskValues(pid uint32) (uintptr, uintptr, error) {
	handle, _, _ := procOpenProcess.Call(PROCESS_ALL_ACCESS, 0, uintptr(pid))
	if handle == 0 {
		return 0, 0, fmt.Errorf("OpenProcess failed for PID %d", pid)
	}
	defer procCloseHandle.Call(handle)

	var processMask, systemMask uintptr
	ret, _, _ := procGetProcessAffinityMask.Call(
		handle,
		uintptr(unsafe.Pointer(&processMask)),
		uintptr(unsafe.Pointer(&systemMask)),
	)
	if ret == 0 {
		return 0, 0, fmt.Errorf("GetProcessAffinityMask failed for PID %d", pid)
	}

	return processMask, systemMask, nil
}

// ResetCurrentProcessAffinity restores the current process affinity to the full system mask.
func ResetCurrentProcessAffinity() error {
	pid := uint32(os.Getpid())
	_, systemMask, err := GetProcessAffinityMaskValues(pid)
	if err != nil {
		return err
	}

	handle, _, _ := procOpenProcess.Call(PROCESS_ALL_ACCESS, 0, uintptr(pid))
	if handle == 0 {
		return fmt.Errorf("OpenProcess failed for PID %d", pid)
	}
	defer procCloseHandle.Call(handle)

	ret, _, err := procSetProcessAffinityMask.Call(handle, systemMask, 0)
	if ret == 0 {
		return fmt.Errorf("Reset process affinity failed for PID %d: %v", pid, err)
	}
	return nil
}

// SetProcessAffinity sets the CPU affinity of a process.
func SetProcessAffinity(pid uint32, cores []uint32) error {
	if len(cores) == 0 {
		return fmt.Errorf("no cores specified")
	}

	handle, _, _ := procOpenProcess.Call(PROCESS_ALL_ACCESS, 0, uintptr(pid))
	if handle == 0 {
		return fmt.Errorf("OpenProcess failed for PID %d", pid)
	}
	defer procCloseHandle.Call(handle)

	var mask uintptr
	for _, core := range cores {
		mask |= 1 << core
	}

	ret, _, err := procSetProcessAffinityMask.Call(handle, mask, 0)
	if ret == 0 {
		return fmt.Errorf("SetProcessAffinityMask failed for PID %d: %v", pid, err)
	}
	return nil
}

func parsePriority(s string) (uint32, error) {
	switch s {
	case "Idle":
		return IDLE_PRIORITY_CLASS, nil
	case "BelowNormal":
		return BELOW_NORMAL_PRIORITY_CLASS, nil
	case "Normal":
		return NORMAL_PRIORITY_CLASS, nil
	case "AboveNormal":
		return ABOVE_NORMAL_PRIORITY_CLASS, nil
	case "High":
		return HIGH_PRIORITY_CLASS, nil
	case "Realtime":
		return REALTIME_PRIORITY_CLASS, nil
	default:
		return 0, fmt.Errorf("unknown priority: %s", s)
	}
}

func priorityClassToString(pc uint32) string {
	switch pc {
	case IDLE_PRIORITY_CLASS:
		return "Idle"
	case BELOW_NORMAL_PRIORITY_CLASS:
		return "BelowNormal"
	case NORMAL_PRIORITY_CLASS:
		return "Normal"
	case ABOVE_NORMAL_PRIORITY_CLASS:
		return "AboveNormal"
	case HIGH_PRIORITY_CLASS:
		return "High"
	case REALTIME_PRIORITY_CLASS:
		return "Realtime"
	default:
		return "Unknown"
	}
}

func affinityMaskToString(mask uintptr) string {
	cores := []uint32{}
	for i := 0; i < 64; i++ {
		if mask&(1<<i) != 0 {
			cores = append(cores, uint32(i))
		}
	}
	return FormatAffinity(cores)
}

// FormatAffinity formats a list of core indices into "0-3;5;7-9" string.
func FormatAffinity(cores []uint32) string {
	if len(cores) == 0 {
		return ""
	}

	sorted := make([]uint32, len(cores))
	copy(sorted, cores)

	// Simple sort
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j] < sorted[i] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	var parts []string
	start := sorted[0]
	end := sorted[0]

	for i := 1; i < len(sorted); i++ {
		if sorted[i] == end+1 {
			end = sorted[i]
		} else {
			if start == end {
				parts = append(parts, fmt.Sprintf("%d", start))
			} else {
				parts = append(parts, fmt.Sprintf("%d-%d", start, end))
			}
			start = sorted[i]
			end = sorted[i]
		}
	}

	if start == end {
		parts = append(parts, fmt.Sprintf("%d", start))
	} else {
		parts = append(parts, fmt.Sprintf("%d-%d", start, end))
	}

	return strings.Join(parts, ";")
}

// GetCurrentAffinity returns the formatted affinity string for a process.
func GetCurrentAffinity(pid uint32) string {
	return GetProcessAffinityMask(pid)
}

// GetCurrentInfo returns current priority and affinity for a process.
func GetCurrentInfo(pid uint32) (priority string, affinity string) {
	return GetCurrentPriorityClass(pid), GetCurrentAffinity(pid)
}
