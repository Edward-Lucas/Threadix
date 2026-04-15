package process

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	advapi32dll               = windows.NewLazySystemDLL("advapi32.dll")
	procOpenSCManagerW        = advapi32dll.NewProc("OpenSCManagerW")
	procEnumServicesStatusExW = advapi32dll.NewProc("EnumServicesStatusExW")
	procCloseServiceHandleW   = advapi32dll.NewProc("CloseServiceHandle")
)

const (
	SC_MANAGER_ENUMERATE_SERVICE = 0x0004
	SERVICE_WIN32                = 0x00000030
	SERVICE_ACTIVE               = 0x00000001
	SC_ENUM_PROCESS_INFO         = 0
)

// ENUM_SERVICE_STATUS_PROCESS contains service status info including PID.
type ENUM_SERVICE_STATUS_PROCESS struct {
	ServiceName          *uint16
	DisplayName          *uint16
	ServiceStatusProcess SERVICE_STATUS_PROCESS
}

type SERVICE_STATUS_PROCESS struct {
	ServiceType             uint32
	CurrentState            uint32
	ControlsAccepted        uint32
	Win32ExitCode           uint32
	ServiceSpecificExitCode uint32
	CheckPoint              uint32
	WaitHint                uint32
	ProcessId               uint32
	ServiceFlags            uint32
}

// Service represents a running Windows service.
type Service struct {
	Name string `json:"name"`
	PID  uint32 `json:"pid"`
}

// GetRunningServices returns a map of PID -> service name for all running services.
func GetRunningServices() (map[uint32]string, error) {
	scManager, _, err := procOpenSCManagerW.Call(0, 0, SC_MANAGER_ENUMERATE_SERVICE)
	if scManager == 0 {
		return nil, fmt.Errorf("OpenSCManager failed (run as admin): %v", err)
	}
	defer procCloseServiceHandleW.Call(scManager)

	var bytesNeeded, servicesReturned, resumeHandle uint32
	var groupPtr uintptr = 0 // all services

	// First call to get buffer size
	procEnumServicesStatusExW.Call(
		scManager,
		SC_ENUM_PROCESS_INFO,
		SERVICE_WIN32,
		SERVICE_ACTIVE,
		0,
		0,
		uintptr(unsafe.Pointer(&bytesNeeded)),
		uintptr(unsafe.Pointer(&servicesReturned)),
		uintptr(unsafe.Pointer(&resumeHandle)),
		groupPtr,
	)

	buf := make([]byte, bytesNeeded+8192)
	ret, _, _ := procEnumServicesStatusExW.Call(
		scManager,
		SC_ENUM_PROCESS_INFO,
		SERVICE_WIN32,
		SERVICE_ACTIVE,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		uintptr(unsafe.Pointer(&bytesNeeded)),
		uintptr(unsafe.Pointer(&servicesReturned)),
		uintptr(unsafe.Pointer(&resumeHandle)),
		groupPtr,
	)
	if ret == 0 {
		return nil, fmt.Errorf("EnumServicesStatusEx failed")
	}

	result := make(map[uint32]string)
	entrySize := uint32(unsafe.Sizeof(ENUM_SERVICE_STATUS_PROCESS{}))

	for i := uint32(0); i < servicesReturned; i++ {
		offset := uintptr(i) * uintptr(entrySize)
		entry := (*ENUM_SERVICE_STATUS_PROCESS)(unsafe.Pointer(&buf[offset]))

		pid := entry.ServiceStatusProcess.ProcessId
		if pid > 0 {
			serviceName := utf16PtrToString(entry.ServiceName)
			result[pid] = serviceName
		}
	}

	return result, nil
}

// GetServicesList returns a list of all running services with their PIDs.
func GetServicesList() ([]Service, error) {
	services, err := GetRunningServices()
	if err != nil {
		return nil, err
	}

	var result []Service
	for pid, name := range services {
		result = append(result, Service{Name: name, PID: pid})
	}
	return result, nil
}

func utf16PtrToString(p *uint16) string {
	if p == nil {
		return ""
	}
	s := (*[1 << 20]uint16)(unsafe.Pointer(p))
	var length int
	for length = 0; s[length] != 0; length++ {
	}
	return syscall.UTF16ToString(s[:length+1])
}
