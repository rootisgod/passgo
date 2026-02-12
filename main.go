// main.go - Multipass VM management tool with TUI interface
package main

import (
	"fmt"       // For formatted printing (like printf in other languages)
	"log"       // For logging errors and messages
	"os"        // For file operations
	"path/filepath"
	"strings" // For string manipulation functions

	// External libraries (installed via 'go mod')
	"github.com/gdamore/tcell/v2" // Terminal cell library for events
	"github.com/rivo/tview"       // Terminal UI library for creating text-based interfaces
)

// Global variables for UI state management
var globalApp *tview.Application // Pointer to the main TUI application
var globalRoot tview.Primitive   // The root UI element (main container)
var globalVMTable *tview.Table   // Pointer to the VM table widget
var globalPopulateVMTable func() // Function variable to refresh the VM table
var appLogger *log.Logger        // File-backed application logger

// Global variables for filtering and sorting
var globalFilterText string   // Current filter text for VM names
var globalSortColumn int      // Current sort column (0=Name, 1=State, etc.)
var globalSortAscending bool  // Sort direction (true=ascending, false=descending)
var globalFilterInput *tview.InputField // Filter input field
var globalMainFlex *tview.Flex          // Main flex container for showing/hiding filter

// vmData holds VM information and any errors from fetching it
type vmData struct {
	info VMInfo
	err  error
}

// sortVMs sorts a slice of VM data by the specified column and direction
func sortVMs(vms []vmData, column int, ascending bool) {
	// Simple bubble sort for simplicity
	n := len(vms)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			var compare bool
			switch column {
			case 0: // Name
				compare = strings.ToLower(vms[j].info.Name) > strings.ToLower(vms[j+1].info.Name)
			case 1: // State
				compare = strings.ToLower(vms[j].info.State) > strings.ToLower(vms[j+1].info.State)
			case 2: // Snapshots
				compare = vms[j].info.Snapshots > vms[j+1].info.Snapshots
			case 3: // IPv4
				compare = vms[j].info.IPv4 > vms[j+1].info.IPv4
			case 4: // Release
				compare = vms[j].info.Release > vms[j+1].info.Release
			case 5: // CPUs
				compare = vms[j].info.CPUs > vms[j+1].info.CPUs
			case 6: // Disk Usage
				compare = vms[j].info.DiskUsage > vms[j+1].info.DiskUsage
			case 7: // Memory Usage
				compare = vms[j].info.MemoryUsage > vms[j+1].info.MemoryUsage
			case 8: // Mounts
				compare = vms[j].info.Mounts > vms[j+1].info.Mounts
			default:
				compare = false
			}

			if !ascending {
				compare = !compare
			}

			if compare {
				vms[j], vms[j+1] = vms[j+1], vms[j]
			}
		}
	}
}

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

	// Enable mouse support for header clicks
	app.EnableMouse(true)

	vmTable.SetSelectedFunc(func(row, column int) {
		if row > 0 {
			app.Stop()
		}
	})

	// Initialize sort state (default: sort by Name, ascending)
	globalSortColumn = 0
	globalSortAscending = true

	// Function to update table headers with sort indicators
	updateHeaders := func() {
		headers := []string{"Name", "State", "Snapshots", "IPv4", "Release", "CPUs", "Disk Usage", "Memory Usage", "Mounts"}
		for i, header := range headers {
			// Add sort indicator to the current sort column
			displayText := header
			if i == globalSortColumn {
				if globalSortAscending {
					displayText = header + " ▲"
				} else {
					displayText = header + " ▼"
				}
			}
			vmTable.SetCell(0, i, tview.NewTableCell(displayText).SetTextColor(tview.Styles.SecondaryTextColor).SetAlign(tview.AlignLeft))
		}
	}

	updateHeaders()

	// Handle mouse clicks on headers for sorting
	vmTable.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		if action == tview.MouseLeftClick {
			clickRow, clickCol := event.Position()

			// Translate screen coordinates to table coordinates
			x, y, width, _ := vmTable.GetInnerRect()
			if clickRow == y && clickCol >= x && clickCol < x+width {
				// Click is in the header row
				// Determine which column was clicked
				colX := x
				for c := 0; c < vmTable.GetColumnCount(); c++ {
					cell := vmTable.GetCell(0, c)
					if cell != nil {
						colWidth := len(cell.Text) + 2 // Approximate column width
						if clickCol >= colX && clickCol < colX+colWidth {
							// Clicked on column c
							if globalSortColumn == c {
								// Toggle sort direction
								globalSortAscending = !globalSortAscending
							} else {
								// Change sort column
								globalSortColumn = c
								globalSortAscending = true
							}
							updateHeaders()
							// Call populateVMTable (defined below)
							go func() {
								globalApp.QueueUpdateDraw(func() {
									globalPopulateVMTable()
								})
							}()
							return action, nil
						}
						colX += colWidth
					}
				}
			}
		}
		return action, event
	})

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

		// Collect all VM data
		allVMs := make([]vmData, 0, len(vmNames))

		for _, vmName := range vmNames {
			infoOutput, err := GetVMInfo(vmName)
			if err != nil {
				allVMs = append(allVMs, vmData{
					info: VMInfo{Name: vmName, State: "Error"},
					err:  err,
				})
			} else {
				allVMs = append(allVMs, vmData{
					info: parseVMInfo(infoOutput),
					err:  nil,
				})
			}
		}

		// Apply filter
		filteredVMs := []vmData{}
		filterLower := strings.ToLower(globalFilterText)
		for _, vm := range allVMs {
			if globalFilterText == "" || strings.Contains(strings.ToLower(vm.info.Name), filterLower) {
				filteredVMs = append(filteredVMs, vm)
			}
		}

		// Apply sorting
		sortVMs(filteredVMs, globalSortColumn, globalSortAscending)

		// Display filtered and sorted VMs
		if len(filteredVMs) == 0 {
			vmTable.SetCell(1, 0, tview.NewTableCell("No matching VMs").SetTextColor(tview.Styles.PrimaryTextColor))
			for i := 1; i < 9; i++ {
				vmTable.SetCell(1, i, tview.NewTableCell("").SetTextColor(tview.Styles.PrimaryTextColor))
			}
			return
		}

		row := 1
		for _, vm := range filteredVMs {
			if vm.err != nil {
				vmTable.SetCell(row, 0, tview.NewTableCell(vm.info.Name).SetTextColor(tview.Styles.PrimaryTextColor))
				vmTable.SetCell(row, 1, tview.NewTableCell("Error").SetTextColor(tview.Styles.PrimaryTextColor))
				for i := 2; i < 9; i++ {
					vmTable.SetCell(row, i, tview.NewTableCell("--").SetTextColor(tview.Styles.PrimaryTextColor))
				}
			} else {
				vmTable.SetCell(row, 0, tview.NewTableCell(vm.info.Name).SetTextColor(tview.Styles.PrimaryTextColor))
				vmTable.SetCell(row, 1, tview.NewTableCell(vm.info.State).SetTextColor(tview.Styles.PrimaryTextColor))
				vmTable.SetCell(row, 2, tview.NewTableCell(vm.info.Snapshots).SetTextColor(tview.Styles.PrimaryTextColor))
				vmTable.SetCell(row, 3, tview.NewTableCell(vm.info.IPv4).SetTextColor(tview.Styles.PrimaryTextColor))
				vmTable.SetCell(row, 4, tview.NewTableCell(vm.info.Release).SetTextColor(tview.Styles.PrimaryTextColor))
				vmTable.SetCell(row, 5, tview.NewTableCell(vm.info.CPUs).SetTextColor(tview.Styles.PrimaryTextColor))
				vmTable.SetCell(row, 6, tview.NewTableCell(vm.info.DiskUsage).SetTextColor(tview.Styles.PrimaryTextColor))
				vmTable.SetCell(row, 7, tview.NewTableCell(vm.info.MemoryUsage).SetTextColor(tview.Styles.PrimaryTextColor))
				vmTable.SetCell(row, 8, tview.NewTableCell(vm.info.Mounts).SetTextColor(tview.Styles.PrimaryTextColor))
			}
			row++
		}

		if vmTable.GetRowCount() > 1 {
			vmTable.Select(1, 0)
		}
	}

	// Create filter input field (initially hidden)
	filterInput := tview.NewInputField()
	filterInput.SetLabel("Filter: ")
	filterInput.SetFieldWidth(30)
	filterInput.SetChangedFunc(func(text string) {
		globalFilterText = text
		populateVMTable()
	})
	filterInput.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEscape || key == tcell.KeyEnter {
			// Hide filter and restore focus to table
			flex.RemoveItem(filterInput)
			app.SetFocus(vmTable)
		}
	})

	globalApp = app
	globalRoot = flex
	globalVMTable = vmTable
	globalPopulateVMTable = populateVMTable
	globalFilterInput = filterInput
	globalMainFlex = flex

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
	footerLine2.SetText("[yellow]d[white] Delete  [yellow]r[white] Recover  [yellow]![white] Purge ALL  [yellow]/[white] Refresh  [yellow]f[white] Filter  [yellow]s[white] Shell  [yellow]n[white] Snapshot  [yellow]m[white] Snapshots  [yellow]q[white] Quit")
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
