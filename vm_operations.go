// vm_operations.go - VM lifecycle operations (create, start, stop, delete, etc.)
package main

import (
	"fmt"
	"log"
	"strconv"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func quickCreateVM(app *tview.Application, vmTable *tview.Table, populateVMTable func(), root tview.Primitive) {
	vmName := "VM-" + randomString(4)
	showLoadingAnimated(app, "Creating VM: "+vmName, root)

	go func() {
		vmName := "VM-" + randomString(4)
		_, err := LaunchVM(vmName, "24.04")
		app.QueueUpdateDraw(func() {
			if err != nil {
				showError(app, "Launch Error", err.Error(), root)
			} else {
				populateVMTable()
				setupGlobalInputCapture()
				app.SetRoot(root, true)
			}
		})
	}()
}

func createAdvancedVM(app *tview.Application, vmTable *tview.Table, populateVMTable func(), root tview.Primitive) {
	// Available Ubuntu releases
	releases := []string{
		"22.04",
		"20.04",
		"18.04",
		"24.04",
		"daily",
	}

	// Collect cloud-init templates (local + repo via .config)
	templateOptions, cleanupDirs, _ := GetAllCloudInitTemplateOptions()
	// Build dropdown labels with a leading "None"
	cloudInitOptions := make([]string, 0, len(templateOptions)+1)
	cloudInitOptions = append(cloudInitOptions, "None")
	for _, opt := range templateOptions {
		cloudInitOptions = append(cloudInitOptions, opt.Label)
	}

	// Create the form
	form := tview.NewForm()

	// Instance Name input field
	form.AddInputField("Instance Name:", "", 20, nil, nil)

	// Instance Type dropdown (default to Ubuntu 24.04)
	releaseIndex := 3 // Index for "24.04"
	form.AddDropDown("Instance Type:", releases, releaseIndex, nil)

	// CPU Cores input field (default 2) - numeric only
	form.AddInputField("CPU Cores:", "2", 10, func(textToCheck string, lastChar rune) bool {
		// Only allow digits
		return lastChar >= '0' && lastChar <= '9'
	}, nil)

	// RAM input field (default 2048MB) - numeric only
	form.AddInputField("RAM (MB):", "1024", 10, func(textToCheck string, lastChar rune) bool {
		// Only allow digits
		return lastChar >= '0' && lastChar <= '9'
	}, nil)

	// Disk GB input field (default 8GB) - numeric only
	form.AddInputField("Disk (GB):", "8", 10, func(textToCheck string, lastChar rune) bool {
		// Only allow digits
		return lastChar >= '0' && lastChar <= '9'
	}, nil)

	// Cloud-init file dropdown (default to "None")
	form.AddDropDown("Cloud-init File:", cloudInitOptions, 0, nil)

	// Add Create button
	form.AddButton("Create", func() {
		// Get form values
		vmName := form.GetFormItem(0).(*tview.InputField).GetText()
		_, release := form.GetFormItem(1).(*tview.DropDown).GetCurrentOption()
		cpuText := form.GetFormItem(2).(*tview.InputField).GetText()
		memoryText := form.GetFormItem(3).(*tview.InputField).GetText()
		diskText := form.GetFormItem(4).(*tview.InputField).GetText()
		selectedIndex, selectedLabel := form.GetFormItem(5).(*tview.DropDown).GetCurrentOption()

		// Validate inputs
		if vmName == "" {
			showError(app, "Validation Error", "Instance name cannot be empty", root)
			return
		}

		// Parse numeric values
		var cpus, memoryMB, diskGB int
		if _, err := fmt.Sscanf(cpuText, "%d", &cpus); err != nil || cpus < 1 {
			showError(app, "Validation Error", "CPU cores must be a positive integer", root)
			return
		}
		if _, err := fmt.Sscanf(memoryText, "%d", &memoryMB); err != nil || memoryMB < 512 {
			showError(app, "Validation Error", "RAM must be at least 512MB", root)
			return
		}
		if _, err := fmt.Sscanf(diskText, "%d", &diskGB); err != nil || diskGB < 1 {
			showError(app, "Validation Error", "Disk size must be at least 1GB", root)
			return
		}

		// Show loading popup
		showLoadingAnimated(app, fmt.Sprintf("Creating VM: %s", vmName), root)

		// Run the operation in a goroutine to avoid blocking the UI
		go func() {
			var err error
			// Map selected label to actual file path
			var selectedPath string
			if selectedIndex > 0 { // index 0 is "None"
				// templateOptions is aligned with labels starting at index 1
				idx := selectedIndex - 1
				if idx >= 0 && idx < len(templateOptions) {
					selectedPath = templateOptions[idx].Path
				}
			}

			if selectedIndex == 0 || selectedLabel == "None" || selectedPath == "" {
				_, err = LaunchVMAdvanced(vmName, release, cpus, memoryMB, diskGB)
			} else {
				_, err = LaunchVMWithCloudInit(vmName, release, cpus, memoryMB, diskGB, selectedPath)
			}
			app.QueueUpdateDraw(func() {
				if err != nil {
					showError(app, "Launch Error", err.Error(), root)
				} else {
					populateVMTable()
					setupGlobalInputCapture()
					app.SetRoot(root, true) // Return to main interface
				}
				// Cleanup any temp dirs from repo clone
				CleanupTempDirs(cleanupDirs)
			})
		}()
	})

	// Add Cancel button
	form.AddButton("Cancel", func() {
		// Restore global input capture
		setupGlobalInputCapture()
		app.SetRoot(root, true)
	})

	form.SetBorder(true).SetTitle("Create New Instance")

	// Create a flex layout to add footer with instructions
	flex := tview.NewFlex().SetDirection(tview.FlexRow)

	// Add the form
	flex.AddItem(form, 0, 1, true)

	// Add footer with instructions
	footer := tview.NewTextView()
	footer.SetText("Use < > to adjust CPU/RAM/Disk values")
	footer.SetTextAlign(tview.AlignCenter)
	footer.SetTextColor(tview.Styles.SecondaryTextColor)
	footer.SetBorder(true).SetTitle("Instructions")

	flex.AddItem(footer, 3, 1, false) // Footer takes 3 lines

	// Temporarily disable global input capture
	app.SetInputCapture(nil)

	// Set up form-specific input capture
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Handle Escape key to cancel
		if event.Key() == tcell.KeyEscape {
			setupGlobalInputCapture()
			app.SetRoot(root, true)
			return nil
		}

		// Handle < and , keys for decreasing values
		if event.Rune() == '<' || event.Rune() == ',' {
			currentItem, _ := form.GetFocusedItemIndex()
			// Check if we're on CPU Cores (index 2), RAM (index 3), or Disk (index 4)
			if currentItem == 2 { // CPU Cores
				cpuField := form.GetFormItem(2).(*tview.InputField)
				if currentVal, err := strconv.Atoi(cpuField.GetText()); err == nil && currentVal > 1 {
					cpuField.SetText(strconv.Itoa(currentVal - 1))
				}
				return nil
			} else if currentItem == 3 { // RAM
				ramField := form.GetFormItem(3).(*tview.InputField)
				if currentVal, err := strconv.Atoi(ramField.GetText()); err == nil && currentVal > 1024 {
					ramField.SetText(strconv.Itoa(currentVal - 1024))
				}
				return nil
			} else if currentItem == 4 { // Disk
				diskField := form.GetFormItem(4).(*tview.InputField)
				if currentVal, err := strconv.Atoi(diskField.GetText()); err == nil && currentVal > 8 {
					diskField.SetText(strconv.Itoa(currentVal - 8))
				}
				return nil
			}
		}

		// Handle > and . keys for increasing values
		if event.Rune() == '>' || event.Rune() == '.' {
			currentItem, _ := form.GetFocusedItemIndex()
			// Check if we're on CPU Cores (index 2), RAM (index 3), or Disk (index 4)
			if currentItem == 2 { // CPU Cores
				cpuField := form.GetFormItem(2).(*tview.InputField)
				if currentVal, err := strconv.Atoi(cpuField.GetText()); err == nil {
					cpuField.SetText(strconv.Itoa(currentVal + 1))
				}
				return nil
			} else if currentItem == 3 { // RAM
				ramField := form.GetFormItem(3).(*tview.InputField)
				if currentVal, err := strconv.Atoi(ramField.GetText()); err == nil {
					ramField.SetText(strconv.Itoa(currentVal + 1024))
				}
				return nil
			} else if currentItem == 4 { // Disk
				diskField := form.GetFormItem(4).(*tview.InputField)
				if currentVal, err := strconv.Atoi(diskField.GetText()); err == nil {
					diskField.SetText(strconv.Itoa(currentVal + 8))
				}
				return nil
			}
		}

		// Let the form handle all other input
		return event
	})

	app.SetRoot(flex, true)
}

func stopSelectedVM(app *tview.Application, vmTable *tview.Table, populateVMTable func(), root tview.Primitive) {
	row, _ := vmTable.GetSelection()
	if row > 0 {
		cell := vmTable.GetCell(row, 0)
		if cell != nil {
			vmName := cell.Text
			showLoadingAnimated(app, fmt.Sprintf("Stopping VM: %s", vmName), root)

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
			showLoadingAnimated(app, fmt.Sprintf("Starting VM: %s", vmName), root)

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
	showLoadingAnimated(app, fmt.Sprintf("Stopping all VMs (%d total)", len(vmNames)), root)

	go func() {
		// Process each VM individually to show progress
		for i, vmName := range vmNames {
			// Create local copies to avoid closure capturing loop variables
			vmNameCopy := vmName
			iCopy := i
			app.QueueUpdateDraw(func() {
				showLoadingAnimated(app, fmt.Sprintf("Stopping VM: %s (%d of %d)", vmNameCopy, iCopy+1, len(vmNames)), root)
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
	showLoadingAnimated(app, fmt.Sprintf("Starting all VMs (%d total)", len(vmNames)), root)

	go func() {
		// Process each VM individually to show progress
		for i, vmName := range vmNames {
			// Create local copies to avoid closure capturing loop variables
			vmNameCopy := vmName
			iCopy := i
			app.QueueUpdateDraw(func() {
				showLoadingAnimated(app, fmt.Sprintf("Starting VM: %s (%d of %d)", vmNameCopy, iCopy+1, len(vmNames)), root)
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

			// Check if VM is running
			stateCell := vmTable.GetCell(row, 1)
			if stateCell == nil || stateCell.Text != "Running" {
				showError(app, "Shell Error", fmt.Sprintf("VM '%s' is not running. Please start the VM first using the ']' key.", vmName), globalRoot)
				return
			}

			// Run shell in a goroutine to avoid blocking
			go func() {

				// Suspend the TUI application
				app.Suspend(func() {
					// Launch the shell session
					err := ShellVM(vmName)
					if err != nil {
						// If shell fails, show error when we resume
						log.Printf("Shell session failed: %v", err)
					}
				})

				// When we return from the shell, refresh the VM table
				app.QueueUpdateDraw(func() {
					globalPopulateVMTable()
					setupGlobalInputCapture()
					app.SetRoot(globalRoot, true)
				})
			}()
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
