// mount_operations.go - Mount management with TUI file picker
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// MountInfo represents a mount point between local filesystem and VM
type MountInfo struct {
	SourcePath string
	TargetPath string
	UIDMaps    []string
	GIDMaps    []string
}

// --- JSON parsing types for multipass info --format json ---

type multipassInfoResponse struct {
	Errors []string                          `json:"errors"`
	Info   map[string]multipassVMInfoDetail `json:"info"`
}

type multipassVMInfoDetail struct {
	Mounts map[string]multipassMountDetail `json:"mounts"`
}

type multipassMountDetail struct {
	SourcePath  string   `json:"source_path"`
	GIDMappings []string `json:"gid_mappings"`
	UIDMappings []string `json:"uid_mappings"`
}

// getVMMounts retrieves the current mounts for a VM using JSON output
func getVMMounts(vmName string) ([]MountInfo, error) {
	output, err := runMultipassCommand("info", vmName, "--format", "json")
	if err != nil {
		return nil, err
	}

	var response multipassInfoResponse
	if err := json.Unmarshal([]byte(output), &response); err != nil {
		return nil, fmt.Errorf("failed to parse VM info JSON: %v", err)
	}

	vmDetail, ok := response.Info[vmName]
	if !ok {
		return nil, fmt.Errorf("VM '%s' not found in info response", vmName)
	}

	var mounts []MountInfo
	for targetPath, detail := range vmDetail.Mounts {
		mounts = append(mounts, MountInfo{
			SourcePath: detail.SourcePath,
			TargetPath: targetPath,
			UIDMaps:    detail.UIDMappings,
			GIDMaps:    detail.GIDMappings,
		})
	}

	// Sort by target path for consistent display
	sort.Slice(mounts, func(i, j int) bool {
		return mounts[i].TargetPath < mounts[j].TargetPath
	})

	return mounts, nil
}

// ─── Mount Manager Entry Point ─────────────────────────────────────────────────

// manageMounts is the entry point for mount management from the main view
func manageMounts(app *tview.Application, vmTable *tview.Table, populateVMTable func(), root tview.Primitive) {
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

	// Mounting requires a running instance
	stateCell := vmTable.GetCell(row, 1)
	if stateCell == nil || stateCell.Text != "Running" {
		showError(app, "Mount Error",
			fmt.Sprintf("Mount operations require a running VM.\n\nVM '%s' is not running. Please start it first with ']'.", selectedVMName), root)
		return
	}

	// Get current mounts
	mounts, err := getVMMounts(selectedVMName)
	if err != nil {
		if appLogger != nil {
			appLogger.Printf("Failed to get mounts for %s: %v", selectedVMName, err)
		}
		mounts = []MountInfo{}
	}

	showMountManager(app, selectedVMName, mounts, populateVMTable, root)
}

// ─── Mount Manager View ────────────────────────────────────────────────────────

// showMountManager renders the mount management view with a table and details panel
func showMountManager(app *tview.Application, vmName string, mounts []MountInfo, populateVMTable func(), root tview.Primitive) {
	// === Mount Table ===
	mountTable := tview.NewTable()
	mountTable.SetBorder(true).SetTitle(fmt.Sprintf(" Mounts for: %s (%d) ", vmName, len(mounts)))
	mountTable.SetSelectable(true, false)
	mountTable.SetBorderPadding(0, 0, 1, 1)

	// Table headers
	mountTable.SetCell(0, 0, tview.NewTableCell("Source (Local)").
		SetTextColor(tview.Styles.SecondaryTextColor).SetSelectable(false).SetExpansion(2))
	mountTable.SetCell(0, 1, tview.NewTableCell(" → ").
		SetTextColor(tcell.ColorYellow).SetSelectable(false))
	mountTable.SetCell(0, 2, tview.NewTableCell("Target (VM)").
		SetTextColor(tview.Styles.SecondaryTextColor).SetSelectable(false).SetExpansion(2))

	// Populate table rows
	if len(mounts) == 0 {
		mountTable.SetCell(1, 0, tview.NewTableCell("No mounts configured").
			SetTextColor(tcell.ColorGray))
		mountTable.SetCell(1, 1, tview.NewTableCell("").SetSelectable(false))
		mountTable.SetCell(1, 2, tview.NewTableCell("Press 'a' to add a mount").
			SetTextColor(tcell.ColorGray))
	} else {
		for i, m := range mounts {
			mountTable.SetCell(i+1, 0, tview.NewTableCell(m.SourcePath).
				SetTextColor(tview.Styles.PrimaryTextColor).SetExpansion(2))
			mountTable.SetCell(i+1, 1, tview.NewTableCell(" → ").
				SetTextColor(tcell.ColorYellow).SetSelectable(false))
			mountTable.SetCell(i+1, 2, tview.NewTableCell(m.TargetPath).
				SetTextColor(tview.Styles.PrimaryTextColor).SetExpansion(2))
		}
		mountTable.Select(1, 0)
	}

	// === Details Panel ===
	detailsPanel := tview.NewTextView()
	detailsPanel.SetBorder(true).SetTitle(" Mount Details ")
	detailsPanel.SetBorderPadding(1, 1, 1, 1)
	detailsPanel.SetDynamicColors(true)
	detailsPanel.SetWrap(true)

	updateDetails := func(idx int) {
		if idx >= 0 && idx < len(mounts) {
			m := mounts[idx]
			d := fmt.Sprintf("[::b]Source Path:[::-]\n  %s\n\n", m.SourcePath)
			d += fmt.Sprintf("[::b]Target Path:[::-]\n  %s\n\n", m.TargetPath)
			if len(m.UIDMaps) > 0 {
				d += fmt.Sprintf("[::b]UID Mapping:[::-]\n  %s\n\n", strings.Join(m.UIDMaps, ", "))
			}
			if len(m.GIDMaps) > 0 {
				d += fmt.Sprintf("[::b]GID Mapping:[::-]\n  %s\n\n", strings.Join(m.GIDMaps, ", "))
			}
			d += "[::d]Enter: Actions  a: Add  e: Modify  d: Delete[::-]"
			detailsPanel.SetText(d)
		} else {
			detailsPanel.SetText("[::d]No mounts to display[::-]\n\nPress [yellow]a[::-] to add a new mount point.")
		}
	}

	if len(mounts) > 0 {
		updateDetails(0)
	} else {
		updateDetails(-1)
	}

	mountTable.SetSelectionChangedFunc(func(row, column int) {
		updateDetails(row - 1)
	})

	// Enter key shows actions modal
	mountTable.SetSelectedFunc(func(row, column int) {
		if row > 0 && row <= len(mounts) {
			showMountActions(app, vmName, mounts[row-1], populateVMTable, root)
		}
	})

	// === Layout ===
	contentFlex := tview.NewFlex().SetDirection(tview.FlexColumn)
	contentFlex.AddItem(mountTable, 0, 3, true)
	contentFlex.AddItem(detailsPanel, 0, 2, false)

	instructions := tview.NewTextView()
	instructions.SetText("[yellow]a[white] Add  [yellow]e[white] Modify  [yellow]d[white] Delete  [yellow]Enter[white] Actions  [yellow]Esc[white] Return")
	instructions.SetTextAlign(tview.AlignCenter)
	instructions.SetDynamicColors(true)

	mainFlex := tview.NewFlex().SetDirection(tview.FlexRow)
	mainFlex.AddItem(contentFlex, 0, 1, true)
	mainFlex.AddItem(instructions, 1, 0, false)

	// === Input ===
	app.SetInputCapture(nil)

	mountTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			setupGlobalInputCapture()
			populateVMTable()
			app.SetRoot(root, true)
			return nil
		}

		switch event.Rune() {
		case 'a':
			showFilePickerForMount(app, vmName, populateVMTable, root)
			return nil
		case 'd':
			row, _ := mountTable.GetSelection()
			if row > 0 && row <= len(mounts) {
				confirmDeleteMount(app, vmName, mounts[row-1], populateVMTable, root)
			}
			return nil
		case 'e':
			row, _ := mountTable.GetSelection()
			if row > 0 && row <= len(mounts) {
				showModifyMountForm(app, vmName, mounts[row-1], populateVMTable, root)
			}
			return nil
		}

		return event
	})

	app.SetRoot(mainFlex, true)
}

// ─── Mount Actions ─────────────────────────────────────────────────────────────

// showMountActions shows a modal with actions for a selected mount
func showMountActions(app *tview.Application, vmName string, mount MountInfo, populateVMTable func(), root tview.Primitive) {
	text := fmt.Sprintf("[::b]Mount:[::-]\n%s → %s\n\nWhat would you like to do?", mount.SourcePath, mount.TargetPath)
	modal := tview.NewModal().
		SetText(text).
		AddButtons([]string{"Modify", "Delete", "Cancel"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			switch buttonLabel {
			case "Modify":
				showModifyMountForm(app, vmName, mount, populateVMTable, root)
			case "Delete":
				confirmDeleteMount(app, vmName, mount, populateVMTable, root)
			default:
				mounts, _ := getVMMounts(vmName)
				showMountManager(app, vmName, mounts, populateVMTable, root)
			}
		})
	app.SetRoot(modal, false)
}

// confirmDeleteMount shows a confirmation dialog for unmounting
func confirmDeleteMount(app *tview.Application, vmName string, mount MountInfo, populateVMTable func(), root tview.Primitive) {
	modal := tview.NewModal().
		SetText(fmt.Sprintf("Unmount '%s' from VM '%s'?\n\nSource: %s\nTarget: %s",
			mount.TargetPath, vmName, mount.SourcePath, mount.TargetPath)).
		AddButtons([]string{"Unmount", "Cancel"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if buttonLabel == "Unmount" {
				showLoadingAnimated(app,
					fmt.Sprintf("Unmounting %s from %s", mount.TargetPath, vmName), root)

				go func() {
					if appLogger != nil {
						appLogger.Printf("Unmounting %s:%s", vmName, mount.TargetPath)
					}
					_, err := runMultipassCommand("umount", vmName+":"+mount.TargetPath)
					app.QueueUpdateDraw(func() {
						if err != nil {
							if appLogger != nil {
								appLogger.Printf("Failed to unmount %s:%s: %v", vmName, mount.TargetPath, err)
							}
							showError(app, "Unmount Error", err.Error(), root)
						} else {
							if appLogger != nil {
								appLogger.Printf("Successfully unmounted %s:%s", vmName, mount.TargetPath)
							}
							newMounts, _ := getVMMounts(vmName)
							showMountManager(app, vmName, newMounts, populateVMTable, root)
						}
					})
				}()
			} else {
				mounts, _ := getVMMounts(vmName)
				showMountManager(app, vmName, mounts, populateVMTable, root)
			}
		})
	app.SetRoot(modal, false)
}

// ─── Modify Mount ──────────────────────────────────────────────────────────────

// showModifyMountForm shows a form to modify an existing mount (unmount old, mount new)
func showModifyMountForm(app *tview.Application, vmName string, currentMount MountInfo, populateVMTable func(), root tview.Primitive) {
	form := tview.NewForm()
	form.AddInputField("Source (Local):", currentMount.SourcePath, 60, nil, nil)
	form.AddInputField("Target (VM):", currentMount.TargetPath, 60, nil, nil)

	form.AddButton("Save", func() {
		newSource := form.GetFormItem(0).(*tview.InputField).GetText()
		newTarget := form.GetFormItem(1).(*tview.InputField).GetText()

		if newSource == "" || newTarget == "" {
			showError(app, "Validation Error", "Both source and target paths are required", form)
			return
		}

		// Nothing changed?
		if newSource == currentMount.SourcePath && newTarget == currentMount.TargetPath {
			mounts, _ := getVMMounts(vmName)
			showMountManager(app, vmName, mounts, populateVMTable, root)
			return
		}

		info, err := os.Stat(newSource)
		if err != nil || !info.IsDir() {
			showError(app, "Validation Error",
				fmt.Sprintf("Source '%s' must be an existing directory", newSource), form)
			return
		}

		showLoadingAnimated(app, "Updating mount...", root)

		go func() {
			if appLogger != nil {
				appLogger.Printf("Modifying mount: unmounting %s:%s", vmName, currentMount.TargetPath)
			}
			_, err := runMultipassCommand("umount", vmName+":"+currentMount.TargetPath)
			if err != nil {
				app.QueueUpdateDraw(func() {
					if appLogger != nil {
						appLogger.Printf("Failed to unmount during modify: %v", err)
					}
					showError(app, "Unmount Error",
						fmt.Sprintf("Failed to remove old mount: %v", err), root)
				})
				return
			}

			if appLogger != nil {
				appLogger.Printf("Mounting %s to %s:%s", newSource, vmName, newTarget)
			}
			_, err = runMultipassCommand("mount", newSource, vmName+":"+newTarget)
			app.QueueUpdateDraw(func() {
				if err != nil {
					if appLogger != nil {
						appLogger.Printf("Mount failed during modify: %v", err)
					}
					showError(app, "Mount Error",
						fmt.Sprintf("Old mount removed but new mount failed: %v", err), root)
				} else {
					if appLogger != nil {
						appLogger.Printf("Successfully modified mount to %s:%s", vmName, newTarget)
					}
					newMounts, _ := getVMMounts(vmName)
					showMountManager(app, vmName, newMounts, populateVMTable, root)
				}
			})
		}()
	})

	form.AddButton("Browse", func() {
		showFilePicker(app,
			func(sourcePath string) {
				// Return to modify form with updated source
				modified := currentMount
				modified.SourcePath = sourcePath
				showModifyMountForm(app, vmName, modified, populateVMTable, root)
			},
			func() {
				// Cancelled file picker, return to modify form
				showModifyMountForm(app, vmName, currentMount, populateVMTable, root)
			},
		)
	})

	form.AddButton("Cancel", func() {
		mounts, _ := getVMMounts(vmName)
		showMountManager(app, vmName, mounts, populateVMTable, root)
	})

	form.SetBorder(true).SetTitle(fmt.Sprintf(" Modify Mount for: %s ", vmName))

	app.SetInputCapture(nil)
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			mounts, _ := getVMMounts(vmName)
			showMountManager(app, vmName, mounts, populateVMTable, root)
			return nil
		}
		return event
	})

	app.SetRoot(form, true)
}

// ─── Add Mount Flow ────────────────────────────────────────────────────────────

// showFilePickerForMount opens the file picker and feeds into the mount target form
func showFilePickerForMount(app *tview.Application, vmName string, populateVMTable func(), root tview.Primitive) {
	showFilePicker(app,
		func(sourcePath string) {
			// Source selected → show target path form
			showMountTargetForm(app, vmName, sourcePath, populateVMTable, root)
		},
		func() {
			// Cancelled → return to mount manager
			mounts, _ := getVMMounts(vmName)
			showMountManager(app, vmName, mounts, populateVMTable, root)
		},
	)
}

// showMountTargetForm shows a form for confirming source and setting the VM target path
func showMountTargetForm(app *tview.Application, vmName, sourcePath string, populateVMTable func(), root tview.Primitive) {
	// Default target: /home/ubuntu/<basename>
	baseName := filepath.Base(sourcePath)
	defaultTarget := "/home/ubuntu/" + baseName

	form := tview.NewForm()
	form.AddInputField("Source (Local):", sourcePath, 60, nil, nil)
	form.AddInputField("Target (VM):", defaultTarget, 60, nil, nil)

	form.AddButton("Mount", func() {
		source := form.GetFormItem(0).(*tview.InputField).GetText()
		target := form.GetFormItem(1).(*tview.InputField).GetText()

		if source == "" || target == "" {
			showError(app, "Validation Error", "Both source and target paths are required", form)
			return
		}

		info, err := os.Stat(source)
		if err != nil || !info.IsDir() {
			showError(app, "Validation Error",
				fmt.Sprintf("Source '%s' must be an existing directory", source), form)
			return
		}

		showLoadingAnimated(app,
			fmt.Sprintf("Mounting %s → %s:%s", source, vmName, target), root)

		go func() {
			if appLogger != nil {
				appLogger.Printf("Mounting %s to %s:%s", source, vmName, target)
			}
			_, err := runMultipassCommand("mount", source, vmName+":"+target)
			app.QueueUpdateDraw(func() {
				if err != nil {
					if appLogger != nil {
						appLogger.Printf("Mount failed: %v", err)
					}
					showError(app, "Mount Error", err.Error(), root)
				} else {
					if appLogger != nil {
						appLogger.Printf("Successfully mounted %s to %s:%s", source, vmName, target)
					}
					newMounts, _ := getVMMounts(vmName)
					showMountManager(app, vmName, newMounts, populateVMTable, root)
				}
			})
		}()
	})

	form.AddButton("Back", func() {
		// Go back to file picker
		showFilePickerForMount(app, vmName, populateVMTable, root)
	})

	form.AddButton("Cancel", func() {
		mounts, _ := getVMMounts(vmName)
		showMountManager(app, vmName, mounts, populateVMTable, root)
	})

	form.SetBorder(true).SetTitle(fmt.Sprintf(" Add Mount to: %s ", vmName))

	app.SetInputCapture(nil)
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			mounts, _ := getVMMounts(vmName)
			showMountManager(app, vmName, mounts, populateVMTable, root)
			return nil
		}
		return event
	})

	app.SetRoot(form, true)
}

// ─── TUI File Picker ───────────────────────────────────────────────────────────

// showFilePicker opens a directory browser starting at the user's home directory
func showFilePicker(app *tview.Application, onSelect func(string), onCancel func()) {
	homeDir, _ := os.UserHomeDir()
	if homeDir == "" {
		homeDir = "/"
	}
	showFilePickerAt(app, homeDir, false, onSelect, onCancel)
}

// showFilePickerAt opens a directory browser starting at the given directory.
// showHidden controls whether dotfiles/directories are displayed.
func showFilePickerAt(app *tview.Application, startDir string, showHidden bool, onSelect func(string), onCancel func()) {
	loadedNodes := make(map[*tview.TreeNode]bool)

	// --- Path display bar ---
	pathDisplay := tview.NewTextView()
	pathDisplay.SetBorder(true).SetTitle(" Selected Path ")
	pathDisplay.SetBorderPadding(0, 0, 1, 1)
	pathDisplay.SetDynamicColors(true)
	pathDisplay.SetText("[::b]" + startDir + "[::-]")

	// --- Directory loading helper ---
	loadChildren := func(node *tview.TreeNode) {
		if node.GetReference() == nil {
			return
		}
		path := node.GetReference().(string)

		entries, err := os.ReadDir(path)
		if err != nil {
			return
		}

		var dirs []os.DirEntry
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			if !showHidden && strings.HasPrefix(e.Name(), ".") {
				continue
			}
			dirs = append(dirs, e)
		}
		sort.Slice(dirs, func(i, j int) bool {
			return strings.ToLower(dirs[i].Name()) < strings.ToLower(dirs[j].Name())
		})

		for _, d := range dirs {
			childPath := filepath.Join(path, d.Name())
			child := tview.NewTreeNode(d.Name())
			child.SetReference(childPath)
			child.SetColor(tview.Styles.PrimaryTextColor)
			child.SetSelectable(true)
			child.SetExpanded(false)
			node.AddChild(child)
		}
	}

	// --- Build root node ---
	rootNode := tview.NewTreeNode(startDir)
	rootNode.SetReference(startDir)
	rootNode.SetColor(tcell.ColorYellow)
	rootNode.SetExpanded(true)
	loadChildren(rootNode)
	loadedNodes[rootNode] = true

	// --- Tree view ---
	tree := tview.NewTreeView()
	tree.SetRoot(rootNode)
	tree.SetCurrentNode(rootNode)

	hiddenLabel := ""
	if showHidden {
		hiddenLabel = " (showing hidden)"
	}
	tree.SetBorder(true).SetTitle(" Browse Directories" + hiddenLabel + " ")
	tree.SetBorderPadding(0, 0, 1, 1)

	// Enter expands/collapses; lazy-loads children on first expand
	tree.SetSelectedFunc(func(node *tview.TreeNode) {
		if node.GetReference() == nil {
			return
		}
		if node.IsExpanded() {
			node.SetExpanded(false)
		} else {
			if !loadedNodes[node] {
				loadChildren(node)
				loadedNodes[node] = true
			}
			node.SetExpanded(true)
		}
	})

	// Update path display when cursor moves
	tree.SetChangedFunc(func(node *tview.TreeNode) {
		if node.GetReference() != nil {
			pathDisplay.SetText("[::b]" + node.GetReference().(string) + "[::-]")
		}
	})

	// --- Instructions ---
	instructions := tview.NewTextView()
	instructions.SetText("[yellow]↑↓[white] Navigate  [yellow]Enter[white] Expand  [yellow]Space[white] Select  [yellow]u[white] Go Up  [yellow].[white] Toggle Hidden  [yellow]t[white] Type Path  [yellow]Esc[white] Cancel")
	instructions.SetTextAlign(tview.AlignCenter)
	instructions.SetDynamicColors(true)

	// --- Layout ---
	pickerFlex := tview.NewFlex().SetDirection(tview.FlexRow)
	pickerFlex.AddItem(pathDisplay, 3, 0, false)
	pickerFlex.AddItem(tree, 0, 1, true)
	pickerFlex.AddItem(instructions, 1, 0, false)

	// --- Input handling ---
	app.SetInputCapture(nil)

	tree.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			onCancel()
			return nil
		}

		switch event.Rune() {
		case ' ':
			// Select the currently highlighted directory
			node := tree.GetCurrentNode()
			if node != nil && node.GetReference() != nil {
				onSelect(node.GetReference().(string))
			}
			return nil

		case 'u':
			// Navigate up to parent directory
			parentPath := filepath.Dir(startDir)
			if parentPath != startDir {
				showFilePickerAt(app, parentPath, showHidden, onSelect, onCancel)
			}
			return nil

		case '.':
			// Toggle hidden files/directories
			showFilePickerAt(app, startDir, !showHidden, onSelect, onCancel)
			return nil

		case 't':
			// Type a path manually
			showPathInput(app, onSelect, func() {
				showFilePickerAt(app, startDir, showHidden, onSelect, onCancel)
			})
			return nil
		}

		return event
	})

	app.SetRoot(pickerFlex, true)
}

// showPathInput shows a simple form for typing a directory path manually
func showPathInput(app *tview.Application, onSelect func(string), onCancel func()) {
	homeDir, _ := os.UserHomeDir()
	if homeDir == "" {
		homeDir = "/"
	}

	form := tview.NewForm()
	form.AddInputField("Directory Path:", homeDir+"/", 60, nil, nil)

	form.AddButton("Select", func() {
		path := form.GetFormItem(0).(*tview.InputField).GetText()

		// Expand ~ to home directory
		if strings.HasPrefix(path, "~/") {
			path = filepath.Join(homeDir, path[2:])
		} else if path == "~" {
			path = homeDir
		}

		info, err := os.Stat(path)
		if err != nil {
			showError(app, "Invalid Path",
				fmt.Sprintf("'%s' does not exist", path), form)
			return
		}
		if !info.IsDir() {
			showError(app, "Invalid Path",
				fmt.Sprintf("'%s' is not a directory", path), form)
			return
		}

		onSelect(path)
	})

	form.AddButton("Cancel", func() {
		onCancel()
	})

	form.SetBorder(true).SetTitle(" Enter Directory Path ")
	form.SetBorderPadding(1, 1, 2, 2)

	app.SetInputCapture(nil)
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			onCancel()
			return nil
		}
		return event
	})

	app.SetRoot(form, true)
}
