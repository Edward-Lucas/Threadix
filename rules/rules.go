package rules

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/threadix/backend/config"
	"github.com/threadix/backend/process"
	"github.com/threadix/backend/thread"
)

// ParseAffinity parses an affinity string like "0-3;5;7-9" into a slice of core indices.
func ParseAffinity(s string) ([]uint32, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty affinity string")
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
			start, err := strconv.ParseUint(strings.TrimSpace(rangeParts[0]), 10, 32)
			if err != nil {
				return nil, fmt.Errorf("invalid affinity range start: %s", rangeParts[0])
			}
			end, err := strconv.ParseUint(strings.TrimSpace(rangeParts[1]), 10, 32)
			if err != nil {
				return nil, fmt.Errorf("invalid affinity range end: %s", rangeParts[1])
			}
			if start > end {
				return nil, fmt.Errorf("affinity range start > end: %d > %d", start, end)
			}
			for i := start; i <= end; i++ {
				if !seen[uint32(i)] {
					cores = append(cores, uint32(i))
					seen[uint32(i)] = true
				}
			}
		} else if len(rangeParts) == 1 {
			core, err := strconv.ParseUint(strings.TrimSpace(rangeParts[0]), 10, 32)
			if err != nil {
				return nil, fmt.Errorf("invalid affinity core: %s", rangeParts[0])
			}
			if !seen[uint32(core)] {
				cores = append(cores, uint32(core))
				seen[uint32(core)] = true
			}
		} else {
			return nil, fmt.Errorf("invalid affinity format: %s", part)
		}
	}

	if len(cores) == 0 {
		return nil, fmt.Errorf("no cores specified in affinity")
	}

	maxCore := uint32(runtime.NumCPU() - 1)
	for _, core := range cores {
		if core > maxCore {
			return nil, fmt.Errorf("core index %d exceeds available CPU cores (max: %d)", core, maxCore)
		}
	}

	return cores, nil
}

// PatternToRegex converts a glob-like pattern to a compiled regex.
func PatternToRegex(pattern string) (*regexp.Regexp, error) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return nil, fmt.Errorf("empty pattern")
	}

	reStr := regexp.QuoteMeta(pattern)
	reStr = strings.ReplaceAll(reStr, `\*\*`, `__DOUBLE_STAR__`)
	reStr = strings.ReplaceAll(reStr, `\*`, `[^/\\]*`)
	reStr = strings.ReplaceAll(reStr, `\?`, `[^/\\]`)
	reStr = strings.ReplaceAll(reStr, `__DOUBLE_STAR__`, `.*`)

	return regexp.Compile(`(?i)^` + reStr + `$`)
}

// MatchPattern checks if a value matches a wildcard pattern.
func MatchPattern(pattern, value string) bool {
	if pattern == "" || value == "" {
		return false
	}
	if pattern == value {
		return true
	}
	re, err := PatternToRegex(pattern)
	if err != nil {
		return false
	}
	return re.MatchString(value)
}

// ApplyResult represents the result of applying a rule to a process.
type ApplyResult struct {
	PID      uint32
	Name     string
	Rule     string
	Applied  bool
	Priority string
	Affinity string
	Error    error
}

type matchedRule struct {
	Priority   *string
	Affinity   *string
	ForceApply bool
	Delay      int
	RuleDesc   string
	IsService  bool
}

// Engine manages rule application to processes.
type Engine struct {
	mu             sync.RWMutex
	ignorePIDs     map[uint32]bool
	knownProcesses map[uint32]bool
	servicesCache  map[uint32]string
	servicesTime   time.Time
}

// NewEngine creates a new rules engine.
func NewEngine() *Engine {
	currentPID := uint32(os.Getpid())
	return &Engine{
		ignorePIDs:     map[uint32]bool{0: true, currentPID: true},
		knownProcesses: make(map[uint32]bool),
		servicesCache:  make(map[uint32]string),
	}
}

// ApplyRules applies configuration rules to all running processes.
func (e *Engine) ApplyRules(cfg *config.Config, onlyNew bool) ([]ApplyResult, error) {
	snapshot, err := process.GetRunningProcesses()
	if err != nil {
		return nil, fmt.Errorf("failed to enumerate processes: %w", err)
	}

	// Build service PID map (cache for 30 seconds)
	services := e.getServicesCached()

	e.mu.Lock()
	currentPIDs := make(map[uint32]bool)
	for _, p := range snapshot.Processes {
		currentPIDs[p.PID] = true
	}
	for pid := range e.knownProcesses {
		if !currentPIDs[pid] {
			delete(e.knownProcesses, pid)
		}
	}
	e.mu.Unlock()

	var results []ApplyResult

	for i := range snapshot.Processes {
		proc := snapshot.Processes[i]

		e.mu.RLock()
		isIgnored := e.ignorePIDs[proc.PID]
		isKnown := e.knownProcesses[proc.PID]
		e.mu.RUnlock()

		if isIgnored {
			continue
		}

		isNew := !isKnown
		if isNew {
			e.mu.Lock()
			e.knownProcesses[proc.PID] = true
			e.mu.Unlock()
		}

		// Attach service name if available
		if svcName, ok := services[proc.PID]; ok {
			proc.ServiceName = svcName
		}

		// Find matching rule
		rule := e.findMatchingRule(cfg, proc)
		if rule != nil {
			// force=N + onlyNew + not new => skip
			if !rule.ForceApply && onlyNew && !isNew {
				continue
			}

			// Apply with delay
			if rule.Delay > 0 {
				pid := proc.PID
				delaySec := rule.Delay
				go func() {
					time.Sleep(time.Duration(delaySec) * time.Second)
					e.applyRule(proc, rule)
					log.Printf("[delay] Applied rule '%s' to PID %d (%s) after %ds delay",
						rule.RuleDesc, pid, proc.Name, delaySec)
				}()
				results = append(results, ApplyResult{
					PID:     proc.PID,
					Name:    proc.Name,
					Rule:    rule.RuleDesc,
					Applied: false,
				})
			} else {
				result := e.applyRule(proc, rule)
				results = append(results, result)
			}
			continue
		}

		// No explicit rule matched: apply default group affinity for System/Main processes.
		autoResult := e.applyDefaultGroupAffinity(proc, cfg)
		if autoResult.Applied || autoResult.Error != nil {
			results = append(results, autoResult)
		}
	}

	return results, nil
}

func (e *Engine) applyDefaultGroupAffinity(proc process.Process, cfg *config.Config) ApplyResult {
	if proc.PID == uint32(os.Getpid()) {
		return ApplyResult{
			PID:  proc.PID,
			Name: proc.Name,
			Rule: "auto:self",
		}
	}

	groupType := thread.GroupMain
	if thread.IsSystemProcess(proc) {
		groupType = thread.GroupSystem
	}

	groups := loadThreadGroups(cfg)
	group := groups.Get(groupType)
	affinity := group.Affinity()
	result := ApplyResult{
		PID:  proc.PID,
		Name: proc.Name,
		Rule: fmt.Sprintf("auto:%s", groupType.String()),
	}

	if affinity == "" {
		return result
	}

	currentAff := process.GetCurrentAffinity(proc.PID)
	if currentAff == affinity {
		return result
	}

	if len(group.Cores) == 0 {
		return result
	}

	if err := process.SetProcessAffinity(proc.PID, group.Cores); err != nil {
		result.Error = fmt.Errorf("auto affinity: %w", err)
		return result
	}

	result.Applied = true
	result.Affinity = affinity
	return result
}

func loadThreadGroups(cfg *config.Config) *thread.Groups {
	totalThreads := thread.GetTotalThreads()
	defaults := thread.CalculateDefaultAllocation(totalThreads)

	if cfg == nil || cfg.ThreadGroups == nil {
		return defaults
	}

	if cores, err := thread.ParseCoreString(cfg.ThreadGroups.System.Cores); err == nil && len(cores) > 0 {
		defaults.System.Cores = cores
	}
	if cores, err := thread.ParseCoreString(cfg.ThreadGroups.Main.Cores); err == nil && len(cores) > 0 {
		defaults.Main.Cores = cores
	}
	if cores, err := thread.ParseCoreString(cfg.ThreadGroups.Auxiliary.Cores); err == nil && len(cores) > 0 {
		defaults.Auxiliary.Cores = cores
	}

	return defaults
}

func (e *Engine) getServicesCached() map[uint32]string {
	e.mu.RLock()
	if time.Since(e.servicesTime) < 30*time.Second && len(e.servicesCache) > 0 {
		cache := e.servicesCache
		e.mu.RUnlock()
		return cache
	}
	e.mu.RUnlock()

	services, err := process.GetRunningServices()
	if err != nil {
		// Return cached if available
		e.mu.RLock()
		cache := e.servicesCache
		e.mu.RUnlock()
		return cache
	}

	e.mu.Lock()
	e.servicesCache = services
	e.servicesTime = time.Now()
	e.mu.Unlock()

	return services
}

func (e *Engine) findMatchingRule(cfg *config.Config, proc process.Process) *matchedRule {
	// Check service rules first (higher priority)
	if proc.ServiceName != "" {
		for _, rule := range cfg.ServiceRules {
			if MatchPattern(rule.Selector, proc.ServiceName) {
				return &matchedRule{
					Priority:   rule.Priority,
					Affinity:   rule.Affinity,
					ForceApply: rule.Force == "Y",
					Delay:      rule.GetDelay(),
					RuleDesc:   fmt.Sprintf("service:%s", rule.Selector),
					IsService:  true,
				}
			}
		}
	}

	// Check process rules
	for _, rule := range cfg.ProcessRules {
		value := e.getMatchValue(proc, rule.SelectorBy)
		if MatchPattern(rule.Selector, value) {
			return &matchedRule{
				Priority:   rule.Priority,
				Affinity:   rule.Affinity,
				ForceApply: rule.Force == "Y",
				Delay:      rule.GetDelay(),
				RuleDesc:   fmt.Sprintf("process:%s:%s", rule.SelectorBy, rule.Selector),
				IsService:  false,
			}
		}
	}

	return nil
}

func (e *Engine) getMatchValue(proc process.Process, selectorBy string) string {
	switch selectorBy {
	case "Path":
		return proc.ExePath
	case "CommandLine":
		return proc.ExePath
	default: // "Name"
		return proc.Name
	}
}

func (e *Engine) applyRule(proc process.Process, rule *matchedRule) ApplyResult {
	result := ApplyResult{
		PID:  proc.PID,
		Name: proc.Name,
		Rule: rule.RuleDesc,
	}

	if proc.PID == uint32(os.Getpid()) {
		return result
	}

	// Apply priority
	if rule.Priority != nil && *rule.Priority != "" {
		currentPri := process.GetCurrentPriorityClass(proc.PID)
		if currentPri != *rule.Priority {
			if err := process.SetProcessPriority(proc.PID, *rule.Priority); err != nil {
				result.Error = fmt.Errorf("set priority: %w", err)
			} else {
				result.Applied = true
				result.Priority = *rule.Priority
			}
		}
	}

	// Apply affinity
	if rule.Affinity != nil && *rule.Affinity != "" {
		currentAff := process.GetCurrentAffinity(proc.PID)
		if currentAff != *rule.Affinity {
			cores, err := ParseAffinity(*rule.Affinity)
			if err != nil {
				if result.Error != nil {
					result.Error = fmt.Errorf("%v; parse affinity: %w", result.Error, err)
				} else {
					result.Error = fmt.Errorf("parse affinity: %w", err)
				}
			} else {
				if err := process.SetProcessAffinity(proc.PID, cores); err != nil {
					if result.Error != nil {
						result.Error = fmt.Errorf("%v; set affinity: %w", result.Error, err)
					} else {
						result.Error = fmt.Errorf("set affinity: %w", err)
					}
				} else {
					result.Applied = true
					result.Affinity = *rule.Affinity
				}
			}
		}
	}

	return result
}
