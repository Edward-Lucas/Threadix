package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// ThreadGroupConfig represents a single thread group allocation in config.
type ThreadGroupConfig struct {
	Cores string `json:"cores"` // "0-3", "4-9", "10-11"
}

// ThreadGroupsConfig holds the thread group configuration.
type ThreadGroupsConfig struct {
	System    ThreadGroupConfig `json:"system"`
	Main      ThreadGroupConfig `json:"main"`
	Auxiliary ThreadGroupConfig `json:"auxiliary"`
}

// Config represents the application configuration.
type Config struct {
	Version                  *int                `json:"version,omitempty"`
	RuleApplyIntervalSeconds int                 `json:"ruleApplyIntervalSeconds"`
	ThreadGroups             *ThreadGroupsConfig `json:"threadGroups,omitempty"`
	ProcessRules             []ProcessRule       `json:"processRules"`
	ServiceRules             []ServiceRule       `json:"serviceRules"`
}

// ProcessRule defines a rule applied to processes.
type ProcessRule struct {
	SelectorBy string  `json:"selectorBy"` // "Name", "Path", "CommandLine"
	Selector   string  `json:"selector"`
	Priority   *string `json:"priority,omitempty"`   // "Idle","BelowNormal","Normal","AboveNormal","High","Realtime"
	IOPriority *string `json:"ioPriority,omitempty"` // "VeryLow","Low","Normal"
	Affinity   *string `json:"affinity,omitempty"`   // "0-3", "0;2;4", "1;3-5"
	Force      string  `json:"force"`                // "Y" or "N"
	Delay      *int    `json:"delay,omitempty"`
}

// ServiceRule defines a rule applied to Windows services.
type ServiceRule struct {
	Selector   string  `json:"selector"`
	Priority   *string `json:"priority,omitempty"`
	IOPriority *string `json:"ioPriority,omitempty"`
	Affinity   *string `json:"affinity,omitempty"`
	Force      string  `json:"force"`
	Delay      *int    `json:"delay,omitempty"`
}

// GetDelay returns the delay value, defaulting to 0.
func (r *ProcessRule) GetDelay() int {
	if r.Delay != nil && *r.Delay > 0 {
		return *r.Delay
	}
	return 0
}

// GetDelay returns the delay value, defaulting to 0.
func (r *ServiceRule) GetDelay() int {
	if r.Delay != nil && *r.Delay > 0 {
		return *r.Delay
	}
	return 0
}

const (
	DefaultConfigFile       = "config.json"
	DefaultApplyIntervalSec = 1
)

// Loader handles config loading and change detection.
type Loader struct {
	mu          sync.RWMutex
	filePath    string
	prevModTime int64
	config      *Config
}

// NewLoader creates a new config loader.
func NewLoader(filePath string) *Loader {
	if filePath == "" {
		filePath = DefaultConfigFile
	}
	return &Loader{filePath: filePath}
}

// LoadConfig reads and parses the config file.
func (l *Loader) LoadConfig() (*Config, error) {
	data, err := os.ReadFile(l.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := &Config{
				RuleApplyIntervalSeconds: DefaultApplyIntervalSec,
				ProcessRules:             []ProcessRule{},
				ServiceRules:             []ServiceRule{},
			}
			if saveErr := l.SaveConfig(cfg); saveErr != nil {
				return cfg, fmt.Errorf("failed to create default config: %w", saveErr)
			}
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Strip UTF-8 BOM if present (PowerShell ConvertTo-Json adds this)
	data = bytes.TrimPrefix(data, utf8BOM)

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if cfg.ProcessRules == nil {
		cfg.ProcessRules = []ProcessRule{}
	}
	if cfg.ServiceRules == nil {
		cfg.ServiceRules = []ServiceRule{}
	}
	if cfg.RuleApplyIntervalSeconds <= 0 {
		cfg.RuleApplyIntervalSeconds = DefaultApplyIntervalSec
	}

	return &cfg, nil
}

// SaveConfig writes the config to file.
func (l *Loader) SaveConfig(cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	return os.WriteFile(l.filePath, data, 0644)
}

// ReloadIfChanged returns the config if it has been modified since last check.
// Returns (config, true) if changed, (config, false) if unchanged.
func (l *Loader) ReloadIfChanged() (*Config, bool, error) {
	info, err := os.Stat(l.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			l.mu.Lock()
			defer l.mu.Unlock()
			if l.config == nil {
				cfg, loadErr := l.LoadConfig()
				if loadErr != nil {
					return nil, false, loadErr
				}
				l.config = cfg
				return cfg, true, nil
			}
			return l.config, false, nil
		}
		return nil, false, err
	}

	modTime := info.ModTime().UnixNano()

	l.mu.RLock()
	changed := modTime != l.prevModTime
	currentCfg := l.config
	l.mu.RUnlock()

	if !changed && currentCfg != nil {
		return currentCfg, false, nil
	}

	cfg, err := l.LoadConfig()
	if err != nil {
		return nil, false, err
	}

	l.mu.Lock()
	l.prevModTime = modTime
	l.config = cfg
	l.mu.Unlock()

	return cfg, true, nil
}
