// main.go (for Option 1, same package)
package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func main() {
	app := tview.NewApplication()
	flex := tview.NewFlex().SetDirection(tview.FlexRow)

	// Create selectable VM table
	vmTable := tview.NewTable()
	vmTable.SetBorder(true).SetTitle("Multipass VMs")
	vmTable.SetSelectable(true, false) // Allow row selection, not column selection
	vmTable.SetSelectedFunc(func(row, column int) {
		// Only allow selection of VM rows (skip header row 0)
		if row > 0 {
			// Handle VM selection - you can add actions here later
			app.Stop()
		}
	})

	// Set table styling with all fields
	headers := []string{"Name", "State", "Snapshots", "IPv4", "Release", "CPUs", "Disk Usage", "Memory Usage", "Mounts"}
	for i, header := range headers {
		vmTable.SetCell(0, i, tview.NewTableCell(header).SetTextColor(tview.Styles.SecondaryTextColor).SetAlign(tview.AlignLeft))
	}

	// Set initial selection to first VM row (row 1) instead of header (row 0)
	vmTable.SetSelectionChangedFunc(func(row, column int) {
		// If we're on the header row, move to the first VM row
		if row == 0 && vmTable.GetRowCount() > 1 {
			vmTable.Select(1, 0)
		}
	})

	// Helper function to clear VM rows (keep header)
	clearVMRows := func() {
		for i := vmTable.GetRowCount() - 1; i > 0; i-- {
			vmTable.RemoveRow(i)
		}
	}

	// VMInfo represents detailed VM information
	type VMInfo struct {
		Name        string
		State       string
		Snapshots   string
		IPv4        string
		Release     string
		CPUs        string
		DiskUsage   string
		MemoryUsage string
		Mounts      string
	}

	// Parse detailed VM info from multipass info output
	parseVMInfo := func(info string) VMInfo {
		vm := VMInfo{}
		lines := strings.Split(info, "\n")

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.Contains(line, ":") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					value := strings.TrimSpace(parts[1])

					switch key {
					case "Name":
						vm.Name = value
					case "State":
						vm.State = value
					case "Snapshots":
						vm.Snapshots = value
					case "IPv4":
						vm.IPv4 = value
					case "Release":
						vm.Release = value
					case "CPU(s)":
						vm.CPUs = value
					case "Disk usage":
						vm.DiskUsage = value
					case "Memory usage":
						vm.MemoryUsage = value
					case "Mounts":
						vm.Mounts = value
					}
				}
			}
		}

		return vm
	}

	// Function to populate the table with detailed VM info
	populateVMTable := func() {
		clearVMRows()

		// First get the list of VMs
		listOutput, err := ListVMs()
		if err != nil {
			vmTable.SetCell(1, 0, tview.NewTableCell("Error fetching VMs").SetTextColor(tview.Styles.PrimaryTextColor))
			for i := 1; i < 9; i++ {
				vmTable.SetCell(1, i, tview.NewTableCell("").SetTextColor(tview.Styles.PrimaryTextColor))
			}
			return
		}

		// Parse VM names from list output
		lines := strings.Split(listOutput, "\n")
		vmNames := []string{}
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.Contains(line, "Name") || strings.Contains(line, "---") {
				continue // Skip header and separator lines
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

		// Get detailed info for each VM
		row := 1
		for _, vmName := range vmNames {
			infoOutput, err := GetVMInfo(vmName)
			if err != nil {
				// If we can't get detailed info, show basic info
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

		// Set selection to first VM row (row 1) after populating
		if vmTable.GetRowCount() > 1 {
			vmTable.Select(1, 0)
		}
	}

	// Fetch and display VMs at startup
	go func() {
		app.QueueUpdateDraw(populateVMTable)
	}()

	flex.AddItem(vmTable, 0, 1, true) // Make the table focusable

	// Create footer with keyboard shortcuts
	footer := tview.NewTextView()
	footer.SetBorder(true).SetTitle("Shortcuts")
	footer.SetText(`h (Help) | c (Quick Create) | [ (Stop) | ] (Start) | p (Suspend) | < (Stop ALL) | > (Start ALL) | d (Delete) | r (Recover) | ! (Purge ALL) | / (Refresh) | s (Shell) | q (Quit)`)
	footer.SetTextAlign(tview.AlignCenter)
	footer.SetDynamicColors(true)
	flex.AddItem(footer, 3, 1, false) // Give footer more height (3 lines)

	// Store reference to root for modal dialogs
	root := flex

	// Add keyboard input handling
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlQ, tcell.KeyEscape:
			app.Stop()
			return nil
		}

		switch event.Rune() {
		case 'q':
			app.Stop()
			return nil
		case 'h':
			// Show help dialog
			showHelp(app, root)
			return nil
		case 'c':
			// Quick create instance
			quickCreateVM(app, vmTable, populateVMTable, root)
			return nil
		case '[':
			// Stop selected instance
			stopSelectedVM(app, vmTable, populateVMTable, root)
			return nil
		case ']':
			// Start selected instance
			startSelectedVM(app, vmTable, populateVMTable, root)
			return nil
		case 'p':
			// Suspend selected instance
			suspendSelectedVM(app, vmTable, populateVMTable, root)
			return nil
		case '<':
			// Stop all instances
			stopAllVMs(app, vmTable, populateVMTable, root)
			return nil
		case '>':
			// Start all instances
			startAllVMs(app, vmTable, populateVMTable, root)
			return nil
		case 'd':
			// Delete selected instance
			deleteSelectedVM(app, vmTable, populateVMTable, root)
			return nil
		case 'r':
			// Recover selected instance
			recoverSelectedVM(app, vmTable, populateVMTable, root)
			return nil
		case '!':
			// Purge all instances
			purgeAllVMs(app, vmTable, populateVMTable, root)
			return nil
		case '/':
			// Refresh table
			populateVMTable()
			return nil
		case 's':
			// Shell into selected instance
			shellIntoVM(app, vmTable)
			return nil
		}

		return event
	})

	if err := app.SetRoot(flex, true).Run(); err != nil {
		log.Fatalf("tview error: %v", err)
	}
}

// Helper functions for keyboard shortcuts
func showHelp(app *tview.Application, root tview.Primitive) {
	modal := tview.NewModal().
		SetText("Keyboard Shortcuts:\n\nh: Help\nc: Quick Create\n[: Stop\n]: Start\np: Suspend\n<: Stop ALL\n>: Start ALL\nd: Delete\nr: Recover\n!: Purge ALL\n/: Refresh\ns: Shell\nq: Quit").
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			app.SetRoot(root, true)
		})
	app.SetRoot(modal, false)
}

func quickCreateVM(app *tview.Application, vmTable *tview.Table, populateVMTable func(), root tview.Primitive) {
	// Show loading popup
	showLoading(app, "Creating VM", root)

	// Run the operation in a goroutine to avoid blocking the UI
	go func() {
		_, err := LaunchVM("test-vm", "22.04")
		app.QueueUpdateDraw(func() {
			if err != nil {
				showError(app, "Launch Error", err.Error(), root)
			} else {
				populateVMTable()
				app.SetRoot(root, true) // Return to main interface
			}
		})
	}()
}

func stopSelectedVM(app *tview.Application, vmTable *tview.Table, populateVMTable func(), root tview.Primitive) {
	row, _ := vmTable.GetSelection()
	if row > 0 {
		cell := vmTable.GetCell(row, 0)
		if cell != nil {
			vmName := cell.Text
			showLoading(app, fmt.Sprintf("Stopping VM: %s", vmName), root)

			go func() {
				_, err := StopVM(vmName)
				app.QueueUpdateDraw(func() {
					if err != nil {
						showError(app, "Stop Error", err.Error(), root)
					} else {
						populateVMTable()
						app.SetRoot(root, true) // Return to main interface
					}
				})
			}()
		}
	}
}

func startSelectedVM(app *tview.Application, vmTable *tview.Table, populateVMTable func(), root tview.Primitive) {
	row, _ := vmTable.GetSelection()
	if row > 0 {
		cell := vmTable.GetCell(row, 0)
		if cell != nil {
			vmName := cell.Text
			showLoading(app, fmt.Sprintf("Starting VM: %s", vmName), root)

			go func() {
				_, err := StartVM(vmName)
				app.QueueUpdateDraw(func() {
					if err != nil {
						showError(app, "Start Error", err.Error(), root)
					} else {
						populateVMTable()
						app.SetRoot(root, true) // Return to main interface
					}
				})
			}()
		}
	}
}

func suspendSelectedVM(app *tview.Application, vmTable *tview.Table, populateVMTable func(), root tview.Primitive) {
	row, _ := vmTable.GetSelection()
	if row > 0 {
		cell := vmTable.GetCell(row, 0)
		if cell != nil {
			vmName := cell.Text
			_, err := runMultipassCommand("suspend", vmName)
			if err != nil {
				showError(app, "Suspend Error", err.Error(), root)
			} else {
				populateVMTable()
			}
		}
	}
}

func stopAllVMs(app *tview.Application, vmTable *tview.Table, populateVMTable func(), root tview.Primitive) {
	// Get list of VMs first
	listOutput, err := ListVMs()
	if err != nil {
		showError(app, "Error", "Failed to get VM list", root)
		return
	}

	// Parse VM names
	vmNames := parseVMNames(listOutput)
	if len(vmNames) == 0 {
		showError(app, "Info", "No VMs found to stop", root)
		return
	}

	// Show initial loading with count
	showLoading(app, fmt.Sprintf("Stopping all VMs (%d total)", len(vmNames)), root)

	go func() {
		// Process each VM individually to show progress
		for i, vmName := range vmNames {
			app.QueueUpdateDraw(func() {
				showLoading(app, fmt.Sprintf("Stopping VM: %s (%d of %d)", vmName, i+1, len(vmNames)), root)
			})

			_, err := StopVM(vmName)
			if err != nil {
				app.QueueUpdateDraw(func() {
					showError(app, "Stop All Error", fmt.Sprintf("Failed to stop %s: %v", vmName, err), root)
				})
				return
			}
		}

		// All completed successfully
		app.QueueUpdateDraw(func() {
			populateVMTable()
			app.SetRoot(root, true)
		})
	}()
}

func startAllVMs(app *tview.Application, vmTable *tview.Table, populateVMTable func(), root tview.Primitive) {
	// Get list of VMs first
	listOutput, err := ListVMs()
	if err != nil {
		showError(app, "Error", "Failed to get VM list", root)
		return
	}

	// Parse VM names
	vmNames := parseVMNames(listOutput)
	if len(vmNames) == 0 {
		showError(app, "Info", "No VMs found to start", root)
		return
	}

	// Show initial loading with count
	showLoading(app, fmt.Sprintf("Starting all VMs (%d total)", len(vmNames)), root)

	go func() {
		// Process each VM individually to show progress
		for i, vmName := range vmNames {
			app.QueueUpdateDraw(func() {
				showLoading(app, fmt.Sprintf("Starting VM: %s (%d of %d)", vmName, i+1, len(vmNames)), root)
			})

			_, err := StartVM(vmName)
			if err != nil {
				app.QueueUpdateDraw(func() {
					showError(app, "Start All Error", fmt.Sprintf("Failed to start %s: %v", vmName, err), root)
				})
				return
			}
		}

		// All completed successfully
		app.QueueUpdateDraw(func() {
			populateVMTable()
			app.SetRoot(root, true)
		})
	}()
}

func deleteSelectedVM(app *tview.Application, vmTable *tview.Table, populateVMTable func(), root tview.Primitive) {
	row, _ := vmTable.GetSelection()
	if row > 0 {
		cell := vmTable.GetCell(row, 0)
		if cell != nil {
			vmName := cell.Text
			modal := tview.NewModal().
				SetText("Are you sure you want to delete " + vmName + "?").
				AddButtons([]string{"Yes", "No"}).
				SetDoneFunc(func(buttonIndex int, buttonLabel string) {
					if buttonLabel == "Yes" {
						_, err := DeleteVM(vmName, false)
						if err != nil {
							showError(app, "Delete Error", err.Error(), root)
						} else {
							populateVMTable()
						}
					}
					app.SetRoot(root, true)
				})
			app.SetRoot(modal, false)
		}
	}
}

func recoverSelectedVM(app *tview.Application, vmTable *tview.Table, populateVMTable func(), root tview.Primitive) {
	row, _ := vmTable.GetSelection()
	if row > 0 {
		cell := vmTable.GetCell(row, 0)
		if cell != nil {
			vmName := cell.Text
			_, err := runMultipassCommand("recover", vmName)
			if err != nil {
				showError(app, "Recover Error", err.Error(), root)
			} else {
				populateVMTable()
			}
		}
	}
}

func purgeAllVMs(app *tview.Application, vmTable *tview.Table, populateVMTable func(), root tview.Primitive) {
	modal := tview.NewModal().
		SetText("Are you sure you want to PURGE ALL VMs? This cannot be undone!").
		AddButtons([]string{"Yes", "No"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if buttonLabel == "Yes" {
				_, err := runMultipassCommand("purge")
				if err != nil {
					showError(app, "Purge Error", err.Error(), root)
				} else {
					populateVMTable()
				}
			}
			app.SetRoot(root, true)
		})
	app.SetRoot(modal, false)
}

func shellIntoVM(app *tview.Application, vmTable *tview.Table) {
	row, _ := vmTable.GetSelection()
	if row > 0 {
		cell := vmTable.GetCell(row, 0)
		if cell != nil {
			vmName := cell.Text
			app.Stop()
			// Note: This would need to be implemented to actually shell into the VM
			// For now, we'll just show a message
			log.Printf("Would shell into VM: %s", vmName)
		}
	}
}

func showError(app *tview.Application, title, message string, root tview.Primitive) {
	modal := tview.NewModal().
		SetText(title + ": " + message).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			app.SetRoot(root, true)
		})
	app.SetRoot(modal, false)
}

// showLoading displays a loading popup for long-running operations
func showLoading(app *tview.Application, message string, root tview.Primitive) {
	modal := tview.NewModal().
		SetText(message + "\n\nPlease wait...").
		AddButtons([]string{}) // No buttons - just loading
	app.SetRoot(modal, false)
}

// parseVMNames extracts VM names from multipass list output
func parseVMNames(listOutput string) []string {
	lines := strings.Split(listOutput, "\n")
	vmNames := []string{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "Name") || strings.Contains(line, "---") {
			continue // Skip header and separator lines
		}

		fields := strings.Fields(line)
		if len(fields) >= 4 {
			vmNames = append(vmNames, fields[0])
		}
	}
	return vmNames
}
