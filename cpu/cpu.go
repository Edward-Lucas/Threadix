package cpu

import (
	"fmt"
	"math/bits"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32dll                        = windows.NewLazySystemDLL("kernel32.dll")
	procGetLogicalProcessorInformation = kernel32dll.NewProc("GetLogicalProcessorInformation")
)

// Info holds CPU information.
type Info struct {
	ModelName     string `json:"modelName"`
	PhysicalCores int    `json:"physicalCores"`
	LogicalCores  int    `json:"logicalCores"`
	Threads       int    `json:"threads"`
}

// GetInfo returns CPU information.
func GetInfo() (*Info, error) {
	logicalCores, err := getLogicalProcessorCount()
	if err != nil {
		logicalCores = runtime.NumCPU()
	}
	info := &Info{
		LogicalCores: logicalCores,
		Threads:      logicalCores,
	}

	physicalCores, err := getPhysicalCoreCount()
	if err == nil {
		info.PhysicalCores = physicalCores
	} else {
		info.PhysicalCores = info.LogicalCores / 2
		if info.PhysicalCores < 1 {
			info.PhysicalCores = info.LogicalCores
		}
	}

	modelName, err := getCPUModelName()
	if err == nil {
		info.ModelName = modelName
	} else {
		info.ModelName = "Unknown"
	}

	return info, nil
}

// Relation types for GetLogicalProcessorInformation
const (
	RelationProcessorCore    = 0
	RelationNumaNode         = 1
	RelationCache            = 2
	RelationProcessorPackage = 3
)

type SYSTEM_LOGICAL_PROCESSOR_INFORMATION struct {
	ProcessorMask uintptr
	Relationship  uint32
	_             [4]byte // padding
	Union         [16]byte
}

func getLogicalProcessorCount() (int, error) {
	var returnLength uint32

	// First call to get buffer size
	ret, _, err := procGetLogicalProcessorInformation.Call(0, uintptr(unsafe.Pointer(&returnLength)))
	if ret != 0 {
		return 0, fmt.Errorf("unexpected success on first call")
	}
	if err.Error() != "The data area passed to a system call is too small." {
		return 0, fmt.Errorf("GetLogicalProcessorInformation failed: %v", err)
	}

	// Allocate buffer
	buf := make([]byte, returnLength)
	ret, _, err = procGetLogicalProcessorInformation.Call(
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&returnLength)),
	)
	if ret == 0 {
		return 0, fmt.Errorf("GetLogicalProcessorInformation failed: %v", err)
	}

	count := 0
	offset := 0
	entrySize := int(unsafe.Sizeof(SYSTEM_LOGICAL_PROCESSOR_INFORMATION{}))

	for offset+entrySize <= int(returnLength) {
		info := (*SYSTEM_LOGICAL_PROCESSOR_INFORMATION)(unsafe.Pointer(&buf[offset]))
		if info.Relationship == RelationProcessorCore {
			count += bits.OnesCount64(uint64(info.ProcessorMask))
		}
		offset += entrySize
	}

	return count, nil
}

func getPhysicalCoreCount() (int, error) {
	var returnLength uint32

	// First call to get buffer size
	ret, _, err := procGetLogicalProcessorInformation.Call(0, uintptr(unsafe.Pointer(&returnLength)))
	if ret != 0 {
		return 0, fmt.Errorf("unexpected success on first call")
	}
	if err.Error() != "The data area passed to a system call is too small." {
		return 0, fmt.Errorf("GetLogicalProcessorInformation failed: %v", err)
	}

	// Allocate buffer
	buf := make([]byte, returnLength)
	ret, _, err = procGetLogicalProcessorInformation.Call(
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&returnLength)),
	)
	if ret == 0 {
		return 0, fmt.Errorf("GetLogicalProcessorInformation failed: %v", err)
	}

	// Count physical cores (RelationProcessorCore entries)
	count := 0
	offset := 0
	entrySize := int(unsafe.Sizeof(SYSTEM_LOGICAL_PROCESSOR_INFORMATION{}))

	for offset+entrySize <= int(returnLength) {
		info := (*SYSTEM_LOGICAL_PROCESSOR_INFORMATION)(unsafe.Pointer(&buf[offset]))
		if info.Relationship == RelationProcessorCore {
			count++
		}
		offset += entrySize
	}

	return count, nil
}

func getCPUModelName() (string, error) {
	var key windows.Handle
	subKey, err := windows.UTF16PtrFromString(`HARDWARE\DESCRIPTION\System\CentralProcessor\0`)
	if err != nil {
		return "", err
	}

	err = windows.RegOpenKeyEx(windows.HKEY_LOCAL_MACHINE, subKey, 0, windows.KEY_READ, &key)
	if err != nil {
		return "", err
	}
	defer windows.RegCloseKey(key)

	valueName, _ := windows.UTF16PtrFromString("ProcessorNameString")
	var bufType uint32
	buf := make([]uint16, 256)
	bufSize := uint32(len(buf) * 2)

	err = windows.RegQueryValueEx(key, valueName, nil, &bufType, (*byte)(unsafe.Pointer(&buf[0])), &bufSize)
	if err != nil {
		return "", err
	}

	return windows.UTF16ToString(buf), nil
}
