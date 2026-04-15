package ui

import (
	"fmt"
	"image/color"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/threadix/backend/process"
	"github.com/threadix/backend/thread"
)

// ProcessListView creates a process list with search and action buttons.
type ProcessListView struct {
	table       *widget.Table
	searchEntry *widget.Entry
	processes   []process.Process
	filtered    []process.Process
	selectedRow int
	groupType   thread.GroupType
	onRefresh   func() []process.Process
	onPriority  func(pid uint32, priority string)
	onMoveGroup func(pid uint32, from, to thread.GroupType)
	onSelect    func(*process.Process)
	window      fyne.Window
	statusLabel *widget.Label
}

// NewProcessListView creates a new process list view.
func NewProcessListView(w fyne.Window) *ProcessListView {
	v := &ProcessListView{
		selectedRow: -1,
		window:      w,
		statusLabel: widget.NewLabel(""),
	}

	// Search entry
	v.searchEntry = widget.NewEntry()
	v.searchEntry.PlaceHolder = "Search processes..."
	v.searchEntry.OnChanged = func(text string) {
		v.filterProcesses(text)
	}

	// Table
	v.table = widget.NewTable(
		func() (int, int) {
			return len(v.filtered) + 1, 4
		},
		func() fyne.CanvasObject {
			label := widget.NewLabel("template")
			label.Truncation = fyne.TextTruncateEllipsis
			rect := canvas.NewRectangle(theme.InputBackgroundColor())
			rect.SetMinSize(fyne.NewSize(1, 32))
			return container.NewMax(rect, label)
		},
		func(i widget.TableCellID, o fyne.CanvasObject) {
			cellRect := o.(*fyne.Container).Objects[0].(*canvas.Rectangle)
			label := o.(*fyne.Container).Objects[1].(*widget.Label)
			if i.Row == 0 {
				cellRect.FillColor = theme.HeaderBackgroundColor()
				cellRect.Refresh()
				label.TextStyle = fyne.TextStyle{Bold: true}
				label.Alignment = fyne.TextAlignCenter
				switch i.Col {
				case 0:
					label.SetText("PID")
				case 1:
					label.SetText("Process Name")
				case 2:
					label.SetText("Priority")
				case 3:
					label.SetText("Affinity")
				}
				return
			}

			selected := i.Row-1 == v.selectedRow
			if selected {
				cellRect.FillColor = color.NRGBA{R: 46, G: 92, B: 156, A: 255}
				label.TextStyle = fyne.TextStyle{Bold: true}
			} else {
				cellRect.FillColor = theme.InputBackgroundColor()
				label.TextStyle = fyne.TextStyle{}
			}
			cellRect.Refresh()

			label.Alignment = fyne.TextAlignLeading
			if i.Row-1 < len(v.filtered) {
				proc := v.filtered[i.Row-1]
				switch i.Col {
				case 0:
					label.SetText(fmt.Sprintf("%d", proc.PID))
				case 1:
					label.SetText(proc.Name)
				case 2:
					label.SetText(proc.Priority)
				case 3:
					label.SetText(proc.AffinityStr)
				}
			}
		},
	)

	v.table.SetColumnWidth(0, 70)
	v.table.SetColumnWidth(1, 220)
	v.table.SetColumnWidth(2, 110)
	v.table.SetColumnWidth(3, 130)

	// Handle row selection
	v.table.OnSelected = func(id widget.TableCellID) {
		if id.Row > 0 && id.Row-1 < len(v.filtered) {
			v.selectedRow = id.Row - 1
			v.table.Refresh()
			proc := v.filtered[v.selectedRow]
			v.statusLabel.SetText(fmt.Sprintf("Selected: PID %d | %s | %s | %s", proc.PID, proc.Name, proc.Priority, proc.AffinityStr))
			if v.onSelect != nil {
				v.onSelect(&proc)
			}
		} else {
			v.selectedRow = -1
			v.table.Refresh()
			v.statusLabel.SetText(fmt.Sprintf("%s | Processes: %d", v.groupType.DisplayName(), len(v.filtered)))
			if v.onSelect != nil {
				v.onSelect(nil)
			}
		}
	}

	return v
}

// SetData sets the process data and refreshes the view.
func (v *ProcessListView) SetData(processes []process.Process, groupType thread.GroupType) {
	v.processes = processes
	v.groupType = groupType
	v.selectedRow = -1
	v.table.UnselectAll()
	v.filterProcesses(v.searchEntry.Text)
	v.updateStatus()
}

// SetCallbacks sets the callback functions.
func (v *ProcessListView) SetCallbacks(
	onRefresh func() []process.Process,
	onPriority func(uint32, string),
	onMoveGroup func(uint32, thread.GroupType, thread.GroupType),
	onSelect func(*process.Process),
) {
	v.onRefresh = onRefresh
	v.onPriority = onPriority
	v.onMoveGroup = onMoveGroup
	v.onSelect = onSelect
}

func (v *ProcessListView) SelectedProcess() *process.Process {
	if v.selectedRow >= 0 && v.selectedRow < len(v.filtered) {
		return &v.filtered[v.selectedRow]
	}
	return nil
}

func (v *ProcessListView) filterProcesses(query string) {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		v.filtered = make([]process.Process, len(v.processes))
		copy(v.filtered, v.processes)
	} else {
		v.filtered = nil
		for _, p := range v.processes {
			if strings.Contains(strings.ToLower(p.Name), query) ||
				strings.Contains(strings.ToLower(p.ExePath), query) ||
				fmt.Sprintf("%d", p.PID) == query {
				v.filtered = append(v.filtered, p)
			}
		}
	}
	v.table.Refresh()
	v.updateStatus()
}

func (v *ProcessListView) updateStatus() {
	v.statusLabel.SetText(fmt.Sprintf("%s | Processes: %d", v.groupType.DisplayName(), len(v.filtered)))
}

// Container returns the full process list view with search, actions, and table.
func (v *ProcessListView) Container() fyne.CanvasObject {
	refreshBtn := widget.NewButtonWithIcon("Refresh", theme.ViewRefreshIcon(), func() {
		if v.onRefresh != nil {
			v.processes = v.onRefresh()
			v.filterProcesses(v.searchEntry.Text)
		}
	})

	searchRow := container.NewBorder(nil, nil, nil, refreshBtn, v.searchEntry)

	searchCard := widget.NewCard("Search & Actions", "Search processes and manage selected items.", container.NewVBox(searchRow))
	tableScroll := container.NewScroll(v.table)
	tableScroll.SetMinSize(fyne.NewSize(0, 320))
	listContent := container.NewBorder(nil, v.statusLabel, nil, nil, tableScroll)

	return container.NewBorder(searchCard, nil, nil, nil, listContent)
}

func (v *ProcessListView) showPriorityDialog() {
	priorities := []string{"Idle", "BelowNormal", "Normal", "AboveNormal", "High", "Realtime"}
	proc := v.filtered[v.selectedRow]
	selectWidget := widget.NewSelect(priorities, nil)
	selectWidget.PlaceHolder = "Select priority..."
	infoLabel := widget.NewLabel(fmt.Sprintf("Selected process: PID %d | %s | Current %s | %s", proc.PID, proc.Name, proc.Priority, proc.AffinityStr))

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
			proc := v.filtered[v.selectedRow]
			if v.onPriority != nil {
				v.onPriority(proc.PID, selectWidget.Selected)
			}
			// Refresh data
			if v.onRefresh != nil {
				v.processes = v.onRefresh()
				v.filterProcesses(v.searchEntry.Text)
			}
		}, v.window)
	d.Resize(fyne.NewSize(300, 180))
	d.Show()
}

func (v *ProcessListView) showMoveGroupDialog() {
	groups := []string{
		thread.GroupSystem.DisplayName(),
		thread.GroupMain.DisplayName(),
		thread.GroupAuxiliary.DisplayName(),
	}

	// Remove current group from options
	currentName := v.groupType.DisplayName()
	var options []string
	for _, g := range groups {
		if g != currentName {
			options = append(options, g)
		}
	}

	proc := v.filtered[v.selectedRow]
	selectWidget := widget.NewSelect(options, nil)
	selectWidget.PlaceHolder = "Select target group..."
	infoLabel := widget.NewLabel(fmt.Sprintf("Selected process: PID %d | %s | Current %s | %s", proc.PID, proc.Name, proc.Priority, proc.AffinityStr))

	d := dialog.NewCustomConfirm("Move Group", "Move", "Cancel",
		container.NewVBox(
			infoLabel,
			widget.NewSeparator(),
			widget.NewLabel(fmt.Sprintf("Current Group: %s", currentName)),
			widget.NewLabel("Select the group to move to:"),
			selectWidget,
		),
		func(ok bool) {
			if !ok || selectWidget.Selected == "" {
				return
			}
			proc := v.filtered[v.selectedRow]
			var target thread.GroupType
			switch selectWidget.Selected {
			case thread.GroupSystem.DisplayName():
				target = thread.GroupSystem
			case thread.GroupMain.DisplayName():
				target = thread.GroupMain
			case thread.GroupAuxiliary.DisplayName():
				target = thread.GroupAuxiliary
			}
			if v.onMoveGroup != nil {
				v.onMoveGroup(proc.PID, v.groupType, target)
			}
		}, v.window)
	d.Resize(fyne.NewSize(350, 220))
	d.Show()
}

// GetProcesses returns the current filtered process list.
func (v *ProcessListView) GetProcesses() []process.Process {
	return v.filtered
}

// SortByColumn sorts the process list by the given column.
func SortByColumn(procs []process.Process, col int, ascending bool) {
	sort.Slice(procs, func(i, j int) bool {
		var less bool
		switch col {
		case 0:
			less = procs[i].PID < procs[j].PID
		case 1:
			less = procs[i].Name < procs[j].Name
		case 2:
			less = procs[i].Priority < procs[j].Priority
		default:
			less = procs[i].PID < procs[j].PID
		}
		if ascending {
			return less
		}
		return !less
	})
}
