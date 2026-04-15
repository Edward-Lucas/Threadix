package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/driver/desktop"

	"github.com/threadix/backend/config"
	"github.com/threadix/backend/process"
	"github.com/threadix/backend/rules"
	"github.com/threadix/backend/thread"
	"github.com/threadix/backend/ui"
)

const (
	appTitle           = "Threadix"
	githubURL          = "https://github.com/Edward-Lucas/Threadix"
	errorAlreadyExists = 183
)

var (
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	user32                  = syscall.NewLazyDLL("user32.dll")
	procCreateMutexW        = kernel32.NewProc("CreateMutexW")
	procCloseHandle         = kernel32.NewProc("CloseHandle")
	procEnumWindows         = user32.NewProc("EnumWindows")
	procGetWindowTextLength = user32.NewProc("GetWindowTextLengthW")
	procGetWindowText       = user32.NewProc("GetWindowTextW")
	procIsWindowVisible     = user32.NewProc("IsWindowVisible")
	procIsIconic            = user32.NewProc("IsIconic")
	procShowWindow          = user32.NewProc("ShowWindow")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
)

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)

	// Prevent multiple instances using named mutex
	mutex := createSingleInstanceMutex()
	if mutex == 0 {
		activateExistingWindow()
		return
	}
	defer releaseMutex(mutex)

	// Initialize Fyne app
	a := app.NewWithID("com.threadix.process-governor")
	a.Settings().SetTheme(ui.NewKoreanTheme())

	if err := process.ResetCurrentProcessAffinity(); err != nil {
		log.Printf("[self-affinity] failed to restore current process affinity: %v", err)
	} else {
		log.Printf("[self-affinity] restored current process affinity to system mask")
	}

	var appIcon fyne.Resource
	iconPath := filepath.Join(getExeDir(), "resources", "app.ico")
	if iconRes, err := fyne.LoadResourceFromPath(iconPath); err == nil {
		appIcon = iconRes
		a.SetIcon(appIcon)
		log.Printf("[icon] loaded: %s", iconPath)
	} else {
		log.Printf("[icon] failed to load: %v", err)
	}

	w := a.NewWindow(appTitle)
	if appIcon != nil {
		w.SetIcon(appIcon)
	}
	w.Resize(fyne.NewSize(1100, 700))
	w.CenterOnScreen()

	// Config loader
	cfgLoader := config.NewLoader(config.DefaultConfigFile)

	// Check if first run (no threadGroups in config)
	cfg, err := cfgLoader.LoadConfig()
	isFirstRun := err != nil || cfg.ThreadGroups == nil

	var mainView *ui.MainView

	if isFirstRun {
		w.SetContent(ui.SetupScreen(w, cfgLoader, func(savedCfg *config.Config) {
			groups := loadThreadGroups(savedCfg)
			mainView = ui.NewMainView(w, cfgLoader, groups)
			w.SetContent(mainView.Container())
			w.SetTitle(appTitle)
			startBackgroundEngine(cfgLoader)
		}))
		w.SetTitle(appTitle + " - Initial Setup")
	} else {
		groups := loadThreadGroups(cfg)
		mainView = ui.NewMainView(w, cfgLoader, groups)
		w.SetContent(mainView.Container())
		startBackgroundEngine(cfgLoader)
	}

	// System tray
	if desk, ok := a.(desktop.App); ok {
		menu := fyne.NewMenu(appTitle,
			fyne.NewMenuItem("Open Window", func() {
				w.Show()
				w.RequestFocus()
			}),
			fyne.NewMenuItemSeparator(),
			fyne.NewMenuItem("Open config.json", func() {
				ui.LaunchConfigEditor(config.DefaultConfigFile)
			}),
			fyne.NewMenuItem("GitHub", func() {
				ui.LaunchGitHub(githubURL)
			}),
		)
		desk.SetSystemTrayMenu(menu)
	}

	// Close to tray instead of quit
	w.SetCloseIntercept(func() {
		w.Hide()
	})

	w.ShowAndRun()
}

// createSingleInstanceMutex creates a session-local named mutex.
// Returns the mutex handle if this is the first instance, 0 otherwise.
func createSingleInstanceMutex() syscall.Handle {
	mutexName, _ := syscall.UTF16PtrFromString("Local\\ThreadixProcessGovernorMutex")

	handle, _, err := procCreateMutexW.Call(
		0, // default security
		0, // not initially owned
		uintptr(unsafe.Pointer(mutexName)),
	)

	if handle == 0 {
		if err != nil {
			log.Printf("[single-instance] CreateMutexW failed: %v", err)
		} else {
			log.Printf("[single-instance] CreateMutexW failed: unknown error")
		}
		return 0
	}

	if err == syscall.Errno(errorAlreadyExists) {
		procCloseHandle.Call(handle)
		log.Printf("[single-instance] another instance is already running")
		return 0
	}

	return syscall.Handle(handle)
}

func activateExistingWindow() {
	hwnd := findExistingWindow()
	if hwnd == 0 {
		log.Printf("[single-instance] existing instance detected, but no window found")
		return
	}

	if isWindowMinimized(hwnd) {
		showWindow(hwnd, 9) // SW_RESTORE
	}

	setForegroundWindow(hwnd)
}

func findExistingWindow() uintptr {
	var found uintptr
	cb := syscall.NewCallback(func(hwnd uintptr, lparam uintptr) uintptr {
		if !isWindowVisible(hwnd) {
			return 1
		}

		title := getWindowText(hwnd)
		if strings.Contains(title, appTitle) {
			found = hwnd
			return 0
		}

		return 1
	})

	procEnumWindows.Call(cb, 0)
	return found
}

func getWindowText(hwnd uintptr) string {
	length, _, _ := procGetWindowTextLength.Call(hwnd)
	if length == 0 {
		return ""
	}

	buf := make([]uint16, length+1)
	procGetWindowText.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return syscall.UTF16ToString(buf)
}

func isWindowVisible(hwnd uintptr) bool {
	ret, _, _ := procIsWindowVisible.Call(hwnd)
	return ret != 0
}

func isWindowMinimized(hwnd uintptr) bool {
	ret, _, _ := procIsIconic.Call(hwnd)
	return ret != 0
}

func showWindow(hwnd uintptr, cmd int) {
	procShowWindow.Call(hwnd, uintptr(cmd))
}

func setForegroundWindow(hwnd uintptr) {
	procSetForegroundWindow.Call(hwnd)
}

func releaseMutex(handle syscall.Handle) {
	if handle != 0 {
		procCloseHandle.Call(uintptr(handle))
	}
}

func getExeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

func loadThreadGroups(cfg *config.Config) *thread.Groups {
	totalThreads := thread.GetTotalThreads()
	defaults := thread.CalculateDefaultAllocation(totalThreads)

	if cfg.ThreadGroups != nil {
		if cores, err := thread.ParseCoreString(cfg.ThreadGroups.System.Cores); err == nil && len(cores) > 0 {
			defaults.System.Cores = cores
		}
		if cores, err := thread.ParseCoreString(cfg.ThreadGroups.Main.Cores); err == nil && len(cores) > 0 {
			defaults.Main.Cores = cores
		}
		if cores, err := thread.ParseCoreString(cfg.ThreadGroups.Auxiliary.Cores); err == nil && len(cores) > 0 {
			defaults.Auxiliary.Cores = cores
		}
	}

	return defaults
}

func startBackgroundEngine(cfgLoader *config.Loader) {
	go func() {
		engine := rules.NewEngine()
		interval := 1 * time.Second
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			cfg, changed, err := cfgLoader.ReloadIfChanged()
			if err != nil {
				continue
			}

			if changed {
				newInterval := time.Duration(cfg.RuleApplyIntervalSeconds) * time.Second
				if newInterval > 0 && newInterval != interval {
					interval = newInterval
					ticker.Reset(interval)
				}
			}

			results, err := engine.ApplyRules(cfg, !changed)
			if err != nil {
				log.Printf("[Engine] Error: %v", err)
				continue
			}

			for _, r := range results {
				if r.Applied {
					log.Printf("[Engine] Applied rule '%s' to PID %d (%s)", r.Rule, r.PID, r.Name)
				}
			}
		}
	}()
}
