// snapshot_operations.go - Snapshot management operations
package main

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// createSnapshot displays a form to create a new snapshot for the selected VM.
// The VM must be in a stopped state before snapshot creation is allowed.
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

			// Add input field for snapshot name with space-to-dash conversion
			form.AddInputField("Snapshot name", "", 20, func(textToCheck string, lastChar rune) bool {
				// If the last character is a space, replace it with a dash
				if lastChar == ' ' {
					// Get the text without the space and add a dash
					textWithoutSpace := textToCheck[:len(textToCheck)-1]
					newText := textWithoutSpace + "-"
					// Update the field with the corrected text
					snapshotNameField := form.GetFormItem(0).(*tview.InputField)
					snapshotNameField.SetText(newText)
					return false // Don't allow the space to be added
				}
				return true // Allow all other characters
			}, nil)

			// Add input field for description with default timestamp
			timestamp := time.Now().Format("2006-01-02_15-04")
			form.AddInputField("Description", timestamp, 30, nil, nil)

			// Add Create button
			form.AddButton("Create", func() {
				snapshotName := form.GetFormItem(0).(*tview.InputField).GetText()
				description := form.GetFormItem(1).(*tview.InputField).GetText()
				if snapshotName == "" {
					showError(app, "Error", "Snapshot name cannot be empty", root)
					return
				}

				// Show loading popup
				showLoadingAnimated(app, fmt.Sprintf("Creating snapshot '%s' for VM: %s", snapshotName, vmName), root)

				// Create snapshot in goroutine
				go func() {
					_, err := CreateSnapshot(vmName, snapshotName, description)
					app.QueueUpdateDraw(func() {
						if err != nil {
							setupGlobalInputCapture()
							showError(app, "Snapshot Error", err.Error(), root)
						} else {
							populateVMTable()
							setupGlobalInputCapture()
							app.SetRoot(root, true)
						}
					})
				}()
			})

			// Add Cancel button
			form.AddButton("Cancel", func() {
				setupGlobalInputCapture()
				app.SetRoot(root, true)
			})

			form.SetBorder(true).SetTitle(fmt.Sprintf("Create Snapshot for VM: %s", vmName))

			// Temporarily disable global input capture
			app.SetInputCapture(nil)

			// Set up form-specific input capture
			form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				if event.Key() == tcell.KeyEscape {
					setupGlobalInputCapture()
					app.SetRoot(root, true)
					return nil
				}
				return event
			})

			app.SetRoot(form, true)
		}
	}
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

	// Create a flex layout for the snapshot manager
	flex := tview.NewFlex().SetDirection(tview.FlexRow)

	// Build snapshot tree structure
	snapshotTree := buildSnapshotTree(snapshots)

	// Create snapshot tree view
	snapshotTreeView := tview.NewTreeView()
	snapshotTreeView.SetBorder(true).SetTitle(fmt.Sprintf("Snapshot Tree for %s", selectedVMName))
	snapshotTreeView.SetBorderPadding(1, 1, 1, 1)
	snapshotTreeView.SetRoot(snapshotTree)
	snapshotTreeView.SetCurrentNode(snapshotTree)

	// Create a details panel to show snapshot information
	detailsPanel := tview.NewTextView()
	detailsPanel.SetBorder(true).SetTitle("Snapshot Details")
	detailsPanel.SetBorderPadding(1, 1, 1, 1)
	detailsPanel.SetDynamicColors(true)
	detailsPanel.SetWrap(true)

	// Set up selection handler for tree view
	snapshotTreeView.SetSelectedFunc(func(node *tview.TreeNode) {
		reference := node.GetReference()
		if reference != nil {
			if snapshot, ok := reference.(SnapshotInfo); ok {
				showSnapshotActions(app, snapshot, populateVMTable, root)
			}
		}
	})

	// Update details when selection changes
	snapshotTreeView.SetChangedFunc(func(node *tview.TreeNode) {
		reference := node.GetReference()
		if reference != nil {
			if snapshot, ok := reference.(SnapshotInfo); ok {
				// Create detailed display
				details := fmt.Sprintf("[::b]Snapshot Name:[::-] %s\n\n", snapshot.Name)
				details += fmt.Sprintf("[::b]VM Instance:[::-] %s\n\n", snapshot.Instance)

				if snapshot.Comment != "" {
					details += fmt.Sprintf("[::b]Description:[::-] %s\n\n", snapshot.Comment)
				} else {
					details += "[::b]Description:[::-] No description provided\n\n"
				}

				if snapshot.Parent != "" {
					details += fmt.Sprintf("[::b]Parent Snapshot:[::-] %s\n\n", snapshot.Parent)
				} else {
					details += "[::b]Parent Snapshot:[::-] Root snapshot\n\n"
				}

				details += "[::d]Press Enter to manage this snapshot[::-]"

				detailsPanel.SetText(details)
			}
		}
	})

	// Add close button - disable global input capture temporarily
	snapshotTreeView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			// Restore global input capture before returning to main interface
			setupGlobalInputCapture()
			app.SetRoot(root, true)
			return nil
		}
		return event
	})

	// Create horizontal flex for tree and details
	horizontalFlex := tview.NewFlex().SetDirection(tview.FlexColumn)
	horizontalFlex.AddItem(snapshotTreeView, 0, 1, true) // Tree takes up left side
	horizontalFlex.AddItem(detailsPanel, 0, 1, false)    // Details take up right side

	// Add instructions at the bottom
	instructions := tview.NewTextView()
	instructions.SetText("Use ↑↓←→ to navigate tree • Space/Enter to expand/manage • Esc to return")
	instructions.SetTextAlign(tview.AlignCenter)
	instructions.SetTextColor(tview.Styles.SecondaryTextColor)

	// Add everything to the main flex
	flex.AddItem(horizontalFlex, 0, 1, true)
	flex.AddItem(instructions, 1, 1, false)

	// Temporarily disable global input capture
	app.SetInputCapture(nil)
	app.SetRoot(flex, true)
}

func showSnapshotActions(app *tview.Application, snapshot SnapshotInfo, populateVMTable func(), root tview.Primitive) {
	// Create a more detailed modal text
	var modalText string
	modalText += "[::b]Snapshot Management[::-]\n\n"
	modalText += fmt.Sprintf("[::b]Name:[::-] %s\n", snapshot.Name)
	modalText += fmt.Sprintf("[::b]VM:[::-] %s\n", snapshot.Instance)

	if snapshot.Comment != "" {
		modalText += fmt.Sprintf("[::b]Description:[::-] %s\n", snapshot.Comment)
	}

	modalText += "\nWhat would you like to do?"

	modal := tview.NewModal().
		SetText(modalText).
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
							showLoadingAnimated(app, fmt.Sprintf("Reverting %s to snapshot '%s'", snapshot.Instance, snapshot.Name), root)

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
							showLoadingAnimated(app, fmt.Sprintf("Deleting snapshot '%s' of %s", snapshot.Name, snapshot.Instance), root)

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
