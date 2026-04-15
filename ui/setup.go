package ui

import (
	"fmt"
	"log"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/threadix/backend/config"
	"github.com/threadix/backend/cpu"
	"github.com/threadix/backend/thread"
)

// SetupScreen creates the initial setup screen shown on first run.
// Returns the content widget and a callback for when the user saves.
func SetupScreen(w fyne.Window, cfgLoader *config.Loader, onSave func(*config.Config)) fyne.CanvasObject {
	cpuInfo, err := cpu.GetInfo()
	if err != nil {
		cpuInfo = &cpu.Info{ModelName: "Unknown", LogicalCores: runtime.NumCPU(), Threads: runtime.NumCPU()}
	}

	defaults := thread.CalculateDefaultAllocation(cpuInfo.Threads)

	// --- Left Panel: CPU Info + Description ---
	cpuTitle := widget.NewLabel("CPU Information")
	cpuTitle.TextStyle = fyne.TextStyle{Bold: true}
	cpuTitle.Importance = widget.HighImportance

	cpuDetails := widget.NewRichTextFromMarkdown(fmt.Sprintf(
		"**Model:** %s\n\n**Physical Cores:** %d\n\n**Logical Cores:** %d\n\n**Threads:** %d",
		cpuInfo.ModelName, cpuInfo.PhysicalCores, cpuInfo.LogicalCores, cpuInfo.Threads,
	))

	descTitle := widget.NewLabel("About this App")
	descTitle.TextStyle = fyne.TextStyle{Bold: true}
	descTitle.Importance = widget.HighImportance

	description := widget.NewRichTextFromMarkdown(
		"This app organizes CPU threads into **three categories**\n" +
			"and manages processes accordingly.\n\n" +
			"**System Threads**\n" +
			"Includes core system components like the OS and drivers.\n\n" +
			"**Main Threads**\n" +
			"Used by all regular applications.\n\n" +
			"**Auxiliary Threads**\n" +
			"Contains only processes manually assigned by the user.\n\n" +
			"Set the core ranges for each category on the right.\n" +
			"These values can be changed at any time.",
	)

	leftPanel := container.NewVBox(
		widget.NewCard("", "", container.NewVBox(cpuTitle, cpuDetails)),
		widget.NewSeparator(),
		widget.NewCard("", "", container.NewVBox(descTitle, description)),
	)

	// --- Right Panel: Thread Group Settings ---
	makeGroupCard := func(gt thread.GroupType, defaultCores []uint32) (*widget.Card, *widget.Entry) {
		label := widget.NewLabel(fmt.Sprintf("Cores: %d", len(defaultCores)))
		label.TextStyle = fyne.TextStyle{Italic: true}

		entry := widget.NewEntry()
		entry.SetText(thread.FormatCores(defaultCores))
		entry.PlaceHolder = "e.g.: 0-3"

		resetBtn := widget.NewButtonWithIcon("Restore Defaults", theme.ViewRefreshIcon(), func() {
			def := thread.CalculateDefaultAllocation(cpuInfo.Threads)
			entry.SetText(thread.FormatCores(def.Get(gt).Cores))
		})
		resetBtn.Importance = widget.LowImportance

		content := container.NewVBox(
			container.NewHBox(widget.NewLabel("Core range:"), entry, resetBtn),
			label,
		)

		card := widget.NewCard(gt.DisplayName(), "", content)
		return card, entry
	}

	sysCard, sysEntry := makeGroupCard(thread.GroupSystem, defaults.System.Cores)
	mainCard, mainEntry := makeGroupCard(thread.GroupMain, defaults.Main.Cores)
	auxCard, auxEntry := makeGroupCard(thread.GroupAuxiliary, defaults.Auxiliary.Cores)

	// Save button
	saveBtn := widget.NewButtonWithIcon("Save and Start", theme.ConfirmIcon(), func() {
		// Parse and save
		groups := &config.ThreadGroupsConfig{
			System:    config.ThreadGroupConfig{Cores: sysEntry.Text},
			Main:      config.ThreadGroupConfig{Cores: mainEntry.Text},
			Auxiliary: config.ThreadGroupConfig{Cores: auxEntry.Text},
		}

		// Load existing config or create new
		cfg, err := cfgLoader.LoadConfig()
		if err != nil {
			cfg = &config.Config{
				RuleApplyIntervalSeconds: config.DefaultApplyIntervalSec,
				ProcessRules:             []config.ProcessRule{},
				ServiceRules:             []config.ServiceRule{},
			}
		}
		cfg.ThreadGroups = groups

		if err := cfgLoader.SaveConfig(cfg); err != nil {
			log.Printf("Failed to save config: %v", err)
			return
		}

		if onSave != nil {
			onSave(cfg)
		}
	})
	saveBtn.Importance = widget.HighImportance

	rightPanel := container.NewVBox(
		widget.NewLabelWithStyle("Thread Group Settings", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		sysCard,
		mainCard,
		auxCard,
		layout.NewSpacer(),
		container.NewCenter(saveBtn),
	)

	split := container.NewHSplit(
		container.NewPadded(leftPanel),
		container.NewPadded(rightPanel),
	)
	split.Offset = 0.4

	return split
}
