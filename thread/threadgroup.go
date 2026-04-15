package thread

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/threadix/backend/cpu"
)

// GroupType represents the type of thread group.
type GroupType int

const (
	GroupSystem    GroupType = iota // system thread
	GroupMain                       // main thread
	GroupAuxiliary                  // auxiliary thread
)

func (g GroupType) String() string {
	switch g {
	case GroupSystem:
		return "System"
	case GroupMain:
		return "Main"
	case GroupAuxiliary:
		return "Auxiliary"
	default:
		return "Unknown"
	}
}

func (g GroupType) DisplayName() string {
	switch g {
	case GroupSystem:
		return "System Threads"
	case GroupMain:
		return "Main Threads"
	case GroupAuxiliary:
		return "Auxiliary Threads"
	default:
		return "Unknown"
	}
}

// Group represents a thread group with allocated CPU cores.
type Group struct {
	Type  GroupType `json:"type"`
	Cores []uint32  `json:"cores"`
}

// Affinity returns the cores as a formatted affinity string (e.g., "0-3").
func (g *Group) Affinity() string {
	return FormatCores(g.Cores)
}

// CoreCount returns the number of allocated cores.
func (g *Group) CoreCount() int {
	return len(g.Cores)
}

// Groups holds all three thread groups.
type Groups struct {
	System    Group `json:"system"`
	Main      Group `json:"main"`
	Auxiliary Group `json:"auxiliary"`
}

// Get returns the group for the given type.
func (g *Groups) Get(gt GroupType) *Group {
	switch gt {
	case GroupSystem:
		return &g.System
	case GroupMain:
		return &g.Main
	case GroupAuxiliary:
		return &g.Auxiliary
	default:
		return nil
	}
}

// All returns all groups as a slice.
func (g *Groups) All() []*Group {
	return []*Group{&g.System, &g.Main, &g.Auxiliary}
}

// CalculateDefaultAllocation computes default thread group allocation based on total thread count.
// Order: Main first, then System, then Auxiliary.
//
//	N = total threads
//	Main:      N/2            cores (starting from 0)
//	Auxiliary: 2              cores, when possible
//	System:    remaining cores (between Main and Auxiliary)
//
// Example (N=12): Main=0-5, System=6-9, Auxiliary=10-11
// Example (N=16): Main=0-7, System=8-13, Auxiliary=14-15
// Example (N=7):  Main=0-2, System=3-4, Auxiliary=5-6
// Example (N=5):  Main=0-1, System=2, Auxiliary=3-4
func CalculateDefaultAllocation(totalThreads int) *Groups {
	if totalThreads < 1 {
		totalThreads = 1
	}

	mainCount := totalThreads / 2
	auxCount := 2

	// Ensure that there is always at least one core for the system group.
	if totalThreads-mainCount <= auxCount {
		auxCount = totalThreads - mainCount - 1
		if auxCount < 0 {
			auxCount = 0
		}
	}

	systemCount := totalThreads - mainCount - auxCount
	if systemCount < 1 {
		systemCount = 1
		auxCount = totalThreads - mainCount - systemCount
		if auxCount < 0 {
			auxCount = 0
		}
	}

	// Main cores: starting from 0
	mainCores := make([]uint32, mainCount)
	for i := 0; i < mainCount; i++ {
		mainCores[i] = uint32(i)
	}

	// System cores: following main
	systemCores := make([]uint32, systemCount)
	for i := 0; i < systemCount; i++ {
		systemCores[i] = uint32(mainCount + i)
	}

	// Auxiliary cores: following system
	auxCores := make([]uint32, auxCount)
	for i := 0; i < auxCount; i++ {
		auxCores[i] = uint32(mainCount + systemCount + i)
	}

	return &Groups{
		Main:      Group{Type: GroupMain, Cores: mainCores},
		System:    Group{Type: GroupSystem, Cores: systemCores},
		Auxiliary: Group{Type: GroupAuxiliary, Cores: auxCores},
	}
}

// FormatCores formats a list of core indices into "0-3;5;7-9" string.
func FormatCores(cores []uint32) string {
	if len(cores) == 0 {
		return ""
	}

	sorted := make([]uint32, len(cores))
	copy(sorted, cores)

	// Sort
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

// GetTotalThreads returns the number of logical CPU cores.
func GetTotalThreads() int {
	if info, err := cpu.GetInfo(); err == nil {
		return info.Threads
	}
	return runtime.NumCPU()
}

// ParseCoreString parses a core string like "0-3;5" into a slice of core indices.
func ParseCoreString(s string) ([]uint32, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty core string")
	}

	parts := strings.Split(s, ";")
	var cores []uint32
	seen := make(map[uint32]bool)

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		rangeParts := strings.Split(part, "-")
		if len(rangeParts) == 2 {
			start, err := parseUint(strings.TrimSpace(rangeParts[0]))
			if err != nil {
				return nil, fmt.Errorf("invalid range start: %s", rangeParts[0])
			}
			end, err := parseUint(strings.TrimSpace(rangeParts[1]))
			if err != nil {
				return nil, fmt.Errorf("invalid range end: %s", rangeParts[1])
			}
			if start > end {
				return nil, fmt.Errorf("range start > end: %d > %d", start, end)
			}
			for i := start; i <= end; i++ {
				if !seen[i] {
					cores = append(cores, i)
					seen[i] = true
				}
			}
		} else if len(rangeParts) == 1 {
			core, err := parseUint(strings.TrimSpace(rangeParts[0]))
			if err != nil {
				return nil, fmt.Errorf("invalid core: %s", rangeParts[0])
			}
			if !seen[core] {
				cores = append(cores, core)
				seen[core] = true
			}
		} else {
			return nil, fmt.Errorf("invalid format: %s", part)
		}
	}

	if len(cores) == 0 {
		return nil, fmt.Errorf("no cores specified")
	}

	return cores, nil
}

func parseUint(s string) (uint32, error) {
	var n uint32
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}
