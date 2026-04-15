package thread

import (
	"strings"

	"github.com/threadix/backend/process"
)

// knownSystemProcesses is a list of known Windows system process names.
var knownSystemProcesses = map[string]bool{
	"system":                      true,
	"registry":                    true,
	"secure system":               true,
	"smss.exe":                    true,
	"csrss.exe":                   true,
	"wininit.exe":                 true,
	"winlogon.exe":                true,
	"services.exe":                true,
	"lsass.exe":                   true,
	"lsaiso.exe":                  true,
	"svchost.exe":                 true,
	"fontdrvhost.exe":             true,
	"dwm.exe":                     true,
	"sihost.exe":                  true,
	"shellexperiencehost.exe":     true,
	"startmenuexperiencehost.exe": true,
	"runtimebroker.exe":           true,
	"backgroundtaskhost.exe":      true,
	"searchhost.exe":              true,
	"searchindexer.exe":           true,
	"spoolsv.exe":                 true,
	"wlanext.exe":                 true,
	"conhost.exe":                 true,
	"memory compression":          true,
	"wmiprvse.exe":                true,
	"msmpeng.exe":                 true,
	"securityhealthservice.exe":   true,
	"audiodg.exe":                 true,
	"wudfhost.exe":                true,
	"dasHost.exe":                 true,
	"ntoskrnl.exe":                true,
	"system idle process":         true,
	"interrupts":                  true,
	"dpc":                         true,
}

// systemPathPrefixes are executable path prefixes that indicate a system process.
var systemPathPrefixes = []string{
	`C:\Windows\System32\`,
	`C:\Windows\SysWOW64\`,
	`C:\Windows\`,
}

// IsSystemProcess determines if a process should be classified as a system process.
func IsSystemProcess(proc process.Process) bool {
	// PID 0 is System Idle
	if proc.PID == 0 {
		return true
	}

	// Check by known name
	nameLower := strings.ToLower(proc.Name)
	if knownSystemProcesses[nameLower] {
		return true
	}

	// Check by executable path
	if proc.ExePath != "" {
		exeLower := strings.ToLower(proc.ExePath)
		for _, prefix := range systemPathPrefixes {
			if strings.HasPrefix(exeLower, strings.ToLower(prefix)) {
				return true
			}
		}
	}

	return false
}

// ClassifyProcesses classifies all running processes into System and Main groups.
// Returns (systemProcesses, mainProcesses).
func ClassifyProcesses() ([]process.Process, []process.Process, error) {
	snapshot, err := process.GetRunningProcesses()
	if err != nil {
		return nil, nil, err
	}

	var systemProcs, mainProcs []process.Process
	for _, proc := range snapshot.Processes {
		if proc.PID == 0 {
			continue
		}
		if IsSystemProcess(proc) {
			systemProcs = append(systemProcs, proc)
		} else {
			mainProcs = append(mainProcs, proc)
		}
	}

	return systemProcs, mainProcs, nil
}

// ClassifyAndGroup classifies processes and returns them grouped.
// Returns a map of GroupType -> processes.
func ClassifyAndGroup() (map[GroupType][]process.Process, error) {
	systemProcs, mainProcs, err := ClassifyProcesses()
	if err != nil {
		return nil, err
	}

	return map[GroupType][]process.Process{
		GroupSystem: systemProcs,
		GroupMain:   mainProcs,
	}, nil
}
