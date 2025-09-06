// main.go (for Option 1, same package)
package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Global variables for input capture management
var globalApp *tview.Application
var globalRoot tview.Primitive
var globalVMTable *tview.Table
var globalPopulateVMTable func()

// setupGlobalInputCapture sets up the global input capture
func setupGlobalInputCapture() {
	globalApp.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlQ, tcell.KeyEscape:
			globalApp.Stop()
			return nil
		}

		switch event.Rune() {
		case 'q':
			globalApp.Stop()
			return nil
		case 'h':
			// Show help dialog
			showHelp(globalApp, globalRoot)
			return nil
		case 'c':
			// Quick create instance
			quickCreateVM(globalApp, globalVMTable, globalPopulateVMTable, globalRoot)
			return nil
		case '[':
			// Stop selected instance
			stopSelectedVM(globalApp, globalVMTable, globalPopulateVMTable, globalRoot)
			return nil
		case ']':
			// Start selected instance
			startSelectedVM(globalApp, globalVMTable, globalPopulateVMTable, globalRoot)
			return nil
		case 'p':
			// Suspend selected instance
			suspendSelectedVM(globalApp, globalVMTable, globalPopulateVMTable, globalRoot)
			return nil
		case '<':
			// Stop all instances
			stopAllVMs(globalApp, globalVMTable, globalPopulateVMTable, globalRoot)
			return nil
		case '>':
			// Start all instances
			startAllVMs(globalApp, globalVMTable, globalPopulateVMTable, globalRoot)
			return nil
		case 'd':
			// Delete selected instance
			deleteSelectedVM(globalApp, globalVMTable, globalPopulateVMTable, globalRoot)
			return nil
		case 'r':
			// Recover deleted instances
			recoverSelectedVM(globalApp, globalVMTable, globalPopulateVMTable, globalRoot)
			return nil
		case '!':
			// Purge all deleted instances
			purgeAllVMs(globalApp, globalVMTable, globalPopulateVMTable, globalRoot)
			return nil
		case '/':
			// Refresh VM table
			globalPopulateVMTable()
			return nil
		case 's':
			// Shell into selected VM
			shellIntoVM(globalApp, globalVMTable)
			return nil
		case 'n':
			// Create snapshot
			createSnapshot(globalApp, globalVMTable, globalPopulateVMTable, globalRoot)
			return nil
		case 'm':
			// Manage snapshots
			manageSnapshots(globalApp, globalVMTable, globalPopulateVMTable, globalRoot)
			return nil
		case 'v':
			// Show version
			showVersion(globalApp, globalRoot)
			return nil
		}

		return event
	})
}

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

	// Set global variables for input capture management
	globalApp = app
	globalRoot = flex
	globalVMTable = vmTable
	globalPopulateVMTable = populateVMTable

	// Fetch and display VMs at startup
	go func() {
		app.QueueUpdateDraw(populateVMTable)
	}()

	flex.AddItem(vmTable, 0, 1, true) // Make the table focusable

	// Create footer with keyboard shortcuts
	footer := tview.NewTextView()
	footer.SetBorder(true).SetTitle("Shortcuts")
	footer.SetText(`h (Help) | c (Quick Create) | [ (Stop) | ] (Start) | p (Suspend) | < (Stop ALL) | > (Start ALL) | d (Delete) | r (Recover) | ! (Purge ALL) | / (Refresh) | s (Shell) | n (Snapshot) | m (Manage) | v (Version) | q (Quit)`)
	footer.SetTextAlign(tview.AlignCenter)
	footer.SetDynamicColors(true)
	flex.AddItem(footer, 3, 1, false) // Give footer more height (3 lines)

	// Set up the global input capture
	setupGlobalInputCapture()

	if err := app.SetRoot(flex, true).Run(); err != nil {
		log.Fatalf("tview error: %v", err)
	}
}

// Helper functions for keyboard shortcuts
func showVersion(app *tview.Application, root tview.Primitive) {
	modal := tview.NewModal().
		SetText(GetVersion()).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			app.SetRoot(root, true)
		})
	app.SetRoot(modal, false)
}

func showHelp(app *tview.Application, root tview.Primitive) {
	modal := tview.NewModal().
		SetText("Keyboard Shortcuts:\n\nh: Help\nc: Quick Create\n[: Stop\n]: Start\np: Suspend\n<: Stop ALL\n>: Start ALL\nd: Delete\nr: Recover\n!: Purge ALL\n/: Refresh\ns: Shell\nn: Snapshot\nm: Manage Snapshots\nv: Version\nq: Quit").
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
				setupGlobalInputCapture()
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
						setupGlobalInputCapture()
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
						setupGlobalInputCapture()
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
			// Create local copies to avoid closure capturing loop variables
			vmNameCopy := vmName
			iCopy := i
			app.QueueUpdateDraw(func() {
				showLoading(app, fmt.Sprintf("Stopping VM: %s (%d of %d)", vmNameCopy, iCopy+1, len(vmNames)), root)
			})

			_, err := StopVM(vmNameCopy)
			if err != nil {
				app.QueueUpdateDraw(func() {
					showError(app, "Stop All Error", fmt.Sprintf("Failed to stop %s: %v", vmNameCopy, err), root)
				})
				return
			}
		}

		// All completed successfully
		app.QueueUpdateDraw(func() {
			populateVMTable()
			setupGlobalInputCapture()
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
			// Create local copies to avoid closure capturing loop variables
			vmNameCopy := vmName
			iCopy := i
			app.QueueUpdateDraw(func() {
				showLoading(app, fmt.Sprintf("Starting VM: %s (%d of %d)", vmNameCopy, iCopy+1, len(vmNames)), root)
			})

			_, err := StartVM(vmNameCopy)
			if err != nil {
				app.QueueUpdateDraw(func() {
					showError(app, "Start All Error", fmt.Sprintf("Failed to start %s: %v", vmNameCopy, err), root)
				})
				return
			}
		}

		// All completed successfully
		app.QueueUpdateDraw(func() {
			populateVMTable()
			setupGlobalInputCapture()
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

// isVMStopped checks if the selected VM is in a stopped state
func isVMStopped(vmTable *tview.Table) (bool, string) {
	row, _ := vmTable.GetSelection()
	if row > 0 {
		// Get VM name from first column
		nameCell := vmTable.GetCell(row, 0)
		if nameCell == nil {
			return false, ""
		}
		vmName := nameCell.Text

		// Get VM state from second column
		stateCell := vmTable.GetCell(row, 1)
		if stateCell == nil {
			return false, vmName
		}
		vmState := stateCell.Text

		// Check if VM is stopped
		return vmState == "Stopped", vmName
	}
	return false, ""
}

// isVMStoppedByName checks if a VM with the given name is in a stopped state
func isVMStoppedByName(vmTable *tview.Table, vmName string) bool {
	rowCount := vmTable.GetRowCount()
	for row := 1; row < rowCount; row++ { // Skip header row
		nameCell := vmTable.GetCell(row, 0)
		if nameCell != nil && nameCell.Text == vmName {
			stateCell := vmTable.GetCell(row, 1)
			if stateCell != nil {
				return stateCell.Text == "Stopped"
			}
		}
	}
	return false
}

func createSnapshot(app *tview.Application, vmTable *tview.Table, populateVMTable func(), root tview.Primitive) {
	// Check if VM is stopped
	isStopped, vmName := isVMStopped(vmTable)
	if !isStopped {
		if vmName == "" {
			showError(app, "Error", "No VM selected", root)
		} else {
			showError(app, "Snapshot Error", fmt.Sprintf("Snapshot operations are only available on stopped instances.\n\nVM '%s' is not stopped. Please stop the VM first using the '[' key.", vmName), root)
		}
		return
	}

	row, _ := vmTable.GetSelection()
	if row > 0 {
		cell := vmTable.GetCell(row, 0)
		if cell != nil {
			vmName := cell.Text

			// Create a simple form with the input field
			form := tview.NewForm()

			// Add input field
			form.AddInputField("Snapshot name", "", 20, nil, nil)

			// Add Create button
			form.AddButton("Create", func() {
				snapshotName := form.GetFormItem(0).(*tview.InputField).GetText()
				if snapshotName == "" {
					showError(app, "Error", "Snapshot name cannot be empty", root)
					return
				}

				// Show loading popup
				showLoading(app, fmt.Sprintf("Creating snapshot '%s' for VM: %s", snapshotName, vmName), root)

				// Create snapshot in goroutine
				go func() {
					_, err := CreateSnapshot(vmName, snapshotName)
					app.QueueUpdateDraw(func() {
						// Restore global input capture
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
								showHelp(app, root)
								return nil
							case 'c':
								quickCreateVM(app, vmTable, populateVMTable, root)
								return nil
							case '[':
								stopSelectedVM(app, vmTable, populateVMTable, root)
								return nil
							case ']':
								startSelectedVM(app, vmTable, populateVMTable, root)
								return nil
							case 'p':
								suspendSelectedVM(app, vmTable, populateVMTable, root)
								return nil
							case '<':
								stopAllVMs(app, vmTable, populateVMTable, root)
								return nil
							case '>':
								startAllVMs(app, vmTable, populateVMTable, root)
								return nil
							case 'd':
								deleteSelectedVM(app, vmTable, populateVMTable, root)
								return nil
							case 'r':
								recoverSelectedVM(app, vmTable, populateVMTable, root)
								return nil
							case '!':
								purgeAllVMs(app, vmTable, populateVMTable, root)
								return nil
							case '/':
								populateVMTable()
								return nil
							case 's':
								shellIntoVM(app, vmTable)
								return nil
							case 'n':
								createSnapshot(app, vmTable, populateVMTable, root)
								return nil
							}

							return event
						})

						if err != nil {
							showError(app, "Snapshot Error", err.Error(), root)
						} else {
							populateVMTable()
							setupGlobalInputCapture()
							app.SetRoot(root, true) // Return to main interface
						}
					})
				}()
			})

			// Add Cancel button
			form.AddButton("Cancel", func() {
				// Restore global input capture
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
						showHelp(app, root)
						return nil
					case 'c':
						quickCreateVM(app, vmTable, populateVMTable, root)
						return nil
					case '[':
						stopSelectedVM(app, vmTable, populateVMTable, root)
						return nil
					case ']':
						startSelectedVM(app, vmTable, populateVMTable, root)
						return nil
					case 'p':
						suspendSelectedVM(app, vmTable, populateVMTable, root)
						return nil
					case '<':
						stopAllVMs(app, vmTable, populateVMTable, root)
						return nil
					case '>':
						startAllVMs(app, vmTable, populateVMTable, root)
						return nil
					case 'd':
						deleteSelectedVM(app, vmTable, populateVMTable, root)
						return nil
					case 'r':
						recoverSelectedVM(app, vmTable, populateVMTable, root)
						return nil
					case '!':
						purgeAllVMs(app, vmTable, populateVMTable, root)
						return nil
					case '/':
						populateVMTable()
						return nil
					case 's':
						shellIntoVM(app, vmTable)
						return nil
					case 'n':
						createSnapshot(app, vmTable, populateVMTable, root)
						return nil
					}

					return event
				})
				app.SetRoot(root, true)
			})

			form.SetBorder(true).SetTitle(fmt.Sprintf("Create Snapshot for VM: %s", vmName))

			// Temporarily disable global input capture
			app.SetInputCapture(nil)

			// Set up form-specific input capture
			form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				// Handle Escape key to cancel
				if event.Key() == tcell.KeyEscape {
					// Restore global input capture
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
							showHelp(app, root)
							return nil
						case 'c':
							quickCreateVM(app, vmTable, populateVMTable, root)
							return nil
						case '[':
							stopSelectedVM(app, vmTable, populateVMTable, root)
							return nil
						case ']':
							startSelectedVM(app, vmTable, populateVMTable, root)
							return nil
						case 'p':
							suspendSelectedVM(app, vmTable, populateVMTable, root)
							return nil
						case '<':
							stopAllVMs(app, vmTable, populateVMTable, root)
							return nil
						case '>':
							startAllVMs(app, vmTable, populateVMTable, root)
							return nil
						case 'd':
							deleteSelectedVM(app, vmTable, populateVMTable, root)
							return nil
						case 'r':
							recoverSelectedVM(app, vmTable, populateVMTable, root)
							return nil
						case '!':
							purgeAllVMs(app, vmTable, populateVMTable, root)
							return nil
						case '/':
							populateVMTable()
							return nil
						case 's':
							shellIntoVM(app, vmTable)
							return nil
						case 'n':
							createSnapshot(app, vmTable, populateVMTable, root)
							return nil
						}

						return event
					})
					app.SetRoot(root, true)
					return nil
				}
				// Let the form handle all other input
				return event
			})

			app.SetRoot(form, true)
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

func manageSnapshots(app *tview.Application, vmTable *tview.Table, populateVMTable func(), root tview.Primitive) {
	// Check if the selected VM is stopped before allowing access to snapshot management
	isStopped, vmName := isVMStopped(vmTable)
	if !isStopped {
		if vmName == "" {
			showError(app, "Error", "No VM selected", root)
		} else {
			showError(app, "Snapshot Error", fmt.Sprintf("Snapshot operations are only available on stopped instances.\n\nVM '%s' is not stopped. Please stop the VM first using the '[' key.", vmName), root)
		}
		return
	}

	// Get the selected VM name
	row, _ := vmTable.GetSelection()
	if row <= 0 {
		showError(app, "Error", "No VM selected", root)
		return
	}

	vmNameCell := vmTable.GetCell(row, 0)
	if vmNameCell == nil {
		showError(app, "Error", "No VM selected", root)
		return
	}
	selectedVMName := vmNameCell.Text

	// Get list of snapshots
	snapshotsOutput, err := ListSnapshots()
	if err != nil {
		showError(app, "Error", "Failed to get snapshots list", root)
		return
	}

	// Parse snapshots
	allSnapshots := parseSnapshots(snapshotsOutput)

	// Filter snapshots for the selected VM only
	var snapshots []SnapshotInfo
	for _, snapshot := range allSnapshots {
		if snapshot.Instance == selectedVMName {
			snapshots = append(snapshots, snapshot)
		}
	}

	if len(snapshots) == 0 {
		showError(app, "Info", fmt.Sprintf("No snapshots found for VM '%s'", selectedVMName), root)
		return
	}

	// Create snapshot list
	snapshotList := tview.NewList()
	snapshotList.SetBorder(true).SetTitle(fmt.Sprintf("Manage Snapshots - %s", selectedVMName))

	// Add snapshots to list
	for _, snapshot := range snapshots {
		displayText := fmt.Sprintf("%s.%s", snapshot.Instance, snapshot.Name)
		if snapshot.Comment != "" {
			displayText += fmt.Sprintf(" (%s)", snapshot.Comment)
		}
		snapshotList.AddItem(displayText, "", 0, nil)
	}

	// Set up selection handler
	snapshotList.SetSelectedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		if index < len(snapshots) {
			snapshot := snapshots[index]
			showSnapshotActions(app, snapshot, populateVMTable, root)
		}
	})

	// Add close button - disable global input capture temporarily
	snapshotList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			// Restore global input capture before returning to main interface
			setupGlobalInputCapture()
			app.SetRoot(root, true)
			return nil
		}
		return event
	})

	// Temporarily disable global input capture
	app.SetInputCapture(nil)
	app.SetRoot(snapshotList, true)
}

func showSnapshotActions(app *tview.Application, snapshot SnapshotInfo, populateVMTable func(), root tview.Primitive) {
	modal := tview.NewModal().
		SetText(fmt.Sprintf("Snapshot: %s.%s\n\nWhat would you like to do?", snapshot.Instance, snapshot.Name)).
		AddButtons([]string{"Revert", "Delete", "Cancel"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			switch buttonLabel {
			case "Revert":
				// Check if VM is stopped before allowing revert
				if !isVMStoppedByName(globalVMTable, snapshot.Instance) {
					showError(app, "Snapshot Error", fmt.Sprintf("Snapshot operations are only available on stopped instances.\n\nVM '%s' is not stopped. Please stop the VM first using the '[' key.", snapshot.Instance), root)
					return
				}
				// Show confirmation dialog
				confirmModal := tview.NewModal().
					SetText(fmt.Sprintf("Are you sure you want to revert %s to snapshot '%s'?\n\nThis will discard the current state!", snapshot.Instance, snapshot.Name)).
					AddButtons([]string{"Yes", "No"}).
					SetDoneFunc(func(buttonIndex int, buttonLabel string) {
						if buttonLabel == "Yes" {
							// Show loading popup
							showLoading(app, fmt.Sprintf("Reverting %s to snapshot '%s'", snapshot.Instance, snapshot.Name), root)

							// Revert snapshot in goroutine
							go func() {
								_, err := RestoreSnapshot(snapshot.Instance, snapshot.Name)
								app.QueueUpdateDraw(func() {
									if err != nil {
										showError(app, "Revert Error", err.Error(), root)
									} else {
										populateVMTable()
										setupGlobalInputCapture()
										app.SetRoot(root, true) // Return to main interface
									}
								})
							}()
						} else {
							setupGlobalInputCapture()
							app.SetRoot(root, true)
						}
					})
				app.SetRoot(confirmModal, false)
			case "Delete":
				// Check if VM is stopped before allowing delete
				if !isVMStoppedByName(globalVMTable, snapshot.Instance) {
					showError(app, "Snapshot Error", fmt.Sprintf("Snapshot operations are only available on stopped instances.\n\nVM '%s' is not stopped. Please stop the VM first using the '[' key.", snapshot.Instance), root)
					return
				}
				// Show confirmation dialog
				confirmModal := tview.NewModal().
					SetText(fmt.Sprintf("Are you sure you want to delete snapshot '%s' of %s?\n\nThis cannot be undone!", snapshot.Name, snapshot.Instance)).
					AddButtons([]string{"Yes", "No"}).
					SetDoneFunc(func(buttonIndex int, buttonLabel string) {
						if buttonLabel == "Yes" {
							// Show loading popup
							showLoading(app, fmt.Sprintf("Deleting snapshot '%s' of %s", snapshot.Name, snapshot.Instance), root)

							// Delete snapshot in goroutine
							go func() {
								_, err := DeleteSnapshot(snapshot.Instance, snapshot.Name)
								app.QueueUpdateDraw(func() {
									if err != nil {
										showError(app, "Delete Error", err.Error(), root)
									} else {
										populateVMTable()
										setupGlobalInputCapture()
										app.SetRoot(root, true) // Return to main interface
									}
								})
							}()
						} else {
							setupGlobalInputCapture()
							app.SetRoot(root, true)
						}
					})
				app.SetRoot(confirmModal, false)
			case "Cancel":
				setupGlobalInputCapture()
				app.SetRoot(root, true)
			}
		})
	app.SetRoot(modal, false)
}

// SnapshotInfo represents a snapshot
type SnapshotInfo struct {
	Instance string
	Name     string
	Parent   string
	Comment  string
}

// parseSnapshots parses the output from multipass list --snapshots
func parseSnapshots(output string) []SnapshotInfo {
	var snapshots []SnapshotInfo
	lines := strings.Split(output, "\n")

	for i, line := range lines {
		line = strings.TrimSpace(line)

		// Skip header line and empty lines
		if i == 0 || line == "" || strings.Contains(line, "Instance") {
			continue
		}

		// Split by whitespace and take first 4 fields
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			snapshot := SnapshotInfo{
				Instance: fields[0],
				Name:     fields[1],
			}

			if len(fields) >= 3 && fields[2] != "--" {
				snapshot.Parent = fields[2]
			}

			if len(fields) >= 4 && fields[3] != "--" {
				snapshot.Comment = fields[3]
			}

			snapshots = append(snapshots, snapshot)
		}
	}

	return snapshots
}
