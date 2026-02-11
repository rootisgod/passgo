// main.go - Multipass VM management tool with TUI interface
package main

import (
	"fmt"       // For formatted printing (like printf in other languages)
	"log"       // For logging errors and messages
	"os"        // For file operations
	"path/filepath"
	"strings" // For string manipulation functions

	// External libraries (installed via 'go mod')
	"github.com/rivo/tview" // Terminal UI library for creating text-based interfaces
)

// Global variables for UI state management
var globalApp *tview.Application // Pointer to the main TUI application
var globalRoot tview.Primitive   // The root UI element (main container)
var globalVMTable *tview.Table   // Pointer to the VM table widget
var globalPopulateVMTable func() // Function variable to refresh the VM table
var appLogger *log.Logger        // File-backed application logger

// main() sets up the TUI and starts the application
func main() {
	// Initialize file logger early
	if err := initLogger(); err != nil {
		log.Printf("logger init failed: %v", err)
	} else {
		appLogger.Println("passgo starting up")
	}
	app := tview.NewApplication()
	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	vmTable := tview.NewTable()
	vmTable.SetBorder(true).SetTitle("Multipass VMs")
	vmTable.SetSelectable(true, false)

	vmTable.SetSelectedFunc(func(row, column int) {
		if row > 0 {
			app.Stop()
		}
	})

	headers := []string{"Name", "State", "Snapshots", "IPv4", "Release", "CPUs", "Disk Usage", "Memory Usage", "Mounts"}
	for i, header := range headers {
		vmTable.SetCell(0, i, tview.NewTableCell(header).SetTextColor(tview.Styles.SecondaryTextColor).SetAlign(tview.AlignLeft))
	}

	vmTable.SetSelectionChangedFunc(func(row, column int) {
		if row == 0 && vmTable.GetRowCount() > 1 {
			vmTable.Select(1, 0)
		}
	})

	clearVMRows := func() {
		for i := vmTable.GetRowCount() - 1; i > 0; i-- {
			vmTable.RemoveRow(i)
		}
	}

	populateVMTable := func() {
		clearVMRows()

		listOutput, err := ListVMs()
		if err != nil {
			vmTable.SetCell(1, 0, tview.NewTableCell("Error fetching VMs").SetTextColor(tview.Styles.PrimaryTextColor))
			for i := 1; i < 9; i++ {
				vmTable.SetCell(1, i, tview.NewTableCell("").SetTextColor(tview.Styles.PrimaryTextColor))
			}
			return
		}

		lines := strings.Split(listOutput, "\n")
		vmNames := []string{}
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.Contains(line, "Name") || strings.Contains(line, "---") {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) >= 4 {
				vmNames = append(vmNames, fields[0])
			}
		}

		if len(vmNames) == 0 {
			vmTable.SetCell(1, 0, tview.NewTableCell("No VMs found").SetTextColor(tview.Styles.PrimaryTextColor))
			for i := 1; i < 9; i++ {
				vmTable.SetCell(1, i, tview.NewTableCell("").SetTextColor(tview.Styles.PrimaryTextColor))
			}
			return
		}

		row := 1

		for _, vmName := range vmNames {
			infoOutput, err := GetVMInfo(vmName)

			if err != nil {
				vmTable.SetCell(row, 0, tview.NewTableCell(vmName).SetTextColor(tview.Styles.PrimaryTextColor))
				vmTable.SetCell(row, 1, tview.NewTableCell("Error").SetTextColor(tview.Styles.PrimaryTextColor))
				for i := 2; i < 9; i++ {
					vmTable.SetCell(row, i, tview.NewTableCell("--").SetTextColor(tview.Styles.PrimaryTextColor))
				}
			} else {
				vm := parseVMInfo(infoOutput)
				vmTable.SetCell(row, 0, tview.NewTableCell(vm.Name).SetTextColor(tview.Styles.PrimaryTextColor))
				vmTable.SetCell(row, 1, tview.NewTableCell(vm.State).SetTextColor(tview.Styles.PrimaryTextColor))
				vmTable.SetCell(row, 2, tview.NewTableCell(vm.Snapshots).SetTextColor(tview.Styles.PrimaryTextColor))
				vmTable.SetCell(row, 3, tview.NewTableCell(vm.IPv4).SetTextColor(tview.Styles.PrimaryTextColor))
				vmTable.SetCell(row, 4, tview.NewTableCell(vm.Release).SetTextColor(tview.Styles.PrimaryTextColor))
				vmTable.SetCell(row, 5, tview.NewTableCell(vm.CPUs).SetTextColor(tview.Styles.PrimaryTextColor))
				vmTable.SetCell(row, 6, tview.NewTableCell(vm.DiskUsage).SetTextColor(tview.Styles.PrimaryTextColor))
				vmTable.SetCell(row, 7, tview.NewTableCell(vm.MemoryUsage).SetTextColor(tview.Styles.PrimaryTextColor))
				vmTable.SetCell(row, 8, tview.NewTableCell(vm.Mounts).SetTextColor(tview.Styles.PrimaryTextColor))
			}

			row++
		}

		if vmTable.GetRowCount() > 1 {
			vmTable.Select(1, 0)
		}
	}

	globalApp = app
	globalRoot = flex
	globalVMTable = vmTable
	globalPopulateVMTable = populateVMTable

	go func() {
		app.QueueUpdateDraw(populateVMTable)
	}()

	flex.AddItem(vmTable, 0, 1, true)

	footerFlex := tview.NewFlex().SetDirection(tview.FlexRow)
	footerLine1 := tview.NewTextView()
	footerLine1.SetText("[yellow]c[white] Quick Create  [yellow]C[white] Advanced Create  [yellow][[white] Stop  [yellow]][white] Start  [yellow]p[white] Suspend  [yellow]<[white] Stop ALL  [yellow]>[white] Start ALL")
	footerLine1.SetTextAlign(tview.AlignCenter)
	footerLine1.SetDynamicColors(true)
	footerLine1.SetWrap(false)

	footerLine2 := tview.NewTextView()
	footerLine2.SetText("[yellow]d[white] Delete  [yellow]r[white] Recover  [yellow]![white] Purge ALL  [yellow]/[white] Refresh  [yellow]s[white] Shell  [yellow]n[white] New Snapshot  [yellow]m[white] Manage Snapshots  [yellow]q[white] Quit")
	footerLine2.SetTextAlign(tview.AlignCenter)
	footerLine2.SetDynamicColors(true)
	footerLine2.SetWrap(false)

	footerFlex.AddItem(footerLine1, 1, 1, false)
	footerFlex.AddItem(footerLine2, 1, 1, false)
	footerFlex.SetBorder(true).SetTitle("Shortcuts")

	flex.AddItem(footerFlex, 4, 1, false)

	setupGlobalInputCapture()

	if err := app.SetRoot(flex, true).Run(); err != nil {
		log.Fatalf("tview error: %v", err)
	}
}

// initLogger creates ~/.passgo/passgo.log and wires a global logger
func initLogger() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	logDir := filepath.Join(home, ".passgo")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return err
	}
	logPath := filepath.Join(logDir, "passgo.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	appLogger = log.New(f, "", log.LstdFlags)
	return nil
}
