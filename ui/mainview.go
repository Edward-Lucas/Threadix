package ui

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/threadix/backend/config"
	"github.com/threadix/backend/process"
	"github.com/threadix/backend/thread"
)

// MainView manages the main application screen.
type MainView struct {
	window            fyne.Window
	cfgLoader         *config.Loader
	groups            *thread.Groups
	classified        map[thread.GroupType][]process.Process
	procList          *ProcessListView
	selectedTab       thread.GroupType
	selectedProcess   *process.Process
	content           *fyne.Container
	groupButtons      map[thread.GroupType]*widget.Button
	selectedInfoLabel *widget.Label
	priorityButtons   map[string]*widget.Button
	moveGroupButtons  map[thread.GroupType]*widget.Button
}

// NewMainView creates a new main view.
func NewMainView(w fyne.Window, cfgLoader *config.Loader, groups *thread.Groups) *MainView {
	mv := &MainView{
		window:       w,
		cfgLoader:    cfgLoader,
		groups:       groups,
		groupButtons: make(map[thread.GroupType]*widget.Button),
		selectedTab:  thread.GroupMain,
	}

	mv.procList = NewProcessListView(w)
	mv.procList.SetCallbacks(
		mv.refreshProcesses,
		mv.onPriorityChange,
		mv.onMoveGroup,
		mv.onProcessSelect,
	)

	mv.classifyAndRefresh()
	mv.buildUI()

	return mv
}

func (mv *MainView) classifyAndRefresh() {
	snapshot, err := process.GetRunningProcesses()
	if err != nil {
		log.Printf("Classification error: %v", err)
		mv.classified = map[thread.GroupType][]process.Process{}
		return
	}

	selfPID := uint32(os.Getpid())
	classified := map[thread.GroupType][]process.Process{
		thread.GroupSystem:    {},
		thread.GroupMain:      {},
		thread.GroupAuxiliary: {},
	}

	for _, proc := range snapshot.Processes {
		if proc.PID == 0 || proc.PID == selfPID {
			continue
		}

		groupType := mv.determineProcessGroup(proc)
		classified[groupType] = append(classified[groupType], proc)
	}

	mv.classified = classified
}

func (mv *MainView) determineProcessGroup(proc process.Process) thread.GroupType {
	if thread.IsSystemProcess(proc) {
		return thread.GroupSystem
	}

	if mv.isAffinityInGroup(proc.AffinityStr, mv.groups.Auxiliary.Cores) {
		return thread.GroupAuxiliary
	}

	if mv.isAffinityInGroup(proc.AffinityStr, mv.groups.System.Cores) {
		return thread.GroupSystem
	}

	return thread.GroupMain
}

func (mv *MainView) isAffinityInGroup(affinity string, groupCores []uint32) bool {
	if affinity == "" || len(groupCores) == 0 {
		return false
	}

	procCores, err := thread.ParseCoreString(affinity)
	if err != nil {
		return false
	}

	groupSet := make(map[uint32]bool, len(groupCores))
	for _, core := range groupCores {
		groupSet[core] = true
	}

	for _, core := range procCores {
		if !groupSet[core] {
			return false
		}
	}

	return true
}

func (mv *MainView) buildUI() {
	for _, gt := range []thread.GroupType{thread.GroupSystem, thread.GroupMain, thread.GroupAuxiliary} {
		gtCopy := gt
		btn := mv.createGroupButton(gtCopy)
		mv.groupButtons[gt] = btn
	}

	mv.selectedInfoLabel = widget.NewLabel("Select a process to see details and change options directly.")
	mv.selectedInfoLabel.Wrapping = fyne.TextWrapWord

	mv.priorityButtons = make(map[string]*widget.Button)
	for _, priority := range []string{"Idle", "BelowNormal", "Normal", "AboveNormal", "High", "Realtime"} {
		prio := priority
		btn := widget.NewButton(prio, func() {
			if mv.selectedProcess == nil {
				return
			}
			if mv.selectedProcess.Priority == prio {
				return
			}
			mv.onPriorityChange(mv.selectedProcess.PID, prio)
		})
		btn.Importance = widget.LowImportance
		btn.Disable()
		mv.priorityButtons[prio] = btn
	}

	mv.moveGroupButtons = make(map[thread.GroupType]*widget.Button)
	for _, gt := range []thread.GroupType{thread.GroupSystem, thread.GroupMain, thread.GroupAuxiliary} {
		gtCopy := gt
		btn := widget.NewButton(gtCopy.DisplayName(), func() {
			if mv.selectedProcess == nil {
				return
			}
			current := mv.determineProcessGroup(*mv.selectedProcess)
			if current == gtCopy {
				return
			}
			mv.onMoveGroup(mv.selectedProcess.PID, current, gtCopy)
		})
		btn.Importance = widget.LowImportance
		btn.Disable()
		mv.moveGroupButtons[gtCopy] = btn
	}

	// Header
	settingsBtn := widget.NewButtonWithIcon("Settings", theme.SettingsIcon(), func() {
		mv.showSettingsDialog()
	})
	settingsBtn.Importance = widget.LowImportance

	refreshBtn := widget.NewButtonWithIcon("Refresh", theme.ViewRefreshIcon(), func() {
		mv.procList.SetData(mv.refreshProcesses(), mv.selectedTab)
	})
	refreshBtn.Importance = widget.LowImportance

	head := container.NewVBox(
		container.NewHBox(
			widget.NewLabelWithStyle("Threadix", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			layout.NewSpacer(),
			refreshBtn,
			settingsBtn,
		),
		widget.NewLabelWithStyle("Manage threads and priority optimization all in one place.", fyne.TextAlignLeading, fyne.TextStyle{}),
	)

	groupButtonList := container.NewVBox(
		mv.groupButtons[thread.GroupSystem],
		mv.groupButtons[thread.GroupMain],
		mv.groupButtons[thread.GroupAuxiliary],
	)

	priorityButtons := make([]fyne.CanvasObject, 0, len(mv.priorityButtons))
	for _, pr := range []string{"Idle", "BelowNormal", "Normal", "AboveNormal", "High", "Realtime"} {
		priorityButtons = append(priorityButtons, mv.priorityButtons[pr])
	}
	priorityBox := container.NewVBox(widget.NewLabel("Priority"), container.NewGridWithColumns(3, priorityButtons...))

	moveButtons := make([]fyne.CanvasObject, 0, len(mv.moveGroupButtons))
	for _, gt := range []thread.GroupType{thread.GroupSystem, thread.GroupMain, thread.GroupAuxiliary} {
		moveButtons = append(moveButtons, mv.moveGroupButtons[gt])
	}
	groupBox := container.NewVBox(widget.NewLabel("Move Group"), container.NewVBox(moveButtons...))

	selectedProcCard := widget.NewCard("Selected Process", "Displays information for the currently selected process.", container.NewVBox(
		mv.selectedInfoLabel,
		widget.NewSeparator(),
		container.NewHBox(priorityBox, layout.NewSpacer(), groupBox),
	))

	leftPane := container.NewMax(
		mv.procList.Container(),
	)

	rightPane := container.NewVBox(
		groupButtonList,
		widget.NewSeparator(),
		selectedProcCard,
	)
	mainSplit := container.NewHSplit(leftPane, rightPane)
	mainSplit.Offset = 0.65

	mv.selectGroup(thread.GroupMain)

	mv.content = container.NewBorder(
		container.NewVBox(head, widget.NewSeparator()),
		nil, nil, nil,
		mainSplit,
	)
}

func (mv *MainView) createGroupButton(gt thread.GroupType) *widget.Button {
	group := mv.groups.Get(gt)
	procs := mv.classified[gt]
	label := fmt.Sprintf("%s\n%d processes\nCores: %s", gt.DisplayName(), len(procs), group.Affinity())
	btn := widget.NewButton(label, func() {
		mv.selectGroup(gt)
	})
	btn.Importance = widget.LowImportance
	return btn
}

func (mv *MainView) selectGroup(gt thread.GroupType) {
	mv.selectedTab = gt
	mv.selectedProcess = nil

	for t, btn := range mv.groupButtons {
		if btn == nil {
			continue
		}
		if t == gt {
			btn.Importance = widget.HighImportance
		} else {
			btn.Importance = widget.LowImportance
		}
		btn.Refresh()
	}

	mv.updateSelectedProcessInfo()
	mv.procList.SetData(mv.classified[gt], gt)
}

func (mv *MainView) refreshProcesses() []process.Process {
	mv.classifyAndRefresh()
	mv.updateCards()
	mv.selectedProcess = nil
	mv.updateSelectedProcessInfo()
	return mv.classified[mv.selectedTab]
}

func (mv *MainView) updateCards() {
	for _, gt := range []thread.GroupType{thread.GroupSystem, thread.GroupMain, thread.GroupAuxiliary} {
		group := mv.groups.Get(gt)
		procs := mv.classified[gt]
		btn := mv.groupButtons[gt]
		if btn == nil {
			continue
		}

		label := fmt.Sprintf("%s\n%d processes | %s\nCores: %s", gt.DisplayName(), len(procs), group.Affinity(), group.Affinity())
		btn.SetText(label)
		if gt == mv.selectedTab {
			btn.Importance = widget.HighImportance
		} else {
			btn.Importance = widget.LowImportance
		}
		btn.Refresh()
	}
}

func (mv *MainView) onProcessSelect(proc *process.Process) {
	mv.selectedProcess = proc
	mv.updateSelectedProcessInfo()
}

func (mv *MainView) updateSelectedProcessInfo() {
	if mv.selectedProcess == nil {
		if mv.selectedInfoLabel != nil {
			mv.selectedInfoLabel.SetText("Select a process to see details and change options directly.")
		}
		for _, btn := range mv.priorityButtons {
			btn.Disable()
			btn.Importance = widget.LowImportance
			btn.Refresh()
		}
		for _, btn := range mv.moveGroupButtons {
			btn.Disable()
			btn.Importance = widget.LowImportance
			btn.Refresh()
		}
		return
	}

	if mv.selectedInfoLabel != nil {
		mv.selectedInfoLabel.SetText(mv.buildSelectedProcessText())
	}
	for name, btn := range mv.priorityButtons {
		btn.Enable()
		if mv.selectedProcess.Priority == name {
			btn.Importance = widget.HighImportance
		} else {
			btn.Importance = widget.LowImportance
		}
		btn.Refresh()
	}
	currentGroup := mv.determineProcessGroup(*mv.selectedProcess)
	for gt, btn := range mv.moveGroupButtons {
		btn.Enable()
		if gt == currentGroup {
			btn.Importance = widget.HighImportance
		} else {
			btn.Importance = widget.LowImportance
		}
		btn.Refresh()
	}
}

func (mv *MainView) buildSelectedProcessText() string {
	groupType := mv.determineProcessGroup(*mv.selectedProcess)
	return fmt.Sprintf("PID: %d\nName: %s\nPath: %s\nCurrent Group: %s\nPriority: %s\nAffinity: %s",
		mv.selectedProcess.PID,
		mv.selectedProcess.Name,
		mv.selectedProcess.ExePath,
		groupType.DisplayName(),
		mv.selectedProcess.Priority,
		mv.selectedProcess.AffinityStr,
	)
}

func (mv *MainView) showPriorityDialogForProcess(proc *process.Process) {
	priorities := []string{"Idle", "BelowNormal", "Normal", "AboveNormal", "High", "Realtime"}
	selectWidget := widget.NewSelect(priorities, nil)
	selectWidget.PlaceHolder = "Select priority..."

	infoLabel := widget.NewLabel(mv.buildSelectedProcessText())
	infoLabel.Wrapping = fyne.TextWrapWord

	d := dialog.NewCustomConfirm("Change Priority", "Apply", "Cancel",
		container.NewVBox(
			infoLabel,
			widget.NewSeparator(),
			widget.NewLabel("Select a new priority:"),
			selectWidget,
		),
		func(ok bool) {
			if !ok || selectWidget.Selected == "" {
				return
			}
			mv.onPriorityChange(proc.PID, selectWidget.Selected)
			mv.refreshProcesses()
			mv.procList.SetData(mv.classified[mv.selectedTab], mv.selectedTab)
		}, mv.window)
	d.Resize(fyne.NewSize(320, 220))
	d.Show()
}

func (mv *MainView) showMoveGroupDialogForProcess(proc *process.Process) {
	groups := []string{
		thread.GroupSystem.DisplayName(),
		thread.GroupMain.DisplayName(),
		thread.GroupAuxiliary.DisplayName(),
	}

	currentGroup := mv.determineProcessGroup(*proc).DisplayName()
	var options []string
	for _, g := range groups {
		if g != currentGroup {
			options = append(options, g)
		}
	}

	selectWidget := widget.NewSelect(options, nil)
	selectWidget.PlaceHolder = "Select target group..."
	infoLabel := widget.NewLabel(mv.buildSelectedProcessText())
	infoLabel.Wrapping = fyne.TextWrapWord

	d := dialog.NewCustomConfirm("Move Group", "Move", "Cancel",
		container.NewVBox(
			infoLabel,
			widget.NewSeparator(),
			widget.NewLabel(fmt.Sprintf("Current Group: %s", currentGroup)),
			widget.NewLabel("Select the group to move to:"),
			selectWidget,
		),
		func(ok bool) {
			if !ok || selectWidget.Selected == "" {
				return
			}
			var target thread.GroupType
			switch selectWidget.Selected {
			case thread.GroupSystem.DisplayName():
				target = thread.GroupSystem
			case thread.GroupMain.DisplayName():
				target = thread.GroupMain
			case thread.GroupAuxiliary.DisplayName():
				target = thread.GroupAuxiliary
			}
			mv.onMoveGroup(proc.PID, mv.determineProcessGroup(*proc), target)
			mv.refreshProcesses()
			mv.procList.SetData(mv.classified[mv.selectedTab], mv.selectedTab)
		}, mv.window)
	d.Resize(fyne.NewSize(360, 240))
	d.Show()
}

func (mv *MainView) onPriorityChange(pid uint32, priority string) {
	if err := process.SetProcessPriority(pid, priority); err != nil {
		dialog.ShowError(err, mv.window)
		return
	}

	// Find process name and current group's affinity
	var procName string
	var currentAffinity string
	for gt, procs := range mv.classified {
		for _, p := range procs {
			if p.PID == pid {
				procName = p.Name
				currentAffinity = mv.groups.Get(gt).Affinity()
				break
			}
		}
	}

	if procName != "" {
		mv.removeProcessRule(procName)
		mv.addProcessRule(procName, priority, currentAffinity)
	}

	dialog.ShowInformation("Done", fmt.Sprintf("Changed priority of PID %d to %s.", pid, priority), mv.window)
	mv.procList.SetData(mv.classified[mv.selectedTab], mv.selectedTab)
	if mv.selectedProcess != nil && mv.selectedProcess.PID == pid {
		mv.selectedProcess.Priority = priority
		mv.selectedInfoLabel.SetText(mv.buildSelectedProcessText())
	}
}

func (mv *MainView) onMoveGroup(pid uint32, from, to thread.GroupType) {
	if mv.selectedProcess != nil && mv.selectedProcess.PID == pid {
		mv.selectedProcess = nil
	}
	// Find the process
	var targetProc process.Process
	var found bool
	for _, p := range mv.classified[from] {
		if p.PID == pid {
			targetProc = p
			found = true
			break
		}
	}
	if !found {
		return
	}

	// Get target group's affinity
	targetGroup := mv.groups.Get(to)
	affinity := targetGroup.Affinity()

	// Get current priority
	currentPri := targetProc.Priority

	// Remove old rule and create new one with correct affinity
	mv.removeProcessRule(targetProc.Name)
	mv.addProcessRule(targetProc.Name, currentPri, affinity)

	// Apply immediately via Windows API
	if affinity != "" {
		if cores, err := thread.ParseCoreString(affinity); err == nil {
			process.SetProcessAffinity(pid, cores)
		}
	}

	// Refresh
	mv.classifyAndRefresh()
	mv.updateCards()
	mv.procList.SetData(mv.classified[mv.selectedTab], mv.selectedTab)

	log.Printf("Moved PID %d (%s) to %s (affinity: %s)", pid, targetProc.Name, to.DisplayName(), affinity)
}

// removeProcessRule removes all rules matching the given process name (case-insensitive).
func (mv *MainView) removeProcessRule(procName string) {
	cfg, err := mv.cfgLoader.LoadConfig()
	if err != nil {
		log.Printf("Failed to load config: %v", err)
		return
	}

	nameLower := strings.ToLower(procName)
	var filtered []config.ProcessRule
	for _, rule := range cfg.ProcessRules {
		if strings.ToLower(rule.Selector) != nameLower {
			filtered = append(filtered, rule)
		}
	}
	cfg.ProcessRules = filtered

	if err := mv.cfgLoader.SaveConfig(cfg); err != nil {
		log.Printf("Failed to save config: %v", err)
	}
}

// addProcessRule adds a new process rule with the given priority and affinity.
func (mv *MainView) addProcessRule(procName, priority, affinity string) {
	cfg, err := mv.cfgLoader.LoadConfig()
	if err != nil {
		log.Printf("Failed to load config: %v", err)
		return
	}

	rule := config.ProcessRule{
		SelectorBy: "Name",
		Selector:   procName,
		Force:      "Y",
	}
	if priority != "" {
		rule.Priority = &priority
	}
	if affinity != "" {
		rule.Affinity = &affinity
	}
	cfg.ProcessRules = append(cfg.ProcessRules, rule)

	if err := mv.cfgLoader.SaveConfig(cfg); err != nil {
		log.Printf("Failed to save config: %v", err)
	}
}

func (mv *MainView) showSettingsDialog() {
	// Thread group core settings
	sysEntry := widget.NewEntry()
	sysEntry.SetText(mv.groups.System.Affinity())

	mainEntry := widget.NewEntry()
	mainEntry.SetText(mv.groups.Main.Affinity())

	auxEntry := widget.NewEntry()
	auxEntry.SetText(mv.groups.Auxiliary.Affinity())

	form := widget.NewForm(
		widget.NewFormItem("System Thread Cores", sysEntry),
		widget.NewFormItem("Main Thread Cores", mainEntry),
		widget.NewFormItem("Auxiliary Thread Cores", auxEntry),
	)

	d := dialog.NewCustomConfirm("Thread Group Settings", "Save", "Cancel", form, func(ok bool) {
		if !ok {
			return
		}
		mv.saveThreadGroupSettings(sysEntry.Text, mainEntry.Text, auxEntry.Text)
	}, mv.window)
	d.Resize(fyne.NewSize(450, 250))
	d.Show()
}

func (mv *MainView) saveThreadGroupSettings(sysCores, mainCores, auxCores string) {
	cfg, err := mv.cfgLoader.LoadConfig()
	if err != nil {
		log.Printf("Failed to load config: %v", err)
		return
	}

	if cfg.ThreadGroups == nil {
		cfg.ThreadGroups = &config.ThreadGroupsConfig{}
	}
	cfg.ThreadGroups.System.Cores = sysCores
	cfg.ThreadGroups.Main.Cores = mainCores
	cfg.ThreadGroups.Auxiliary.Cores = auxCores

	if err := mv.cfgLoader.SaveConfig(cfg); err != nil {
		log.Printf("Failed to save config: %v", err)
		return
	}

	// Reload groups
	mv.groups = loadThreadGroups(cfg)

	// Refresh
	mv.classifyAndRefresh()
	mv.updateCards()
	mv.procList.SetData(mv.classified[mv.selectedTab], mv.selectedTab)

	dialog.ShowInformation("Saved", "Thread group settings have been saved.", mv.window)
}

func loadThreadGroups(cfg *config.Config) *thread.Groups {
	totalThreads := thread.GetTotalThreads()
	defaults := thread.CalculateDefaultAllocation(totalThreads)

	if cfg.ThreadGroups != nil {
		// Parse cores from config
		if cores := parseCoreList(cfg.ThreadGroups.System.Cores); len(cores) > 0 {
			defaults.System.Cores = cores
		}
		if cores := parseCoreList(cfg.ThreadGroups.Main.Cores); len(cores) > 0 {
			defaults.Main.Cores = cores
		}
		if cores := parseCoreList(cfg.ThreadGroups.Auxiliary.Cores); len(cores) > 0 {
			defaults.Auxiliary.Cores = cores
		}
	}

	return defaults
}

func parseCoreList(s string) []uint32 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	cores, err := thread.ParseCoreString(s)
	if err != nil {
		return nil
	}
	return cores
}

// Container returns the main view's root container.
func (mv *MainView) Container() fyne.CanvasObject {
	return mv.content
}

// LaunchConfigEditor opens the config.json file in the default editor.
func LaunchConfigEditor(configFile string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("notepad.exe", configFile)
	case "darwin":
		cmd = exec.Command("open", configFile)
	default:
		cmd = exec.Command("xdg-open", configFile)
	}
	if err := cmd.Start(); err != nil {
		log.Printf("Failed to open config file: %v", err)
	}
}

// LaunchGitHub opens the GitHub URL in the default browser.
func LaunchGitHub(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		log.Printf("Failed to open URL: %v", err)
	}
}
