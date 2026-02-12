// ui_helpers.go - UI utility functions, modals, and helpers
package main

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func updateFilterInputStyle(filterInput *tview.InputField, focused bool) {
	if focused {
		filterInput.SetLabelColor(tview.Styles.SecondaryTextColor)
		filterInput.SetFieldTextColor(tview.Styles.PrimaryTextColor)
		filterInput.SetFieldBackgroundColor(tview.Styles.PrimitiveBackgroundColor)
		return
	}

	// Dim the field when filter is active but not focused to indicate
	// filtering is still applied in the background.
	if globalFilterText != "" {
		filterInput.SetLabelColor(tcell.ColorGray)
		filterInput.SetFieldTextColor(tcell.ColorGray)
		filterInput.SetFieldBackgroundColor(tcell.ColorBlack)
		return
	}

	filterInput.SetLabelColor(tview.Styles.SecondaryTextColor)
	filterInput.SetFieldTextColor(tview.Styles.PrimaryTextColor)
	filterInput.SetFieldBackgroundColor(tview.Styles.PrimitiveBackgroundColor)
}

// setupGlobalInputCapture handles global keyboard shortcuts
func setupGlobalInputCapture() {
	globalApp.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// If filter is visible, Esc should close it instead of quitting.
		if globalFilterVisible && event.Key() == tcell.KeyEscape {
			if globalFilterText == "" {
				globalMainFlex.ResizeItem(globalFilterInput, 0, 0)
				globalFilterVisible = false
			}
			updateFilterInputStyle(globalFilterInput, false)
			globalApp.SetFocus(globalVMTable)
			return nil
		}

		// While the filter box is focused, let the input field consume keys
		// so typing letters like 's' does not trigger global actions.
		if globalFilterVisible && globalApp.GetFocus() == globalFilterInput {
			if event.Key() == tcell.KeyCtrlQ {
				globalApp.Stop()
				return nil
			}
			return event
		}

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
			showHelp(globalApp, globalRoot)
			return nil
		case 'c':
			quickCreateVM(globalApp, globalVMTable, globalPopulateVMTable, globalRoot)
			return nil
		case 'C':
			createAdvancedVM(globalApp, globalVMTable, globalPopulateVMTable, globalRoot)
			return nil
		case '[':
			stopSelectedVM(globalApp, globalVMTable, globalPopulateVMTable, globalRoot)
			return nil
		case ']':
			startSelectedVM(globalApp, globalVMTable, globalPopulateVMTable, globalRoot)
			return nil
		case 'p':
			suspendSelectedVM(globalApp, globalVMTable, globalPopulateVMTable, globalRoot)
			return nil
		case '<':
			stopAllVMs(globalApp, globalVMTable, globalPopulateVMTable, globalRoot)
			return nil
		case '>':
			startAllVMs(globalApp, globalVMTable, globalPopulateVMTable, globalRoot)
			return nil
		case 'd':
			deleteSelectedVM(globalApp, globalVMTable, globalPopulateVMTable, globalRoot)
			return nil
		case 'r':
			recoverSelectedVM(globalApp, globalVMTable, globalPopulateVMTable, globalRoot)
			return nil
		case '!':
			purgeAllVMs(globalApp, globalVMTable, globalPopulateVMTable, globalRoot)
			return nil
		case '/':
			globalPopulateVMTable()
			return nil
		case 'f':
			toggleFilter(globalApp, globalMainFlex, globalVMTable, globalFilterInput)
			return nil
		case 's':
			shellIntoVM(globalApp, globalVMTable)
			return nil
		case 'n':
			createSnapshot(globalApp, globalVMTable, globalPopulateVMTable, globalRoot)
			return nil
		case 'm':
			manageSnapshots(globalApp, globalVMTable, globalPopulateVMTable, globalRoot)
			return nil
		case 'M':
			manageMounts(globalApp, globalVMTable, globalPopulateVMTable, globalRoot)
			return nil
		case 'v':
			showVersion(globalApp, globalRoot)
			return nil
		}

		return event
	})
}

// showVersion displays version info in a modal dialog
func showVersion(app *tview.Application, root tview.Primitive) {
	modal := tview.NewModal().
		SetText(GetVersion()).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			app.SetRoot(root, true)
		})
	app.SetRoot(modal, false)
}

// showHelp displays help modal with keyboard shortcuts
func showHelp(app *tview.Application, root tview.Primitive) {
	modal := tview.NewModal().
		SetText("Keyboard Shortcuts:\n\nh: Help\nc: Quick Create\nC: Advanced Create (with cloud-init support)\n[: Stop\n]: Start\np: Suspend\n<: Stop ALL\n>: Start ALL\nd: Delete\nr: Recover\n!: Purge ALL\n/: Refresh\nf: Filter VMs\ns: Shell (interactive session)\nn: Snapshot\nm: Manage Snapshots\nM: Manage Mounts\nv: Version\nq: Quit\n\nCloud-init: Place YAML files with '#cloud-config' header in your current directory to use them during VM creation.\n\nShell: Press 's' to launch an interactive shell session. The TUI will suspend and restore when you exit the shell.\n\nFilter: Press 'f' to filter VMs by name. Press Esc or Enter to close the filter. Click column headers to sort.\n\nMounts: Press 'M' to manage mount points for the selected VM. Add, modify, or remove mounts between your local filesystem and the VM. A built-in file picker lets you browse directories visually.").
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			app.SetRoot(root, true)
		})
	app.SetRoot(modal, false)
}

// toggleFilter shows or hides the filter input field
func toggleFilter(app *tview.Application, mainFlex *tview.Flex, vmTable *tview.Table, filterInput *tview.InputField) {
	if globalFilterVisible {
		if app.GetFocus() == filterInput {
			if globalFilterText == "" {
				// No active filter, so collapse it.
				mainFlex.ResizeItem(filterInput, 0, 0)
				globalFilterVisible = false
			}
			updateFilterInputStyle(filterInput, false)
			app.SetFocus(vmTable)
			return
		}

		// Already visible but inactive: focus filter for editing.
		updateFilterInputStyle(filterInput, true)
		app.SetFocus(filterInput)
	} else {
		// Filter is hidden, expand it.
		mainFlex.ResizeItem(filterInput, 1, 0)
		globalFilterVisible = true
		updateFilterInputStyle(filterInput, true)
		app.SetFocus(filterInput)
	}
}

// showError displays error modal
func showError(app *tview.Application, title, message string, root tview.Primitive) {
	modal := tview.NewModal().
		SetText(title + ": " + message).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			app.SetRoot(root, true)
		})
	app.SetRoot(modal, false)
}

// showLoadingAnimated displays a loading popup with animated progress indicator
func showLoadingAnimated(app *tview.Application, message string, root tview.Primitive) {
	// Animation frames for rotating indicator
	frames := []string{"|", "/", "-", "\\"}
	frameIndex := 0

	// Create a modal-like centered loading popup with initial rotating character
	initialMessage := fmt.Sprintf("%s\n\n[yellow]%s[::-] Please wait...", message, frames[frameIndex])
	modal := tview.NewModal().
		SetText(initialMessage).
		AddButtons([]string{}) // No buttons - just loading

	// Start animation goroutine
	go func() {
		ticker := time.NewTicker(200 * time.Millisecond) // Update every 200ms
		defer ticker.Stop()

		for range ticker.C {
			// Update the animation frame
			animatedMessage := fmt.Sprintf("%s\n\n[yellow]%s[::-] Please wait...", message, frames[frameIndex])
			app.QueueUpdateDraw(func() {
				modal.SetText(animatedMessage)
			})

			// Move to next frame
			frameIndex = (frameIndex + 1) % len(frames)
		}
	}()

	app.SetRoot(modal, false)
}

// buildSnapshotTree creates a tree structure from snapshots showing parent-child relationships
func buildSnapshotTree(snapshots []SnapshotInfo) *tview.TreeNode {
	// Create a map to store snapshots by name for quick lookup
	snapshotMap := make(map[string]*SnapshotInfo)
	for i := range snapshots {
		snapshotMap[snapshots[i].Name] = &snapshots[i]
	}

	// Create a map to store tree nodes by snapshot name
	nodeMap := make(map[string]*tview.TreeNode)

	// Create root node
	rootNode := tview.NewTreeNode("Snapshots").SetColor(tview.Styles.SecondaryTextColor)
	rootNode.SetExpanded(true)

	// First pass: create all nodes
	for _, snapshot := range snapshots {
		var nodeText string

		// Add icon based on whether it has children (we'll determine this later)
		if snapshot.Comment != "" {
			nodeText = fmt.Sprintf("%s (%s)", snapshot.Name, snapshot.Comment)
		} else {
			nodeText = snapshot.Name
		}

		node := tview.NewTreeNode(nodeText)
		node.SetColor(tview.Styles.PrimaryTextColor)
		node.SetReference(snapshot)

		nodeMap[snapshot.Name] = node
	}

	// Second pass: build the tree structure
	for _, snapshot := range snapshots {
		node := nodeMap[snapshot.Name]

		if snapshot.Parent == "" {
			// This is a root snapshot (no parent)
			rootNode.AddChild(node)
		} else {
			// This snapshot has a parent
			if parentNode, exists := nodeMap[snapshot.Parent]; exists {
				parentNode.AddChild(node)
			} else {
				// Parent not found, add to root (orphaned snapshot)
				rootNode.AddChild(node)
			}
		}
	}

	// Third pass: update icons for nodes with children
	for _, snapshot := range snapshots {
		node := nodeMap[snapshot.Name]
		if len(node.GetChildren()) > 0 {
			// This node has children, update its text
			var newText string
			if snapshot.Comment != "" {
				newText = fmt.Sprintf("%s (%s)", snapshot.Name, snapshot.Comment)
			} else {
				newText = snapshot.Name
			}
			node.SetText(newText)
		}
	}

	// Set initial selection to first snapshot if available
	if len(snapshots) > 0 {
		firstSnapshot := snapshots[0]
		if firstNode, exists := nodeMap[firstSnapshot.Name]; exists {
			// Expand the path to the first snapshot
			expandPathToNode(rootNode, firstNode)
		}
	}

	return rootNode
}

// expandPathToNode expands all nodes in the path to reach the target node
func expandPathToNode(root, target *tview.TreeNode) bool {
	if root == target {
		return true
	}

	for _, child := range root.GetChildren() {
		if expandPathToNode(child, target) {
			root.SetExpanded(true)
			return true
		}
	}

	return false
}
